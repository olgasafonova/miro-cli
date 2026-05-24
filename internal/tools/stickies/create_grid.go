package stickies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// gridMaxStickies caps a single create-grid call. Matches the miro-mcp-server
// composite (50) — keeps batch sizes reasonable for the /items/bulk endpoint
// (which itself caps each request at 20, so we batch internally).
const gridMaxStickies = 50

// bulkBatchSize is the per-request cap on /v2/boards/{id}/items/bulk. Miro
// rejects arrays larger than this; we split the grid across as many calls as
// needed and aggregate the responses.
const bulkBatchSize = 20

// createGridFlags drives `stickies create-grid`. Contents come from one of two
// flag shapes — a path to a file with one content per line, or an inline JSON
// array of strings. Same pattern as `items bulk-create`, just specialized to
// strings (the grid only places sticky_note items).
type createGridFlags struct {
	boardID      string
	contentsFile string
	contentsJSON string
	columns      int
	color        string
	startX       float64
	startY       float64
	spacing      float64
	parentID     string
}

func newCreateGridCmd(g *clictx.Globals) *cobra.Command {
	var f createGridFlags
	cmd := &cobra.Command{
		Use:   "create-grid",
		Short: "Create multiple sticky notes laid out in a grid",
		Long: "Composes a grid of sticky_note items and posts them to\n" +
			"/v2/boards/{board_id}/items/bulk in batches of 20. Pass\n" +
			"--contents-file PATH (one sticky per line) or --contents-json\n" +
			"STRING (JSON array of strings). Up to 50 stickies per call.\n\n" +
			"Layout: --columns wide (default 3), filled left-to-right then\n" +
			"top-to-bottom, with --spacing pixels between cell centers\n" +
			"(default 220). --start-x / --start-y position the first cell.\n" +
			"--color and --parent-id apply to every sticky in the grid.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateGrid(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.contentsFile, "contents-file", "", "Path to a file with one sticky's text per line")
	cmd.Flags().StringVar(&f.contentsJSON, "contents-json", "", "Inline JSON array of strings, one per sticky")
	cmd.Flags().IntVar(&f.columns, "columns", 3, "Number of columns in the grid")
	cmd.Flags().StringVar(&f.color, "color", "", "Color applied to every sticky (yellow|green|blue|pink|purple|... or a Miro-native name)")
	cmd.Flags().Float64Var(&f.startX, "start-x", 0, "X coordinate of the top-left cell")
	cmd.Flags().Float64Var(&f.startY, "start-y", 0, "Y coordinate of the top-left cell")
	cmd.Flags().Float64Var(&f.spacing, "spacing", 220, "Pixels between cell centers (horizontal and vertical)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place every sticky inside")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runCreateGrid(ctx context.Context, g *clictx.Globals, f createGridFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	contents, err := loadGridContents(f)
	if err != nil {
		return err
	}
	items := buildGridItems(f, contents)
	path := "/v2/boards/" + f.boardID + "/items/bulk"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	// /items/bulk caps each request at 20. Batch and aggregate so callers
	// see one envelope per CLI invocation rather than per HTTP call. The
	// MCP server batches the same way for the same reason.
	merged := gridResult{}
	for i := 0; i < len(items); i += bulkBatchSize {
		end := i + bulkBatchSize
		if end > len(items) {
			end = len(items)
		}
		var resp map[string]any
		if err := client.Post(ctx, path, items[i:end], &resp); err != nil {
			return err
		}
		merged.append(resp)
	}
	merged.Columns = effectiveColumns(f.columns)
	merged.Rows = (len(items) + merged.Columns - 1) / merged.Columns
	return g.EmitJSON(merged)
}

// gridResult is the envelope emitted to stdout. It aggregates the `data`
// arrays from each bulk batch and adds derived rows/columns so callers don't
// have to count.
type gridResult struct {
	Data    []json.RawMessage `json:"data"`
	Created int               `json:"created"`
	Rows    int               `json:"rows"`
	Columns int               `json:"columns"`
}

// append flattens one /items/bulk response into the running envelope. Miro
// returns `{"data":[{...},{...}]}`; older snapshots in tests use the same
// shape, so we only look for the array.
func (r *gridResult) append(resp map[string]any) {
	raw, ok := resp["data"]
	if !ok {
		return
	}
	// Round-trip through json.Marshal so we can store entries as RawMessage
	// without committing to a specific item schema (sticky payloads differ
	// across API versions).
	b, err := json.Marshal(raw)
	if err != nil {
		return
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		return
	}
	r.Data = append(r.Data, arr...)
	r.Created += len(arr)
}

// loadGridContents reads the sticky-text payload from --contents-file or
// --contents-json. File mode treats each non-empty line as one sticky
// (trailing whitespace trimmed); JSON mode expects an array of strings.
func loadGridContents(f createGridFlags) ([]string, error) {
	if f.contentsFile == "" && f.contentsJSON == "" {
		return nil, errors.New("one of --contents-file or --contents-json is required")
	}
	if f.contentsFile != "" && f.contentsJSON != "" {
		return nil, errors.New("--contents-file and --contents-json are mutually exclusive")
	}
	var contents []string
	if f.contentsFile != "" {
		raw, err := os.ReadFile(f.contentsFile) //nolint:gosec // G304: path is operator-supplied; create-grid exists to load operator-curated content
		if err != nil {
			return nil, fmt.Errorf("read --contents-file: %w", err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimRight(line, "\r")
			line = strings.TrimSpace(line)
			if line != "" {
				contents = append(contents, line)
			}
		}
	} else {
		if err := json.Unmarshal([]byte(f.contentsJSON), &contents); err != nil {
			return nil, fmt.Errorf("parse --contents-json as []string: %w", err)
		}
	}
	if len(contents) == 0 {
		return nil, errors.New("contents is empty")
	}
	if len(contents) > gridMaxStickies {
		return nil, fmt.Errorf("contents has %d entries, max %d", len(contents), gridMaxStickies)
	}
	return contents, nil
}

// effectiveColumns clamps --columns to >=1. Caller code can rely on the
// returned value as a non-zero divisor.
func effectiveColumns(columns int) int {
	if columns <= 0 {
		return 3
	}
	return columns
}

// buildGridItems lays out the contents in a left-to-right, top-to-bottom
// grid and produces the typed-item array for /v2/boards/{id}/items/bulk.
// Each item carries the same color / parent so callers can theme the whole
// grid with one flag.
func buildGridItems(f createGridFlags, contents []string) []bulkItem {
	columns := effectiveColumns(f.columns)
	spacing := f.spacing
	if spacing == 0 {
		spacing = 220
	}
	color := normalizeStickyColor(f.color)
	items := make([]bulkItem, len(contents))
	for i, c := range contents {
		row := i / columns
		col := i % columns
		item := bulkItem{
			Type: "sticky_note",
			Data: &dataField{Content: c},
			Position: &positionData{
				X:      f.startX + float64(col)*spacing,
				Y:      f.startY + float64(row)*spacing,
				Origin: "center",
			},
		}
		if color != "" {
			item.Style = &styleField{FillColor: color}
		}
		if f.parentID != "" {
			item.Parent = &parentRef{ID: f.parentID}
		}
		items[i] = item
	}
	return items
}

// bulkItem is one element in the /v2/boards/{id}/items/bulk request array.
// The shape mirrors the per-type create envelope plus a `type` discriminator,
// which is how Miro's bulk endpoint routes a heterogeneous array. Defined
// here rather than in types.go because no other sticky verb uses it.
type bulkItem struct {
	Type     string        `json:"type"`
	Data     *dataField    `json:"data,omitempty"`
	Style    *styleField   `json:"style,omitempty"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

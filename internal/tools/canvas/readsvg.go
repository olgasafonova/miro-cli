package canvas

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
	"github.com/olgasafonova/miro-cli/internal/tools/connectors"
	"github.com/olgasafonova/miro-cli/internal/tools/items"
)

// maxSVGItems caps the item fetch for a render.
const maxSVGItems = 2000

// defaultSVGItems is the fetch size when the caller doesn't specify one.
const defaultSVGItems = 500

// connectorFetchLimit is the page size for the best-effort connector
// fetch (single page; connectors are decoration, not structure).
const connectorFetchLimit = 50

// readSVGFlags captures the per-invocation knobs for `miro canvas
// read-svg`.
type readSVGFlags struct {
	boardID  string
	frameID  string
	maxItems int
}

// readSVGResult is the JSON envelope `canvas read-svg` emits. SVG is
// the whole document; pipe through `jq -r .svg` to write a .svg file.
type readSVGResult struct {
	SVG       string `json:"svg"`
	ItemCount int    `json:"item_count"`
	Skipped   int    `json:"skipped"`
	Truncated bool   `json:"truncated,omitempty"`
	Message   string `json:"message"`
}

func newReadSVGCmd(g *clictx.Globals) *cobra.Command {
	var f readSVGFlags
	cmd := &cobra.Command{
		Use:   "read-svg",
		Short: "Render a board's items as an SVG document",
		Long: "Lists the board's items (and, best-effort, its connectors) and\n" +
			"renders them locally as a plain SVG document — no image export\n" +
			"job, no rendering service. Frames become dashed outlines, shapes\n" +
			"and stickies become rects/ellipses, text becomes <text>,\n" +
			"connectors become lines between item centers. Items without\n" +
			"drawable geometry are counted in `skipped`.\n\n" +
			"With --frame-id the render scopes to that frame and its\n" +
			"children; child coordinates come back frame-relative (origin at\n" +
			"the frame's top-left), and the frame outline is drawn at (0,0).\n\n" +
			"Output: { svg, item_count, skipped, truncated, message }.\n" +
			"Extract the document with `--select svg` or `jq -r .svg`. The\n" +
			"output is directly re-submittable to `canvas update-from-svg`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReadSVG(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.frameID, "frame-id", "", "Scope the render to one frame and its children")
	cmd.Flags().IntVar(&f.maxItems, "max-items", 0,
		fmt.Sprintf("Item fetch cap (1-%d; 0 = default %d)", maxSVGItems, defaultSVGItems))
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

// clampSVGMaxItems normalizes the requested item cap.
func clampSVGMaxItems(n int) int {
	if n > 0 && n <= maxSVGItems {
		return n
	}
	return defaultSVGItems
}

func runReadSVG(ctx context.Context, g *clictx.Globals, f readSVGFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.frameID != "" {
		return runReadFrameSVG(ctx, g, f)
	}
	lf := items.ListFlags{BoardID: f.boardID}
	if g.DryRun {
		return g.EmitDryRun("GET", items.BuildListPath(lf)+" (paginated, rendered locally)")
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	rawItems, truncated, err := items.FetchAll(ctx, client, lf, items.FetchAllOptions{MaxItems: clampSVGMaxItems(f.maxItems)})
	if err != nil {
		return fmt.Errorf("failed to list items: %w", err)
	}

	// Connectors are best-effort decoration; a failure there shouldn't
	// sink the render.
	var conns []boardConnector
	if resp, connErr := connectors.Fetch(ctx, client, connectors.ListFlags{BoardID: f.boardID, Limit: connectorFetchLimit}); connErr == nil {
		conns = parseBoardConnectors(resp.Data)
	}

	svg, rendered, skipped := renderBoardSVG(parseBoardItems(rawItems), conns)
	return g.EmitJSON(readSVGResult{
		SVG:       svg,
		ItemCount: rendered,
		Skipped:   skipped,
		Truncated: truncated,
		Message:   fmt.Sprintf("Rendered %d item(s) as SVG (%d skipped)", rendered, skipped),
	})
}

// frameChildrenPageSize is the page size for frame children. The items
// listing rejects anything above 50 with a 400 (verified live
// 18-08-2026), so this is the endpoint's own maximum, not a choice.
const frameChildrenPageSize = 50

// frameChildrenPath builds one page request for a frame's children,
// using the same parent_item_id filter `items get-within-frame` wraps.
func frameChildrenPath(boardID, frameID, cursor string) string {
	q := url.Values{}
	q.Set("parent_item_id", frameID)
	q.Set("limit", strconv.Itoa(frameChildrenPageSize))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return "/v2/boards/" + boardID + "/items?" + q.Encode()
}

// fetchFrameChildren pages through a frame's children up to maxItems.
// The returned truncated flag reports whether more children remained.
func fetchFrameChildren(ctx context.Context, client *miro.Client, boardID, frameID string, maxItems int) ([]map[string]any, bool, error) {
	var children []map[string]any
	cursor := ""
	for {
		var resp struct {
			Data   []map[string]any `json:"data"`
			Cursor string           `json:"cursor"`
		}
		if err := client.Get(ctx, frameChildrenPath(boardID, frameID, cursor), &resp); err != nil {
			return nil, false, fmt.Errorf("failed to list frame items: %w", err)
		}
		children = append(children, resp.Data...)
		if len(children) >= maxItems {
			return children[:maxItems], resp.Cursor != "" || len(children) > maxItems, nil
		}
		if resp.Cursor == "" {
			return children, false, nil
		}
		cursor = resp.Cursor
	}
}

// runReadFrameSVG renders one frame and its children. Child coordinates
// come back frame-relative (center-anchored, origin at the frame's
// top-left), so the frame outline is drawn at (0,0) and children render
// as returned.
func runReadFrameSVG(ctx context.Context, g *clictx.Globals, f readSVGFlags) error {
	if err := miro.ValidateID("frame_id", f.frameID); err != nil {
		return err
	}
	framePath := "/v2/boards/" + f.boardID + "/frames/" + f.frameID
	if g.DryRun {
		return g.EmitDryRun("GET", framePath+" + children via parent_item_id (rendered locally)")
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	var frame map[string]any
	if err := client.Get(ctx, framePath, &frame); err != nil {
		return fmt.Errorf("failed to get frame: %w", err)
	}
	frameW := num(subMap(frame, "geometry"), "width")
	frameH := num(subMap(frame, "geometry"), "height")
	frameTitle := str(subMap(frame, "data"), "title")

	children, truncated, err := fetchFrameChildren(ctx, client, f.boardID, f.frameID, clampSVGMaxItems(f.maxItems))
	if err != nil {
		return err
	}

	var body strings.Builder
	bounds := &svgBounds{}
	bounds.add(0, 0, frameW, frameH)
	fmt.Fprintf(&body, `<rect x="0" y="0" width="%.1f" height="%.1f" fill="none" stroke="#999" stroke-dasharray="6,3" data-miro-id=%q data-miro-type="frame"/>`+"\n",
		frameW, frameH, f.frameID)
	if frameTitle != "" {
		fmt.Fprintf(&body, `<text x="4" y="-6" font-size="12" fill="#666">%s</text>`+"\n", html.EscapeString(frameTitle))
	}
	rendered, skipped := renderSVGItems(&body, parseBoardItems(children), bounds)

	return g.EmitJSON(readSVGResult{
		SVG:       wrapSVGDocument(body.String(), bounds),
		ItemCount: rendered,
		Skipped:   skipped,
		Truncated: truncated,
		Message:   fmt.Sprintf("Rendered frame %q with %d item(s) as SVG (%d skipped); child coordinates are relative to the frame's top-left", f.frameID, rendered, skipped),
	})
}

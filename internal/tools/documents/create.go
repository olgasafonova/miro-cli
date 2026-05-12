package documents

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro documents create`.
// Named so the cobra wiring stays flat and the helper signature stays
// friendly to table-driven tests.
type createFlags struct {
	boardID  string
	url      string
	title    string
	x        float64
	y        float64
	width    float64
	height   float64
	parentID string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a document on a board (URL-based)",
		Long: "Calls POST /v2/boards/{board_id}/documents with --url (required)\n" +
			"and optional --title / --x / --y / --width / --height /\n" +
			"--parent-id. Returns the new document's id and metadata.\n\n" +
			"Scope: URL-based documents only; file-upload via multipart\n" +
			"form-data is deferred to Phase 4.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"frame's top-left and the document's center is placed at (x, y).\n" +
			"On the canvas (no parent), they are absolute.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.url, "url", "", "Document source URL (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Document title")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the document inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	if f.url == "" {
		return errors.New("--url is required")
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/documents"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Post(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildCreateRequest is split out so tests can assert on the wire shape
// without spinning an httptest server.
func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{
		Data: dataField{URL: f.url, Title: f.title},
	}
	// Position is always emitted when create is called — Miro defaults
	// to (0, 0) which is fine for canvas placement, and explicit origin
	// keeps the math predictable for frame-relative coords.
	req.Position = &positionData{X: f.x, Y: f.y, Origin: "center"}
	if f.width > 0 || f.height > 0 {
		req.Geometry = &geometryData{Width: f.width, Height: f.height}
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

package images

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro images create`.
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
		Short: "Create an image on a board (from URL)",
		Long: "Calls POST /v2/boards/{board_id}/images with --url (required)\n" +
			"and optional --title / --x / --y / --width / --height /\n" +
			"--parent-id. Returns the new image's id and metadata.\n\n" +
			"Scope: URL-based create only. File upload via multipart/form-data\n" +
			"is a Phase 4 follow-up.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"frame's top-left and the image's center is placed at (x, y). On\n" +
			"the canvas (no parent), they are absolute.\n\n" +
			"Geometry: pass --width OR --height alone; Miro auto-scales the\n" +
			"other to preserve the image's aspect ratio.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.url, "url", "", "Image source URL (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Image title / alt text")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels (Miro auto-scales height if only width is set)")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels (Miro auto-scales width if only height is set)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the image inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.url == "" {
		return errors.New("--url is required")
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/images"
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

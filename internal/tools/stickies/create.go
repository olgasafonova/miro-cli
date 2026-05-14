package stickies

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro stickies create`.
// Named so the cobra wiring stays flat and the helper signature stays
// friendly to table-driven tests.
type createFlags struct {
	boardID  string
	content  string
	color    string
	shape    string
	x        float64
	y        float64
	width    float64
	parentID string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a sticky note on a board",
		Long: "Calls POST /v2/boards/{board_id}/sticky_notes with --content\n" +
			"(required) and optional --color / --shape / --x / --y / --width /\n" +
			"--parent-id. Returns the new sticky's id, content, color, and\n" +
			"viewLink.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"frame's top-left and the sticky's center is placed at (x, y).\n" +
			"On the canvas (no parent), they are absolute.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "Sticky text (required)")
	cmd.Flags().StringVar(&f.color, "color", "", "Sticky color (yellow|green|blue|pink|purple|red|orange|gray|black or a Miro-native name)")
	cmd.Flags().StringVar(&f.shape, "shape", "", "Sticky shape (square|rectangle)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels (default ~199; height auto-scales)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the sticky inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.content == "" {
		return errors.New("--content is required")
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/sticky_notes"
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
// without spinning an httptest server. Color normalization happens here.
func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{
		Data: dataField{Content: f.content, Shape: f.shape},
	}
	if f.color != "" {
		req.Style = &styleField{FillColor: normalizeStickyColor(f.color)}
	}
	// Position is always emitted when create is called — Miro defaults
	// to (0, 0) which is fine for canvas placement, and explicit origin
	// keeps the math predictable for frame-relative coords.
	req.Position = &positionData{X: f.x, Y: f.y, Origin: "center"}
	if f.width > 0 {
		req.Geometry = &geometryData{Width: f.width}
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

package frames

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro frames create`.
// Named so the cobra wiring stays flat and the helper signature stays
// friendly to table-driven tests.
type createFlags struct {
	boardID     string
	title       string
	format      string
	frameType   string
	showContent bool
	color       string
	x           float64
	y           float64
	width       float64
	height      float64
	parentID    string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a frame on a board",
		Long: "Calls POST /v2/boards/{board_id}/frames with optional --title /\n" +
			"--format / --type / --show-content / --color / --x / --y /\n" +
			"--width / --height / --parent-id. A bare frame (no flags beyond\n" +
			"--board-id) is valid and yields a default freeform frame.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"parent's top-left and the frame's center is placed at (x, y).\n" +
			"On the canvas (no parent), they are absolute.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Frame title")
	cmd.Flags().StringVar(&f.format, "format", "", "Frame format (custom|...) — API default is custom when unset")
	cmd.Flags().StringVar(&f.frameType, "type", "", "Frame type (freeform|...) — API default is freeform when unset")
	cmd.Flags().BoolVar(&f.showContent, "show-content", false, "Render frame contents in board overviews")
	cmd.Flags().StringVar(&f.color, "color", "", "Fill color (hex like #ffffff)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or parent-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or parent-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels (API default ~800)")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels (API default ~600)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Parent ID to place the frame inside")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/frames"
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
// without spinning an httptest server. No color normalization happens
// here — fillColor is a hex string the user supplies directly.
func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{
		Data: dataField{
			Title:       f.title,
			Format:      f.format,
			Type:        f.frameType,
			ShowContent: f.showContent,
		},
	}
	if f.color != "" {
		req.Style = &styleField{FillColor: f.color}
	}
	// Position is always emitted when create is called — Miro defaults
	// to (0, 0) which is fine for canvas placement, and explicit origin
	// keeps the math predictable for parent-relative coords.
	req.Position = &positionData{X: f.x, Y: f.y, Origin: "center"}
	if f.width > 0 || f.height > 0 {
		req.Geometry = &geometryData{Width: f.width, Height: f.height}
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

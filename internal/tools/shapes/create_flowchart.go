package shapes

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlowchartFlags drives `shapes create-flowchart`. It mirrors
// `shapes create` for geometry and content, but the styling surface is
// narrower: the v2-experimental flowchart endpoint accepts only fill +
// border colors, not the textColor / textAlign fields the standard
// shapes endpoint supports. Same struct layout is intentional — keeps
// the cobra wiring flat and the request builder testable in isolation.
type createFlowchartFlags struct {
	boardID     string
	shape       string
	content     string
	fillColor   string
	borderColor string
	x           float64
	y           float64
	width       float64
	height      float64
	parentID    string
}

func newCreateFlowchartCmd(g *clictx.Globals) *cobra.Command {
	var f createFlowchartFlags
	cmd := &cobra.Command{
		Use:   "create-flowchart",
		Short: "Create a flowchart stencil shape (v2-experimental)",
		Long: "Calls POST /v2-experimental/boards/{board_id}/shapes for the\n" +
			"flowchart stencil set: rhombus (decision diamond), pentagon,\n" +
			"trapezoid, parallelogram, flow_chart_predefined_process, and\n" +
			"the standard rectangle / circle / round_rectangle variants.\n\n" +
			"Distinct from `shapes create` (which targets /v2/.../shapes\n" +
			"for the general shape set). The flowchart endpoint accepts\n" +
			"--fill-color and --border-color but does not support text\n" +
			"color or alignment — use `shapes update` after creation if\n" +
			"text styling is required.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateFlowchart(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.shape, "shape", "", "Flowchart shape type (rhombus|pentagon|trapezoid|parallelogram|rectangle|circle|...)")
	cmd.Flags().StringVar(&f.content, "content", "", "Text inside the shape")
	cmd.Flags().StringVar(&f.fillColor, "fill-color", "", "Fill color (hex like #006400 or named)")
	cmd.Flags().StringVar(&f.borderColor, "border-color", "", "Border color (hex or named)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels (default 200)")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels (default 200)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the shape inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("shape")
	return cmd
}

func runCreateFlowchart(ctx context.Context, g *clictx.Globals, f createFlowchartFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.shape == "" {
		return errors.New("--shape is required")
	}
	req := buildCreateFlowchartRequest(f)
	path := "/v2-experimental/boards/" + f.boardID + "/shapes"
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

// buildCreateFlowchartRequest assembles the v2-experimental request body.
// Reuses the shared structs in types.go — the only practical difference
// from the standard create body is which style fields the caller is
// allowed to set, and that's enforced at the flag layer.
func buildCreateFlowchartRequest(f createFlowchartFlags) createRequest {
	req := createRequest{
		Data: dataField{Content: f.content, Shape: f.shape},
	}
	if f.fillColor != "" || f.borderColor != "" {
		req.Style = &styleField{
			FillColor:   f.fillColor,
			BorderColor: f.borderColor,
		}
	}
	req.Position = &positionData{X: f.x, Y: f.y, Origin: "center"}
	if f.width > 0 || f.height > 0 {
		req.Geometry = &geometryData{Width: f.width, Height: f.height}
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

package shapes

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

type createFlags struct {
	boardID           string
	shape             string
	content           string
	color             string
	textColor         string
	textAlign         string
	textAlignVertical string
	x                 float64
	y                 float64
	width             float64
	height            float64
	parentID          string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a shape on a board",
		Long: "Calls POST /v2/boards/{board_id}/shapes with --shape (e.g.\n" +
			"rectangle, circle, triangle, round_rectangle) and --content for\n" +
			"text inside the shape. Geometry and styling are optional.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.shape, "shape", "rectangle", "Shape type (rectangle|circle|triangle|round_rectangle|...)")
	cmd.Flags().StringVar(&f.content, "content", "", "Text inside the shape")
	cmd.Flags().StringVar(&f.color, "color", "", "Fill color (hex like #006400 or named)")
	cmd.Flags().StringVar(&f.textColor, "text-color", "", "Text color (hex or named)")
	cmd.Flags().StringVar(&f.textAlign, "text-align", "", "Horizontal text alignment (left|center|right)")
	cmd.Flags().StringVar(&f.textAlignVertical, "text-align-vertical", "", "Vertical text alignment (top|middle|bottom)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels (default 200)")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels (default 200)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the shape inside")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.shape == "" {
		return errors.New("--shape is required")
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/shapes"
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

func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{
		Data: dataField{Content: f.content, Shape: f.shape},
	}
	if f.color != "" || f.textColor != "" || f.textAlign != "" || f.textAlignVertical != "" {
		req.Style = &styleField{
			FillColor:         f.color,
			Color:             f.textColor,
			TextAlign:         f.textAlign,
			TextAlignVertical: f.textAlignVertical,
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

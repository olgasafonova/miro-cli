package shapes

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

type updateFlags struct {
	boardID           string
	itemID            string
	content           string
	shape             string
	color             string
	textColor         string
	textAlign         string
	textAlignVertical string
	x                 float64
	y                 float64
	width             float64
	height            float64
	parentID          string

	contentSet           bool
	shapeSet             bool
	colorSet             bool
	textColorSet         bool
	textAlignSet         bool
	textAlignVerticalSet bool
	xSet                 bool
	ySet                 bool
	widthSet             bool
	heightSet            bool
	parentIDSet          bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a shape (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/shapes/{item_id} with only\n" +
			"the fields you set. Pass --parent-id='' to detach from frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.contentSet = fl.Changed("content")
			f.shapeSet = fl.Changed("shape")
			f.colorSet = fl.Changed("color")
			f.textColorSet = fl.Changed("text-color")
			f.textAlignSet = fl.Changed("text-align")
			f.textAlignVerticalSet = fl.Changed("text-align-vertical")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.widthSet = fl.Changed("width")
			f.heightSet = fl.Changed("height")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Shape ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "New text inside shape")
	cmd.Flags().StringVar(&f.shape, "shape", "", "New shape type")
	cmd.Flags().StringVar(&f.color, "color", "", "New fill color")
	cmd.Flags().StringVar(&f.textColor, "text-color", "", "New text color")
	cmd.Flags().StringVar(&f.textAlign, "text-align", "", "New horizontal alignment")
	cmd.Flags().StringVar(&f.textAlignVertical, "text-align-vertical", "", "New vertical alignment")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width")
	cmd.Flags().Float64Var(&f.height, "height", 0, "New height")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Move to frame (empty string detaches)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", f.itemID); err != nil {
		return err
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/shapes/" + f.itemID
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

func buildUpdateRequest(f updateFlags) (updateRequest, bool) {
	var req updateRequest
	any := false

	if f.contentSet || f.shapeSet {
		req.Data = &dataField{}
		if f.contentSet {
			req.Data.Content = f.content
		}
		if f.shapeSet {
			req.Data.Shape = f.shape
		}
		any = true
	}
	if f.colorSet || f.textColorSet || f.textAlignSet || f.textAlignVerticalSet {
		req.Style = &styleField{}
		if f.colorSet {
			req.Style.FillColor = f.color
		}
		if f.textColorSet {
			req.Style.Color = f.textColor
		}
		if f.textAlignSet {
			req.Style.TextAlign = f.textAlign
		}
		if f.textAlignVerticalSet {
			req.Style.TextAlignVertical = f.textAlignVertical
		}
		any = true
	}
	if f.xSet || f.ySet {
		req.Position = &positionData{Origin: "center"}
		if f.xSet {
			req.Position.X = f.x
		}
		if f.ySet {
			req.Position.Y = f.y
		}
		any = true
	}
	if f.widthSet || f.heightSet {
		req.Geometry = &geometryData{Width: f.width, Height: f.height}
		any = true
	}
	if f.parentIDSet {
		req.Parent = &parentRef{ID: f.parentID}
		any = true
	}
	return req, any
}

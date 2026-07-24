package shapes

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
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

// buildUpdateRequest projects the updateFlags into the wire body and
// reports whether any field was set. ok=false means the caller should
// reject the update — Miro 400s an empty PATCH body anyway, and a
// pre-flight check produces a clearer error.
func buildUpdateRequest(f updateFlags) (updateRequest, bool) {
	var req updateRequest
	applied := f.applyData(&req)
	applied = f.applyStyle(&req) || applied
	applied = f.applyPosition(&req) || applied
	applied = f.applyGeometry(&req) || applied
	applied = f.applyParent(&req) || applied
	return req, applied
}

// dataChanged reports whether any field of the data envelope was set.
func (f updateFlags) dataChanged() bool {
	return f.contentSet || f.shapeSet
}

// styleChanged reports whether any field of the style envelope was set.
func (f updateFlags) styleChanged() bool {
	return f.colorSet || f.textColorSet || f.textAlignSet || f.textAlignVerticalSet
}

func (f updateFlags) positionChanged() bool {
	return f.xSet || f.ySet
}

func (f updateFlags) geometryChanged() bool {
	return f.widthSet || f.heightSet
}

// applyData fills the data envelope with the changed content fields
// and reports whether it set anything.
func (f updateFlags) applyData(req *updateRequest) bool {
	if !f.dataChanged() {
		return false
	}
	req.Data = &dataField{}
	if f.contentSet {
		req.Data.Content = f.content
	}
	if f.shapeSet {
		req.Data.Shape = f.shape
	}
	return true
}

func (f updateFlags) applyStyle(req *updateRequest) bool {
	if !f.styleChanged() {
		return false
	}
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
	return true
}

func (f updateFlags) applyPosition(req *updateRequest) bool {
	if !f.positionChanged() {
		return false
	}
	req.Position = &positionData{Origin: "center"}
	if f.xSet {
		req.Position.X = f.x
	}
	if f.ySet {
		req.Position.Y = f.y
	}
	return true
}

func (f updateFlags) applyGeometry(req *updateRequest) bool {
	if !f.geometryChanged() {
		return false
	}
	req.Geometry = &geometryData{Width: f.width, Height: f.height}
	return true
}

// applyParent re-parents the item. An empty string detaches; non-empty
// re-parents. Both flow through a non-nil parentRef so the JSON
// encoder emits the envelope.
func (f updateFlags) applyParent(req *updateRequest) bool {
	if !f.parentIDSet {
		return false
	}
	req.Parent = &parentRef{ID: f.parentID}
	return true
}

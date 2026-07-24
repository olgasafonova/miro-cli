package texts

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

type updateFlags struct {
	boardID  string
	itemID   string
	content  string
	color    string
	fontSize int
	width    float64
	x        float64
	y        float64
	parentID string

	contentSet  bool
	colorSet    bool
	fontSizeSet bool
	widthSet    bool
	xSet        bool
	ySet        bool
	parentIDSet bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a text item (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/texts/{item_id} with only\n" +
			"the fields you set. Pass --parent-id='' to detach from frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.contentSet = fl.Changed("content")
			f.colorSet = fl.Changed("color")
			f.fontSizeSet = fl.Changed("font-size")
			f.widthSet = fl.Changed("width")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Text ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "New text content")
	cmd.Flags().StringVar(&f.color, "color", "", "New color (hex or named)")
	cmd.Flags().IntVar(&f.fontSize, "font-size", 0, "New font size in points")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
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
	path := "/v2/boards/" + f.boardID + "/texts/" + f.itemID
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
	dataSet := applyDataField(&req, f)
	styleSet := applyStyleFields(&req, f)
	positionSet := applyPositionFields(&req, f)
	geometrySet := applyGeometryField(&req, f)
	parentSet := applyParentField(&req, f)
	return req, dataSet || styleSet || positionSet || geometrySet || parentSet
}

// applyDataField fills the data envelope when --content was passed.
// It reports whether req.Data was set.
func applyDataField(req *updateRequest, f updateFlags) bool {
	if !f.contentSet {
		return false
	}
	req.Data = &dataField{Content: f.content}
	return true
}

// applyStyleFields fills the style envelope when --color or
// --font-size was passed. It reports whether req.Style was set.
func applyStyleFields(req *updateRequest, f updateFlags) bool {
	if !f.colorSet && !f.fontSizeSet {
		return false
	}
	req.Style = &styleField{}
	if f.colorSet {
		req.Style.Color = f.color
	}
	if f.fontSizeSet {
		req.Style.FontSize = fontSizeString(f.fontSize)
	}
	return true
}

// applyPositionFields fills the position envelope when --x or --y was
// passed. It reports whether req.Position was set.
func applyPositionFields(req *updateRequest, f updateFlags) bool {
	if !f.xSet && !f.ySet {
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

// applyGeometryField fills the geometry envelope when --width was
// passed. It reports whether req.Geometry was set.
func applyGeometryField(req *updateRequest, f updateFlags) bool {
	if !f.widthSet {
		return false
	}
	req.Geometry = &geometryData{Width: f.width}
	return true
}

// applyParentField fills the parent envelope when --parent-id was
// passed. Empty string detaches; non-empty re-parents. It reports
// whether req.Parent was set.
func applyParentField(req *updateRequest, f updateFlags) bool {
	if !f.parentIDSet {
		return false
	}
	req.Parent = &parentRef{ID: f.parentID}
	return true
}

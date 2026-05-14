package frames

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set. Cobra zeroes unset float/bool vars, so we can't
// distinguish "user passed --x=0" from "user didn't pass --x" by
// looking at the value alone. The bool *Set fields track presence.
type updateFlags struct {
	boardID     string
	itemID      string
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

	titleSet       bool
	formatSet      bool
	typeSet        bool
	showContentSet bool
	colorSet       bool
	xSet           bool
	ySet           bool
	widthSet       bool
	heightSet      bool
	parentIDSet    bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a frame (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/frames/{item_id} with only\n" +
			"the fields you set. Pass an empty --parent-id to detach the\n" +
			"frame from its parent.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.titleSet = fl.Changed("title")
			f.formatSet = fl.Changed("format")
			f.typeSet = fl.Changed("type")
			f.showContentSet = fl.Changed("show-content")
			f.colorSet = fl.Changed("color")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.widthSet = fl.Changed("width")
			f.heightSet = fl.Changed("height")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Frame ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "New frame title")
	cmd.Flags().StringVar(&f.format, "format", "", "New frame format")
	cmd.Flags().StringVar(&f.frameType, "type", "", "New frame type")
	cmd.Flags().BoolVar(&f.showContent, "show-content", false, "Toggle rendering of frame contents")
	cmd.Flags().StringVar(&f.color, "color", "", "New fill color (hex)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width")
	cmd.Flags().Float64Var(&f.height, "height", 0, "New height")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Move to parent (empty string detaches from any parent)")
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
	path := "/v2/boards/" + f.boardID + "/frames/" + f.itemID
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
	any := false

	if f.titleSet || f.formatSet || f.typeSet || f.showContentSet {
		req.Data = &dataField{}
		if f.titleSet {
			req.Data.Title = f.title
		}
		if f.formatSet {
			req.Data.Format = f.format
		}
		if f.typeSet {
			req.Data.Type = f.frameType
		}
		if f.showContentSet {
			req.Data.ShowContent = f.showContent
		}
		any = true
	}
	if f.colorSet {
		req.Style = &styleField{FillColor: f.color}
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
		req.Geometry = &geometryData{}
		if f.widthSet {
			req.Geometry.Width = f.width
		}
		if f.heightSet {
			req.Geometry.Height = f.height
		}
		any = true
	}
	if f.parentIDSet {
		// Empty string detaches; non-empty re-parents. Both flow
		// through a non-nil parentRef so the JSON encoder emits the
		// envelope.
		req.Parent = &parentRef{ID: f.parentID}
		any = true
	}
	return req, any
}

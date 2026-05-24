package stickies

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set. Cobra zeroes unset float vars, so we can't
// distinguish "user passed --x=0" from "user didn't pass --x" by
// looking at the value alone. The bool *Set fields track presence.
type updateFlags struct {
	boardID  string
	itemID   string
	content  string
	shape    string
	color    string
	x        float64
	y        float64
	width    float64
	parentID string

	contentSet  bool
	shapeSet    bool
	colorSet    bool
	xSet        bool
	ySet        bool
	widthSet    bool
	parentIDSet bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a sticky note (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/sticky_notes/{item_id} with\n" +
			"only the fields you set. Pass an empty --parent-id to detach the\n" +
			"sticky from its frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.contentSet = fl.Changed("content")
			f.shapeSet = fl.Changed("shape")
			f.colorSet = fl.Changed("color")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.widthSet = fl.Changed("width")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Sticky ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "New sticky text")
	cmd.Flags().StringVar(&f.shape, "shape", "", "New shape (square|rectangle)")
	cmd.Flags().StringVar(&f.color, "color", "", "New color (yellow|green|...|black or Miro-native)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Move to frame (empty string detaches from any frame)")
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
	path := "/v2/boards/" + f.boardID + "/sticky_notes/" + f.itemID
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
	if f.colorSet {
		req.Style = &styleField{FillColor: normalizeStickyColor(f.color)}
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
	if f.widthSet {
		req.Geometry = &geometryData{Width: f.width}
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

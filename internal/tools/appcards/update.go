package appcards

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set. Cobra zeroes unset float vars, so we can't
// distinguish "user passed --x=0" from "user didn't pass --x" by
// looking at the value alone. The bool *Set fields track presence.
type updateFlags struct {
	boardID     string
	itemID      string
	title       string
	description string
	status      string
	owned       bool
	color       string
	x           float64
	y           float64
	width       float64
	height      float64
	parentID    string

	titleSet       bool
	descriptionSet bool
	statusSet      bool
	ownedSet       bool
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
		Short: "Update an app card (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/app_cards/{item_id} with\n" +
			"only the fields you set. Pass an empty --parent-id to detach\n" +
			"the card from its frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.titleSet = fl.Changed("title")
			f.descriptionSet = fl.Changed("description")
			f.statusSet = fl.Changed("status")
			f.ownedSet = fl.Changed("owned")
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
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "App card ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "New title")
	cmd.Flags().StringVar(&f.description, "description", "", "New description")
	cmd.Flags().StringVar(&f.status, "status", "", "New status (disconnected|connected|disabled)")
	cmd.Flags().BoolVar(&f.owned, "owned", false, "Mark the card as owned by the calling app")
	cmd.Flags().StringVar(&f.color, "color", "", "New fill color (hex code)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width")
	cmd.Flags().Float64Var(&f.height, "height", 0, "New height")
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
	if f.statusSet {
		if err := validateStatus(f.status); err != nil {
			return err
		}
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/app_cards/" + f.itemID
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

	if f.titleSet || f.descriptionSet || f.statusSet || f.ownedSet {
		req.Data = &dataField{}
		if f.titleSet {
			req.Data.Title = f.title
		}
		if f.descriptionSet {
			req.Data.Description = f.description
		}
		if f.statusSet {
			req.Data.Status = f.status
		}
		if f.ownedSet {
			owned := f.owned
			req.Data.Owned = &owned
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

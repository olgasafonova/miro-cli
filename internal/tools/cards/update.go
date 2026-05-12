package cards

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set. Cobra zeroes unset float vars, so we can't
// distinguish "user passed --x=0" from "user didn't pass --x" by
// looking at the value alone. The bool *Set fields track presence.
// Strings rely on Flags().Changed(), since "set but empty" is a valid
// mutation for fields like --title.
type updateFlags struct {
	boardID     string
	itemID      string
	title       string
	description string
	assigneeID  string
	dueDate     string
	cardTheme   string
	x           float64
	y           float64
	width       float64
	height      float64
	parentID    string

	titleSet       bool
	descriptionSet bool
	assigneeIDSet  bool
	dueDateSet     bool
	cardThemeSet   bool
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
		Short: "Update a card (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/cards/{item_id} with only\n" +
			"the fields you set. Pass an empty --parent-id to detach the\n" +
			"card from its frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.titleSet = fl.Changed("title")
			f.descriptionSet = fl.Changed("description")
			f.assigneeIDSet = fl.Changed("assignee-id")
			f.dueDateSet = fl.Changed("due-date")
			f.cardThemeSet = fl.Changed("card-theme")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.widthSet = fl.Changed("width")
			f.heightSet = fl.Changed("height")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Card ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "New card title")
	cmd.Flags().StringVar(&f.description, "description", "", "New description")
	cmd.Flags().StringVar(&f.assigneeID, "assignee-id", "", "New assignee user ID")
	cmd.Flags().StringVar(&f.dueDate, "due-date", "", "New due date (ISO8601)")
	cmd.Flags().StringVar(&f.cardTheme, "card-theme", "", "New card theme color (hex)")
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
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	if f.itemID == "" {
		return errors.New("--item-id is required")
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/cards/" + f.itemID
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

	if f.titleSet || f.descriptionSet || f.assigneeIDSet || f.dueDateSet {
		req.Data = &dataField{}
		if f.titleSet {
			req.Data.Title = f.title
		}
		if f.descriptionSet {
			req.Data.Description = f.description
		}
		if f.assigneeIDSet {
			req.Data.AssigneeID = f.assigneeID
		}
		if f.dueDateSet {
			req.Data.DueDate = f.dueDate
		}
		any = true
	}
	if f.cardThemeSet {
		req.Style = &styleField{CardTheme: f.cardTheme}
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

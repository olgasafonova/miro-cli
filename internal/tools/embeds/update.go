package embeds

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
	boardID    string
	itemID     string
	url        string
	mode       string
	previewURL string
	x          float64
	y          float64
	width      float64
	height     float64
	parentID   string

	urlSet        bool
	modeSet       bool
	previewURLSet bool
	xSet          bool
	ySet          bool
	widthSet      bool
	heightSet     bool
	parentIDSet   bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an embed (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/embeds/{item_id} with only\n" +
			"the fields you set. Pass an empty --parent-id to detach the\n" +
			"embed from its frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.urlSet = fl.Changed("url")
			f.modeSet = fl.Changed("mode")
			f.previewURLSet = fl.Changed("preview-url")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.widthSet = fl.Changed("width")
			f.heightSet = fl.Changed("height")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Embed ID (required)")
	cmd.Flags().StringVar(&f.url, "url", "", "New embed source URL")
	cmd.Flags().StringVar(&f.mode, "mode", "", "New display mode (inline|modal)")
	cmd.Flags().StringVar(&f.previewURL, "preview-url", "", "New preview image URL")
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
	if err := validateUpdateArgs(f); err != nil {
		return err
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/embeds/" + f.itemID
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

// validateUpdateArgs checks the identifying IDs and any enum-valued
// flags before the request body is built.
func validateUpdateArgs(f updateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", f.itemID); err != nil {
		return err
	}
	if f.modeSet {
		return validateMode(f.mode)
	}
	return nil
}

// buildUpdateRequest projects the updateFlags into the wire body and
// reports whether any field was set. ok=false means the caller should
// reject the update — Miro 400s an empty PATCH body anyway, and a
// pre-flight check produces a clearer error.
func buildUpdateRequest(f updateFlags) (updateRequest, bool) {
	var req updateRequest
	applied := f.applyData(&req)
	applied = f.applyPosition(&req) || applied
	applied = f.applyGeometry(&req) || applied
	applied = f.applyParent(&req) || applied
	return req, applied
}

// dataChanged reports whether any field of the data envelope was set.
func (f updateFlags) dataChanged() bool {
	return f.urlSet || f.modeSet || f.previewURLSet
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
	if f.urlSet {
		req.Data.URL = f.url
	}
	if f.modeSet {
		req.Data.Mode = f.mode
	}
	if f.previewURLSet {
		req.Data.PreviewURL = f.previewURL
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
	req.Geometry = &geometryData{}
	if f.widthSet {
		req.Geometry.Width = f.width
	}
	if f.heightSet {
		req.Geometry.Height = f.height
	}
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

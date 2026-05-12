package tags

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set. Cobra zeroes unset string vars, so we can't
// distinguish "user passed --title=”" from "user didn't pass --title"
// by looking at the value alone. The bool *Set fields track presence.
type updateFlags struct {
	boardID   string
	tagID     string
	title     string
	fillColor string

	titleSet     bool
	fillColorSet bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a tag (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/tags/{tag_id} with only the\n" +
			"fields you set. At least one of --title or --fill-color is\n" +
			"required.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.titleSet = fl.Changed("title")
			f.fillColorSet = fl.Changed("fill-color")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.tagID, "tag-id", "", "Tag ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "New tag title")
	cmd.Flags().StringVar(&f.fillColor, "fill-color", "", "New fill color")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("tag-id")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	if f.tagID == "" {
		return errors.New("--tag-id is required")
	}
	if f.fillColorSet {
		if err := validateFillColor(f.fillColor); err != nil {
			return err
		}
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/tags/" + f.tagID
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
	if f.titleSet {
		t := f.title
		req.Title = &t
		any = true
	}
	if f.fillColorSet {
		c := f.fillColor
		req.FillColor = &c
		any = true
	}
	return req, any
}

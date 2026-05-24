package items

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newGetTagsCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		itemID  string
	)
	cmd := &cobra.Command{
		Use:   "get-tags",
		Short: "List tags attached to an item",
		Long: "Calls GET /v2/boards/{board_id}/items/{item_id}/tags. Returns\n" +
			"the tag envelope verbatim; typically a {tags: [...]} array.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetTags(cmd.Context(), g, boardID, itemID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "Item ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runGetTags(ctx context.Context, g *clictx.Globals, boardID, itemID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", itemID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID + "/items/" + itemID + "/tags"
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

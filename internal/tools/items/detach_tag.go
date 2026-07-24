package items

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newDetachTagCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		itemID  string
		tagID   string
	)
	cmd := &cobra.Command{
		Use:   "detach-tag",
		Short: "Detach a tag from an item (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}/items/{item_id}?tag_id=X.\n" +
			"The tag itself stays on the board; only the association is\n" +
			"removed.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetachTag(cmd.Context(), g, detachTagParams{
				boardID: boardID,
				itemID:  itemID,
				tagID:   tagID,
			})
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "Item ID (required)")
	cmd.Flags().StringVar(&tagID, "tag-id", "", "Tag ID to detach (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	_ = cmd.MarkFlagRequired("tag-id")
	return cmd
}

// detachTagParams bundles the runDetachTag inputs: the board, the item
// carrying the tag, and the tag to detach.
type detachTagParams struct {
	boardID string
	itemID  string
	tagID   string
}

func runDetachTag(ctx context.Context, g *clictx.Globals, p detachTagParams) error {
	if err := miro.ValidateID("board_id", p.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", p.itemID); err != nil {
		return err
	}
	if err := miro.ValidateID("tag_id", p.tagID); err != nil {
		return err
	}
	q := url.Values{}
	q.Set("tag_id", p.tagID)
	path := "/v2/boards/" + p.boardID + "/items/" + p.itemID + "?" + q.Encode()
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to detach tag without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(detachTagResult{Detached: true, ItemID: p.itemID, TagID: p.tagID})
}

package items

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
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
			return runDetachTag(cmd.Context(), g, boardID, itemID, tagID)
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

func runDetachTag(ctx context.Context, g *clictx.Globals, boardID, itemID, tagID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", itemID); err != nil {
		return err
	}
	if err := miro.ValidateID("tag_id", tagID); err != nil {
		return err
	}
	q := url.Values{}
	q.Set("tag_id", tagID)
	path := "/v2/boards/" + boardID + "/items/" + itemID + "?" + q.Encode()
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
	return g.EmitJSON(detachTagResult{Detached: true, ItemID: itemID, TagID: tagID})
}

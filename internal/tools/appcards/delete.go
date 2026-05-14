package appcards

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newDeleteCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		itemID  string
	)
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an app card (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}/app_cards/{item_id}.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), g, boardID, itemID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "App card ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runDelete(ctx context.Context, g *clictx.Globals, boardID, itemID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", itemID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID + "/app_cards/" + itemID
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to delete app card without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(deleteResult{Deleted: true, ID: itemID})
}

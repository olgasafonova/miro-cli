package members

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newRemoveCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID  string
		memberID string
	)
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a board member (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}/members/{board_member_id},\n" +
			"revoking that user's access to the board. The user account\n" +
			"itself is unaffected.\n\n" +
			"--member-id maps to the API's board_member_id path parameter.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd.Context(), g, boardID, memberID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&memberID, "member-id", "", "Board member ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("member-id")
	return cmd
}

func runRemove(ctx context.Context, g *clictx.Globals, boardID, memberID string) error {
	if boardID == "" {
		return errors.New("--board-id is required")
	}
	if memberID == "" {
		return errors.New("--member-id is required")
	}
	path := "/v2/boards/" + boardID + "/members/" + memberID
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to remove board member without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(removeResult{Removed: true, ID: memberID})
}

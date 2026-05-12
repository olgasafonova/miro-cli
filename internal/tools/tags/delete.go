package tags

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newDeleteCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		tagID   string
	)
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a tag (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}/tags/{tag_id}.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), g, boardID, tagID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&tagID, "tag-id", "", "Tag ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("tag-id")
	return cmd
}

func runDelete(ctx context.Context, g *clictx.Globals, boardID, tagID string) error {
	if boardID == "" {
		return errors.New("--board-id is required")
	}
	if tagID == "" {
		return errors.New("--tag-id is required")
	}
	path := "/v2/boards/" + boardID + "/tags/" + tagID
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to delete tag without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(deleteResult{Deleted: true, ID: tagID})
}

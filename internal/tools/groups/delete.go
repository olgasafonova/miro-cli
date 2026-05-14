package groups

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// deleteFlags captures the per-invocation knobs for `miro groups delete`.
// --delete-items mirrors the API's `delete_items` query param: by default
// only the grouping relationship is removed and the items remain on the
// board; passing --delete-items removes the items too.
type deleteFlags struct {
	boardID     string
	groupID     string
	deleteItems bool
}

func newDeleteCmd(g *clictx.Globals) *cobra.Command {
	var f deleteFlags
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Ungroup items (destructive: removes the group association)",
		Long: "Calls DELETE /v2/boards/{board_id}/groups/{group_id}.\n\n" +
			"By default the items remain on the board and only the\n" +
			"grouping relationship is removed; pass --delete-items to\n" +
			"also delete the items themselves.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.groupID, "group-id", "", "Group ID (required)")
	cmd.Flags().BoolVar(&f.deleteItems, "delete-items", false, "Also delete the items, not just the group association")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("group-id")
	return cmd
}

func runDelete(ctx context.Context, g *clictx.Globals, f deleteFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("group_id", f.groupID); err != nil {
		return err
	}
	path := buildDeletePath(f)
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to delete group without --yes; pass --yes to confirm or --dry-run to preview. Items remain on the board; only the group association is removed."}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(deleteResult{Deleted: true, ID: f.groupID, DeleteItems: f.deleteItems})
}

// buildDeletePath assembles path + optional delete_items query. Split
// out for tests.
func buildDeletePath(f deleteFlags) string {
	path := "/v2/boards/" + f.boardID + "/groups/" + f.groupID
	if f.deleteItems {
		q := url.Values{}
		q.Set("delete_items", "true")
		path += "?" + q.Encode()
	}
	return path
}

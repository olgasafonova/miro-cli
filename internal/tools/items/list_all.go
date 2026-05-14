package items

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// listAllFlags drives the paginate-everything wrapper. It reuses
// FetchAll under the hood, so the DefaultFetchAllCap (5000) still
// applies. Callers that genuinely need more should drive the list verb
// + cursor themselves.
type listAllFlags struct {
	boardID  string
	itemType string
}

func newListAllCmd(g *clictx.Globals) *cobra.Command {
	var f listAllFlags
	cmd := &cobra.Command{
		Use:   "list-all",
		Short: "List every item on a board (paginate-everything)",
		Long: "Walks the cursor-paginated GET /v2/boards/{board_id}/items\n" +
			"until exhaustion or the safety cap (DefaultFetchAllCap = 5000)\n" +
			"and emits {items: [...], total: N, truncated: bool}.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListAll(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemType, "type", "", "Filter by item type")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runListAll(ctx context.Context, g *clictx.Globals, f listAllFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	lf := ListFlags{BoardID: f.boardID, ItemType: f.itemType}
	if g.DryRun {
		return g.EmitDryRun("GET", BuildListPath(lf)+" (paginated)")
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	all, truncated, err := FetchAll(ctx, client, lf, FetchAllOptions{})
	if err != nil {
		return err
	}
	return g.EmitJSON(listAllResponse{Items: all, Total: len(all), Truncated: truncated})
}

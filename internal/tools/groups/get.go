package groups

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		groupID string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single group by ID",
		Long: "Calls GET /v2/boards/{board_id}/groups/{group_id} and prints\n" +
			"the response. Returns the group's id, type, and member item IDs.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, boardID, groupID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&groupID, "group-id", "", "Group ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("group-id")
	return cmd
}

func runGet(ctx context.Context, g *clictx.Globals, boardID, groupID string) error {
	if boardID == "" {
		return errors.New("--board-id is required")
	}
	if groupID == "" {
		return errors.New("--group-id is required")
	}
	path := "/v2/boards/" + boardID + "/groups/" + groupID
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

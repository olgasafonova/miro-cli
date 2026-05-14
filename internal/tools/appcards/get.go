package appcards

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		itemID  string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single app card",
		Long: "Calls GET /v2/boards/{board_id}/app_cards/{item_id} and\n" +
			"prints the response. Returns data, style, geometry, position,\n" +
			"and parent.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, boardID, itemID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "App card ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runGet(ctx context.Context, g *clictx.Globals, boardID, itemID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", itemID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID + "/app_cards/" + itemID
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

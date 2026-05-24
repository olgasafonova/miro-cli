package tables

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		itemID  string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single table by item ID",
		Long: "Calls GET /v2/boards/{board_id}/data_table_formats/{item_id}\n" +
			"and prints the response. Returns the table's id, type, position,\n" +
			"geometry, parent (if framed), and audit timestamps.\n\n" +
			"Discover --item-id via `miro tables list` or `miro items list`\n" +
			"with --type data_table_format.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, boardID, itemID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "Table item ID (required)")
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
	path := "/v2/boards/" + boardID + "/data_table_formats/" + itemID
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

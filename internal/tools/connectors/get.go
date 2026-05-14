package connectors

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID     string
		connectorID string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single connector",
		Long: "Calls GET /v2/boards/{board_id}/connectors/{connector_id} and\n" +
			"prints the response. Returns startItem, endItem, shape, style, and\n" +
			"captions for the connector.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, boardID, connectorID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&connectorID, "connector-id", "", "Connector ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("connector-id")
	return cmd
}

func runGet(ctx context.Context, g *clictx.Globals, boardID, connectorID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("connector_id", connectorID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID + "/connectors/" + connectorID
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

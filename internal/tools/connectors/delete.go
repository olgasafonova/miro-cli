package connectors

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newDeleteCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID     string
		connectorID string
	)
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a connector (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}/connectors/{connector_id}.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), g, boardID, connectorID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&connectorID, "connector-id", "", "Connector ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("connector-id")
	return cmd
}

func runDelete(ctx context.Context, g *clictx.Globals, boardID, connectorID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("connector_id", connectorID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID + "/connectors/" + connectorID
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to delete connector without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(deleteResult{Deleted: true, ID: connectorID})
}

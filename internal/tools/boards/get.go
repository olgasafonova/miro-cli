package boards

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <board_id>",
		Short: "Get a single board's metadata",
		Long: "Calls GET /v2/boards/{board_id} and prints the response.\n\n" +
			"Returns name, description, owner, sharing policy, and view link.\n" +
			"For an item-count summary use `boards summary` (Phase 3a follow-up).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, args[0])
		},
	}
	return cmd
}

func runGet(ctx context.Context, g *clictx.Globals, boardID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID
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

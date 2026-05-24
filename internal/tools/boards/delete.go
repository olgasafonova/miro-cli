package boards

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newDeleteCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <board_id>",
		Short: "Delete a board (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}.\n\n" +
			"Destructive operation: requires --yes (or --agent, which implies it)\n" +
			"to proceed. Without --yes, the command refuses. Use --dry-run to\n" +
			"preview the request without sending it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), g, args[0])
		},
	}
	return cmd
}

func runDelete(ctx context.Context, g *clictx.Globals, boardID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID
	if g.DryRun {
		return g.EmitDryRun("DELETE", path)
	}
	if !g.Yes {
		// Refuse interactively. The CLI is non-interactive by contract
		// (see SKILL.md "Non-interactive — never prompts"), so the only
		// way to confirm a destructive op is via the --yes flag (or
		// --agent, which Normalize() expands to imply --yes).
		return &miro.ConfigError{Reason: "refusing to delete board without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	if err := client.Delete(ctx, path); err != nil {
		return err
	}
	return g.EmitJSON(deleteResult{Deleted: true, ID: boardID})
}

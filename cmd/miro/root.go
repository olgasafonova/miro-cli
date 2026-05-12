// Package main hosts the hand-authored `miro` CLI. The root command
// owns persistent flags shared by every subcommand and registers per-
// resource subcommand trees from internal/tools/.
package main

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/boards"
	"miro-cli/internal/tools/clictx"
)

// newRootCmd builds the root *cobra.Command and the Globals it backs.
// Returning both lets main override Stdout/Stderr for tests later, and
// keeps the wiring testable from within the package.
func newRootCmd() (*cobra.Command, *clictx.Globals) {
	g := clictx.New()
	cmd := &cobra.Command{
		Use:           "miro",
		Short:         "Hand-authored CLI for the Miro REST API",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			g.Normalize()
			return nil
		},
	}

	pf := cmd.PersistentFlags()
	pf.StringVar(&g.Token, "token", "", "Miro API access token (overrides $MIRO_ACCESS_TOKEN)")
	pf.BoolVar(&g.JSON, "json", false, "Force JSON output (default when piped)")
	pf.BoolVar(&g.DryRun, "dry-run", false, "Print the request the command would send and exit")
	pf.BoolVar(&g.Agent, "agent", false, "Agent mode: implies --json and --yes")
	pf.BoolVar(&g.Yes, "yes", false, "Skip confirmation prompts on destructive operations")
	pf.BoolVar(&g.Idempotent, "idempotent", false, "Treat already-exists as success on create, already-gone as success on delete")
	pf.StringVar(&g.Select, "select", "", "Comma-separated list of top-level fields to keep in JSON output")

	cmd.AddCommand(boards.NewCmd(g))
	return cmd, g
}

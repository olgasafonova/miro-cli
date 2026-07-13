// Package main hosts the hand-authored `miro` CLI. The root command
// owns persistent flags shared by every subcommand and registers per-
// resource subcommand trees from internal/tools/.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/appcards"
	"github.com/olgasafonova/miro-cli/internal/tools/audit"
	"github.com/olgasafonova/miro-cli/internal/tools/boards"
	"github.com/olgasafonova/miro-cli/internal/tools/cards"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
	"github.com/olgasafonova/miro-cli/internal/tools/codewidgets"
	"github.com/olgasafonova/miro-cli/internal/tools/connectors"
	"github.com/olgasafonova/miro-cli/internal/tools/documents"
	"github.com/olgasafonova/miro-cli/internal/tools/embeds"
	"github.com/olgasafonova/miro-cli/internal/tools/exports"
	"github.com/olgasafonova/miro-cli/internal/tools/frames"
	"github.com/olgasafonova/miro-cli/internal/tools/groups"
	"github.com/olgasafonova/miro-cli/internal/tools/images"
	"github.com/olgasafonova/miro-cli/internal/tools/items"
	"github.com/olgasafonova/miro-cli/internal/tools/members"
	"github.com/olgasafonova/miro-cli/internal/tools/mindmap"
	"github.com/olgasafonova/miro-cli/internal/tools/query"
	"github.com/olgasafonova/miro-cli/internal/tools/shapes"
	"github.com/olgasafonova/miro-cli/internal/tools/stickies"
	"github.com/olgasafonova/miro-cli/internal/tools/sync"
	"github.com/olgasafonova/miro-cli/internal/tools/tables"
	"github.com/olgasafonova/miro-cli/internal/tools/tags"
	"github.com/olgasafonova/miro-cli/internal/tools/texts"
)

// version is the build version, injected at release time via
// -X main.version={{.Version}} (see .goreleaser.yaml). It stays "dev" for
// plain `go build`/`go install` builds so an agent can tell a local build
// from a tagged release.
var version = "dev"

// newVersionCmd prints the installed build version. Plain text by default,
// or a machine-readable {"installed_version": ...} object under --json /
// --agent so an agent can record exactly which binary it is driving
// (Agent-Friendly CLI Checklist §8).
func newVersionCmd(g *clictx.Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the installed miro-cli version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if g.JSON {
				return g.EmitJSON(map[string]string{"installed_version": version})
			}
			_, err := fmt.Fprintf(g.Stdout, "miro-cli %s\n", version)
			return err
		},
	}
}

// newRootCmd builds the root *cobra.Command and the Globals it backs.
// Returning both lets main override Stdout/Stderr for tests later, and
// keeps the wiring testable from within the package.
func newRootCmd() (*cobra.Command, *clictx.Globals) {
	g := clictx.New()
	cmd := &cobra.Command{
		Use:           "miro",
		Short:         "Hand-authored CLI for the Miro REST API",
		Version:       version,
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
	pf.Float64Var(&g.RateLimit, "rate-limit", -1, "Requests/second cap (0 disables; negative uses the conservative default that stays under Miro's per-org tier-1 budget)")
	pf.IntVar(&g.Concurrency, "concurrency", 1, "Max in-flight requests for bulk commands (1 = sequential; higher only speeds things up when --rate-limit is also raised above the default)")
	pf.DurationVar(&g.CacheTTL, "cache-ttl", -1, "Freshness window for the GET response cache (0 disables; negative uses the package default of 60s)")
	pf.BoolVar(&g.NoCache, "no-cache", false, "Bypass the GET response cache for this invocation")
	pf.StringVar(&g.StorePath, "store-path", "", "Override the default local-store path (defaults to $XDG_DATA_HOME/miro-cli/store.db)")

	cmd.AddCommand(appcards.NewCmd(g))
	cmd.AddCommand(audit.NewCmd(g))
	cmd.AddCommand(boards.NewCmd(g))
	cmd.AddCommand(cards.NewCmd(g))
	cmd.AddCommand(codewidgets.NewCmd(g))
	cmd.AddCommand(connectors.NewCmd(g))
	cmd.AddCommand(documents.NewCmd(g))
	cmd.AddCommand(embeds.NewCmd(g))
	cmd.AddCommand(exports.NewCmd(g))
	cmd.AddCommand(frames.NewCmd(g))
	cmd.AddCommand(groups.NewCmd(g))
	cmd.AddCommand(images.NewCmd(g))
	cmd.AddCommand(items.NewCmd(g))
	cmd.AddCommand(members.NewCmd(g))
	cmd.AddCommand(mindmap.NewCmd(g))
	cmd.AddCommand(query.NewCmd(g))
	cmd.AddCommand(shapes.NewCmd(g))
	cmd.AddCommand(stickies.NewCmd(g))
	cmd.AddCommand(sync.NewCmd(g))
	cmd.AddCommand(tables.NewCmd(g))
	cmd.AddCommand(tags.NewCmd(g))
	cmd.AddCommand(texts.NewCmd(g))
	cmd.AddCommand(newVersionCmd(g))
	cmd.AddCommand(newAuthCmd(g))
	return cmd, g
}

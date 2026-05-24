// Package audit holds the hand-authored Cobra subcommands for the
// /v2/audit/logs endpoint (Enterprise audit log retrieval). Phase 3c
// misc ships the single list-logs verb on the same pattern as
// internal/tools/items/ and internal/tools/embeds/.
package audit

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `audit` parent command. Single subcommand
// `list-logs` covers GET /v2/audit/logs.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Retrieve Enterprise audit logs",
	}
	cmd.AddCommand(newListLogsCmd(g))
	return cmd
}

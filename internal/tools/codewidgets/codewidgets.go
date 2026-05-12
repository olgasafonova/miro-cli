// Package codewidgets holds the hand-authored Cobra subcommands for the
// /v2-experimental/boards/{board_id}/code_widgets endpoint family.
// Phase 3c misc ships the single list verb on the same pattern as
// internal/tools/items/ and internal/tools/embeds/.
//
// Note: code widgets live under /v2-experimental, not /v2; the API
// surface may evolve before promotion.
package codewidgets

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `codewidgets` parent command. Single subcommand
// `list` covers GET /v2-experimental/boards/{board_id}/code_widgets.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codewidgets",
		Short: "Manage code widget items on a Miro board (v2-experimental)",
	}
	cmd.AddCommand(newListCmd(g))
	return cmd
}

package stickies

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `stickies` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/sticky_notes.
// The sticky-grid composite ships separately in Phase 3c alongside the
// items bulk verbs.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stickies",
		Short: "Manage sticky notes on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

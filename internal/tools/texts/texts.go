package texts

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `texts` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/texts.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "texts",
		Short: "Manage free-floating text items on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

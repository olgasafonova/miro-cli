package stickies

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `stickies` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/sticky_notes;
// create-grid composes a sticky grid via /v2/boards/{board_id}/items/bulk.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stickies",
		Short: "Manage sticky notes on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newCreateGridCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

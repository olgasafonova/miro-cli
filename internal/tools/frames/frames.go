package frames

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `frames` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/frames.
// Frames are container items; nesting other items inside a frame is
// done through each child item's --parent-id flag, not here.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frames",
		Short: "Manage frames (containers) on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

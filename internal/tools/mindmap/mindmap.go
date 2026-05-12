package mindmap

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `mindmap` parent command. Phase 3c ships
// list/create/get/delete against /v2-experimental/boards/{board_id}/mindmap_nodes
// on the same 4-verb pattern as internal/tools/embeds/ minus update —
// the Miro API does not expose a mindmap update verb today.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mindmap",
		Short: "Manage mind map nodes on a Miro board (v2-experimental)",
	}
	cmd.AddCommand(
		newListCmd(g),
		newCreateCmd(g),
		newGetCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

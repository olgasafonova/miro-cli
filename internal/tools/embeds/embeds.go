package embeds

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `embeds` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/embeds on the
// same pattern as internal/tools/stickies/ and internal/tools/cards/.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embeds",
		Short: "Manage embedded external content (YouTube, Figma, Loom, etc.) on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

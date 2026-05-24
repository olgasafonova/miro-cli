package tags

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `tags` parent command. Phase 3c ships
// list/create/get/update/delete against /v2/boards/{board_id}/tags on
// the same pattern as internal/tools/embeds/. Tags use offset-based
// pagination instead of the cursor pagination most item endpoints use.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage tags on a Miro board",
	}
	cmd.AddCommand(
		newListCmd(g),
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

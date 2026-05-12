package documents

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `documents` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/documents on
// the same pattern as internal/tools/embeds/. URL-only variant; file
// upload via multipart/form-data is deferred to Phase 4.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "documents",
		Short: "Manage external documents on a Miro board (URL-based; file-upload deferred to Phase 4)",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

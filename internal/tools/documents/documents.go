package documents

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `documents` parent command. Ships
// create/get/update/delete against /v2/boards/{board_id}/documents on
// the same pattern as internal/tools/embeds/, plus upload /
// update-from-file for the multipart/form-data variants that send a
// local file to Miro.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "documents",
		Short: "Manage external documents on a Miro board (URL-based or file-upload)",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newUploadCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newUpdateFromFileCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

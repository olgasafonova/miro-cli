package documents

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `documents` parent command. Ships
// create/get/update/delete against /v2/boards/{board_id}/documents on
// the same pattern as internal/tools/embeds/, plus upload /
// update-from-file for the multipart/form-data variants that send a
// local file to Miro, plus the *-doc verbs for the separate
// /v2/boards/{board_id}/docs resource (Markdown rich-text docs).
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "documents",
		Short: "Manage external documents on a Miro board (URL-based or file-upload)",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newCreateDocCmd(g),
		newGetDocCmd(g),
		newUpdateDocCmd(g),
		newDeleteDocCmd(g),
		newUploadCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newUpdateFromFileCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

// docPath addresses one doc-format item on the /docs resource (the
// Markdown rich-text family, distinct from /documents).
func docPath(boardID, itemID string) string {
	return "/v2/boards/" + boardID + "/docs/" + itemID
}

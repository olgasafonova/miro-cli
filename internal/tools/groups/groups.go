package groups

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `groups` parent command. Six verbs against
// /v2/boards/{board_id}/groups and /v2/boards/{board_id}/groups/items
// on the same pattern as internal/tools/embeds/ and internal/tools/connectors/.
//
// Groups gather two or more existing items so they move and resize as one
// unit. The API treats groups as their own resource: list/get/get-items are
// reads, create/update/delete shape membership. The destructive verb
// `delete` only removes the group association — items themselves stay on
// the board unless --delete-items is passed.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Manage groups of items on a Miro board",
	}
	cmd.AddCommand(
		newListCmd(g),
		newCreateCmd(g),
		newGetCmd(g),
		newGetItemsCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

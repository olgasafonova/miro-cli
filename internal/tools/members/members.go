package members

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `members` parent command. Phase 3c ships
// list/get/update/remove against /v2/boards/{board_id}/members on the
// same pattern as internal/tools/embeds/ and the other typed-item
// packages. The "share" verb (POST /v2/boards/{board_id}/members) lives
// in internal/tools/boards/ alongside the other board-level operations
// and is intentionally not re-exposed here.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Manage board members (list, get, update role, remove access)",
	}
	cmd.AddCommand(
		newListCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newRemoveCmd(g),
	)
	return cmd
}

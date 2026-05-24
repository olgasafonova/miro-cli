package cards

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `cards` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/cards on the
// same idiom as internal/tools/stickies/ — one file per verb,
// table-driven tests against httptest, JSON output through
// clictx.Globals.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cards",
		Short: "Manage cards on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

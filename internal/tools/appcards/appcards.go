package appcards

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `app-cards` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/app_cards on the
// same idiom as internal/tools/stickies/ — one file per verb,
// table-driven tests against httptest, JSON output through
// clictx.Globals.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app-cards",
		Short: "Manage app cards on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

package connectors

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `connectors` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/connectors on the
// same pattern as internal/tools/embeds/, with one shape difference:
// connectors have no position/parent/geometry envelopes. They attach to
// two items by ID with optional snapTo or relative position, carry an
// optional style block, and an optional captions array.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connectors",
		Short: "Manage connectors (lines between items) on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newGetCmd(g),
		newListCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

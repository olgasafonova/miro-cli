// Package tables holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/data_table_formats/* family of Miro REST
// endpoints. Two read-only verbs at the moment — get and list — same
// pattern as internal/tools/groups/ for the cursor-paginated list and
// internal/tools/items/ for the single-item get.
//
// Naming: the user-facing CLI verb is `tables` (the term Miro UI uses,
// and what users search for), but the wire resource is named
// `data_table_formats` (legacy plural, kept for API stability). Every
// path string in this package uses the wire name.
package tables

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `tables` parent command.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tables",
		Short: "Inspect data-table items on a Miro board",
	}
	cmd.AddCommand(
		newListCmd(g),
		newGetCmd(g),
	)
	return cmd
}

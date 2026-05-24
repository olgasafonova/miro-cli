package shapes

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `shapes` parent command. Phase 3b ships
// create/get/update/delete against /v2/boards/{board_id}/shapes.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shapes",
		Short: "Manage shapes (rectangles, circles, etc.) on a Miro board",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newCreateFlowchartCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

// Package canvas holds the SVG bridge verbs: render a board's items to
// a plain SVG document (`canvas read-svg`) and create board items from
// a constrained SVG subset (`canvas create-from-svg`).
//
// Both directions are computed locally — read-svg needs nothing beyond
// the item listing, create-from-svg posts to the ordinary /shapes and
// /texts endpoints. The renderer and parser are ported from
// miro-mcp-server miro/svg_read.go + svg_create.go (same author),
// retyped over the CLI's map-shaped item listing; the two repos
// deliberately duplicate this ~600-line stdlib-only transform rather
// than sharing a module (bead miro-cli-hpv records the decision).
package canvas

import (
	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `canvas` parent command with the two SVG verbs.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "canvas",
		Short: "Render a board as SVG, or create items from an SVG document",
	}
	cmd.AddCommand(
		newReadSVGCmd(g),
		newCreateFromSVGCmd(g),
	)
	return cmd
}

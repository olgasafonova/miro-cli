package canvas

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
	"github.com/olgasafonova/miro-cli/internal/tools/connectors"
	"github.com/olgasafonova/miro-cli/internal/tools/items"
)

// maxSVGItems caps the item fetch for a render.
const maxSVGItems = 2000

// defaultSVGItems is the fetch size when the caller doesn't specify one.
const defaultSVGItems = 500

// connectorFetchLimit is the page size for the best-effort connector
// fetch (single page; connectors are decoration, not structure).
const connectorFetchLimit = 50

// readSVGFlags captures the per-invocation knobs for `miro canvas
// read-svg`.
type readSVGFlags struct {
	boardID  string
	maxItems int
}

// readSVGResult is the JSON envelope `canvas read-svg` emits. SVG is
// the whole document; pipe through `jq -r .svg` to write a .svg file.
type readSVGResult struct {
	SVG       string `json:"svg"`
	ItemCount int    `json:"item_count"`
	Skipped   int    `json:"skipped"`
	Truncated bool   `json:"truncated,omitempty"`
	Message   string `json:"message"`
}

func newReadSVGCmd(g *clictx.Globals) *cobra.Command {
	var f readSVGFlags
	cmd := &cobra.Command{
		Use:   "read-svg",
		Short: "Render a board's items as an SVG document",
		Long: "Lists the board's items (and, best-effort, its connectors) and\n" +
			"renders them locally as a plain SVG document — no image export\n" +
			"job, no rendering service. Frames become dashed outlines, shapes\n" +
			"and stickies become rects/ellipses, text becomes <text>,\n" +
			"connectors become lines between item centers. Items without\n" +
			"drawable geometry are counted in `skipped`.\n\n" +
			"Output: { svg, item_count, skipped, truncated, message }.\n" +
			"Extract the document with `--select svg` or `jq -r .svg`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReadSVG(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&f.maxItems, "max-items", 0,
		fmt.Sprintf("Item fetch cap (1-%d; 0 = default %d)", maxSVGItems, defaultSVGItems))
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

// clampSVGMaxItems normalizes the requested item cap.
func clampSVGMaxItems(n int) int {
	if n > 0 && n <= maxSVGItems {
		return n
	}
	return defaultSVGItems
}

func runReadSVG(ctx context.Context, g *clictx.Globals, f readSVGFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	lf := items.ListFlags{BoardID: f.boardID}
	if g.DryRun {
		return g.EmitDryRun("GET", items.BuildListPath(lf)+" (paginated, rendered locally)")
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	rawItems, truncated, err := items.FetchAll(ctx, client, lf, items.FetchAllOptions{MaxItems: clampSVGMaxItems(f.maxItems)})
	if err != nil {
		return fmt.Errorf("failed to list items: %w", err)
	}

	// Connectors are best-effort decoration; a failure there shouldn't
	// sink the render.
	var conns []boardConnector
	if resp, connErr := connectors.Fetch(ctx, client, connectors.ListFlags{BoardID: f.boardID, Limit: connectorFetchLimit}); connErr == nil {
		conns = parseBoardConnectors(resp.Data)
	}

	svg, rendered, skipped := renderBoardSVG(parseBoardItems(rawItems), conns)
	return g.EmitJSON(readSVGResult{
		SVG:       svg,
		ItemCount: rendered,
		Skipped:   skipped,
		Truncated: truncated,
		Message:   fmt.Sprintf("Rendered %d item(s) as SVG (%d skipped)", rendered, skipped),
	})
}

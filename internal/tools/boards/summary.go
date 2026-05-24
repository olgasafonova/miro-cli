package boards

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
	"github.com/olgasafonova/miro-cli/internal/tools/items"
)

type summaryResult struct {
	Board        map[string]any `json:"board"`
	TotalItems   int            `json:"total_items"`
	CountsByType map[string]int `json:"counts_by_type"`
	Truncated    bool           `json:"truncated,omitempty"`
}

func newSummaryCmd(g *clictx.Globals) *cobra.Command {
	var maxItems int
	cmd := &cobra.Command{
		Use:   "summary <board_id>",
		Short: "Board metadata + item-count statistics",
		Long: "Composite: GET /v2/boards/{id} + GET /v2/boards/{id}/items\n" +
			"(paginated). Returns the board envelope plus total_items and\n" +
			"counts_by_type (sticky_note: 12, shape: 4, ...). Useful for an\n" +
			"agent's first look at an unfamiliar board.\n\n" +
			"Pagination is capped via --max-items (default 5000) — Miro\n" +
			"boards in the wild can hold tens of thousands of items and\n" +
			"unbounded iteration burns rate-limit quota.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSummary(cmd.Context(), g, args[0], maxItems)
		},
	}
	cmd.Flags().IntVar(&maxItems, "max-items", items.DefaultFetchAllCap, "Maximum items to scan")
	return cmd
}

func runSummary(ctx context.Context, g *clictx.Globals, boardID string, maxItems int) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if g.DryRun {
		return g.EmitDryRun("GET", "/v2/boards/"+boardID+" + /v2/boards/"+boardID+"/items")
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	var board map[string]any
	if err := client.Get(ctx, "/v2/boards/"+boardID, &board); err != nil {
		return err
	}

	all, truncated, err := items.FetchAll(ctx, client, items.ListFlags{BoardID: boardID}, items.FetchAllOptions{MaxItems: maxItems})
	if err != nil {
		return err
	}

	return g.EmitJSON(summaryResult{
		Board:        board,
		TotalItems:   len(all),
		CountsByType: countItemsByType(all),
		Truncated:    truncated,
	})
}

// countItemsByType aggregates the type field across an items array.
// Items without a type field land in the "" bucket — caller can decide
// whether to surface or drop those. Pure function for testability.
func countItemsByType(rawItems []map[string]any) map[string]int {
	out := make(map[string]int)
	for _, it := range rawItems {
		t, _ := it["type"].(string)
		out[t]++
	}
	return out
}

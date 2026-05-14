package boards

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
	"miro-cli/internal/tools/items"
)

// contentResult is the JSON envelope for `boards content`. Compared
// to summary it adds the raw items array so an AI agent can read or
// analyze content directly, and surfaces a frame hierarchy projection
// (just frame IDs + their child-item IDs) without the deeper connector
// /tag enrichment miro-mcp-server's full GetBoardContent produces.
// That richer projection is its own port — see bead miro-cli-8fq notes.
type contentResult struct {
	Board        map[string]any   `json:"board"`
	Items        []map[string]any `json:"items"`
	TotalItems   int              `json:"total_items"`
	CountsByType map[string]int   `json:"counts_by_type"`
	Frames       []frameSummary   `json:"frames,omitempty"`
	Truncated    bool             `json:"truncated,omitempty"`
}

// frameSummary is the per-frame projection: ID, optional title, plus
// the IDs of items that have parent.id == this frame. Phase 4 polish
// can extend this to include item content excerpts or nested frames.
type frameSummary struct {
	ID      string   `json:"id"`
	Title   string   `json:"title,omitempty"`
	ItemIDs []string `json:"item_ids,omitempty"`
}

func newContentCmd(g *clictx.Globals) *cobra.Command {
	var maxItems int
	cmd := &cobra.Command{
		Use:   "content <board_id>",
		Short: "Board metadata + items + frame hierarchy",
		Long: "Composite for AI consumption: returns the board envelope, the\n" +
			"raw items array, count-by-type aggregate, and a lightweight\n" +
			"frame hierarchy (each frame's ID + its direct-child item IDs).\n\n" +
			"This is the lite shape — connector context and tag context\n" +
			"are not loaded (heavy port, separate bead). Pagination cap is\n" +
			"--max-items (default 5000); breaches set truncated=true.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContent(cmd.Context(), g, args[0], maxItems)
		},
	}
	cmd.Flags().IntVar(&maxItems, "max-items", items.DefaultFetchAllCap, "Maximum items to fetch")
	return cmd
}

func runContent(ctx context.Context, g *clictx.Globals, boardID string, maxItems int) error {
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

	return g.EmitJSON(contentResult{
		Board:        board,
		Items:        all,
		TotalItems:   len(all),
		CountsByType: countItemsByType(all),
		Frames:       buildFrameSummaries(all),
		Truncated:    truncated,
	})
}

// buildFrameSummaries walks the items array twice: first to find every
// frame and seed a summary, second to attach children whose parent.id
// matches an existing frame. Items without a parent.id (top-level on
// the board) are not listed in any frame summary; the items[] array
// is the authoritative complete list.
//
// Pure function — tested independently from HTTP.
func buildFrameSummaries(rawItems []map[string]any) []frameSummary {
	frames := make(map[string]*frameSummary)
	order := make([]string, 0)

	for _, it := range rawItems {
		t, _ := it["type"].(string)
		if t != "frame" {
			continue
		}
		id, _ := it["id"].(string)
		if id == "" {
			continue
		}
		fs := &frameSummary{ID: id, Title: extractFrameTitle(it)}
		frames[id] = fs
		order = append(order, id)
	}

	for _, it := range rawItems {
		parentID := extractParentID(it)
		if parentID == "" {
			continue
		}
		fs, ok := frames[parentID]
		if !ok {
			continue
		}
		childID, _ := it["id"].(string)
		if childID != "" {
			fs.ItemIDs = append(fs.ItemIDs, childID)
		}
	}

	out := make([]frameSummary, 0, len(order))
	for _, id := range order {
		out = append(out, *frames[id])
	}
	return out
}

// extractFrameTitle pulls .data.title from a frame item. Falls back to
// "" if anything is missing or the wrong type.
func extractFrameTitle(frame map[string]any) string {
	data, ok := frame["data"].(map[string]any)
	if !ok {
		return ""
	}
	t, _ := data["title"].(string)
	return t
}

// extractParentID reads .parent.id from an item. Tolerates parent
// being absent, null, or the wrong type.
func extractParentID(it map[string]any) string {
	p, ok := it["parent"].(map[string]any)
	if !ok {
		return ""
	}
	id, _ := p["id"].(string)
	return id
}

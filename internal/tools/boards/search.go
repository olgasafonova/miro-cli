package boards

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
	"miro-cli/internal/tools/items"
)

// itemMatch is what `boards search` emits per matching item. Position
// is included so agents can spatially reason about results; an item
// without a position (e.g. a connector with anchor IDs) gets the
// zero-value pair, which is acceptable for our purposes since the
// match is identified by ID regardless.
type itemMatch struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Content string  `json:"content,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
}

type searchResult struct {
	BoardID string      `json:"board_id"`
	Query   string      `json:"query"`
	Matches []itemMatch `json:"matches"`
	Total   int         `json:"total"`
}

const defaultSearchLimit = 50

func newSearchCmd(g *clictx.Globals) *cobra.Command {
	var (
		query    string
		itemType string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "search <board_id>",
		Short: "Search a board's items for text content",
		Long: "Fetches items from a board (GET /v2/boards/{id}/items) and runs\n" +
			"a case-insensitive substring match on each item's content/title.\n" +
			"This is a client-side scan, not a server-side search; for large\n" +
			"boards page through with --limit. Defaults to 50 items.\n\n" +
			"--type narrows the scan to one item flavour (e.g. sticky_note,\n" +
			"shape, text). Returns { board_id, query, matches[], total }.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), g, args[0], query, itemType, limit)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Search query (required)")
	cmd.Flags().StringVar(&itemType, "type", "", "Restrict scan to one item type")
	cmd.Flags().IntVar(&limit, "limit", defaultSearchLimit, "Max items to scan")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func runSearch(ctx context.Context, g *clictx.Globals, boardID, query, itemType string, limit int) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("--query is required")
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	lf := items.ListFlags{BoardID: boardID, ItemType: itemType, Limit: limit}
	path := items.BuildListPath(lf)
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	resp, err := items.Fetch(ctx, client, lf)
	if err != nil {
		return err
	}

	matches := scanItems(resp.Data, query)
	return g.EmitJSON(searchResult{
		BoardID: boardID,
		Query:   query,
		Matches: matches,
		Total:   len(matches),
	})
}

// scanItems is the pure projection: take the raw items array Miro
// returns, find ones whose content (or fallback title) contains the
// query, return them as itemMatch records. Tested independently from
// the HTTP layer.
func scanItems(rawItems []map[string]any, query string) []itemMatch {
	q := strings.ToLower(query)
	out := make([]itemMatch, 0, len(rawItems))
	for _, it := range rawItems {
		content := extractContent(it)
		if content == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(content), q) {
			continue
		}
		m := itemMatch{
			ID:      stringField(it, "id"),
			Type:    stringField(it, "type"),
			Content: content,
			Snippet: makeSnippet(content, query, 50),
		}
		if pos, ok := it["position"].(map[string]any); ok {
			if x, ok := pos["x"].(float64); ok {
				m.X = x
			}
			if y, ok := pos["y"].(float64); ok {
				m.Y = y
			}
		}
		out = append(out, m)
	}
	return out
}

// extractContent reads .data.content, falling back to .data.title.
// Tolerates missing data field or unexpected shapes — returns "" so
// the caller treats it as a non-match.
func extractContent(it map[string]any) string {
	data, ok := it["data"].(map[string]any)
	if !ok {
		return ""
	}
	if c, ok := data["content"].(string); ok && c != "" {
		return c
	}
	if t, ok := data["title"].(string); ok {
		return t
	}
	return ""
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// makeSnippet returns a window around the first occurrence of query in
// content. window is the number of characters on each side. The match
// itself is included. Pure function so the snippet shape stays stable
// across the test suite.
func makeSnippet(content, query string, window int) string {
	q := strings.ToLower(query)
	c := strings.ToLower(content)
	idx := strings.Index(c, q)
	if idx < 0 {
		return content
	}
	start := idx - window
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + window
	if end > len(content) {
		end = len(content)
	}
	out := content[start:end]
	if start > 0 {
		out = "..." + out
	}
	if end < len(content) {
		out += "..."
	}
	return out
}

package boards

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// findResult is the JSON envelope emitted by `boards find`. Wraps the
// best-matching board with a small "match" field describing how the
// match was decided. Agents can branch on match.kind without text
// parsing.
type findResult struct {
	Board    map[string]any `json:"board"`
	Match    findMatch      `json:"match"`
	NumPeers int            `json:"num_peers,omitempty"`
}

type findMatch struct {
	Kind  string `json:"kind"` // "exact" | "prefix" | "contains" | "fallback"
	Query string `json:"query"`
}

func newFindCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Find a board by exact or partial name match",
		Long: "Calls GET /v2/boards?query=<query>&limit=20 and applies client-\n" +
			"side resolution: exact name match wins, then prefix, then\n" +
			"substring, then the first result.\n\n" +
			"Returns one board envelope: { board: {...}, match: {kind, query} }.\n" +
			"Use `boards list --query` if you want the whole filtered set.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFind(cmd.Context(), g, args[0])
		},
	}
	return cmd
}

func runFind(ctx context.Context, g *clictx.Globals, query string) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("query is required")
	}

	lf := listFlags{query: query, limit: 20}
	path := buildListPath(lf)
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp ListResponse
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	if len(resp.Data) == 0 {
		return fmt.Errorf("no board found matching %q", query)
	}

	best, kind := resolveFindMatch(resp.Data, query)
	return g.EmitJSON(findResult{
		Board:    best,
		Match:    findMatch{Kind: kind, Query: query},
		NumPeers: len(resp.Data) - 1,
	})
}

// resolveFindMatch implements the four-pass algorithm: exact, prefix,
// contains, fallback. Returns the chosen board and the match kind for
// the emitted envelope. Pure function — tested independently from the
// HTTP layer.
func resolveFindMatch(boards []map[string]any, query string) (map[string]any, string) {
	q := strings.ToLower(query)

	exact := -1
	prefix := -1
	contains := -1
	for i, b := range boards {
		name, _ := b["name"].(string)
		nameLower := strings.ToLower(name)
		switch {
		case nameLower == q:
			if exact < 0 {
				exact = i
			}
		case strings.HasPrefix(nameLower, q):
			if prefix < 0 {
				prefix = i
			}
		case strings.Contains(nameLower, q):
			if contains < 0 {
				contains = i
			}
		}
	}
	switch {
	case exact >= 0:
		return boards[exact], "exact"
	case prefix >= 0:
		return boards[prefix], "prefix"
	case contains >= 0:
		return boards[contains], "contains"
	default:
		return boards[0], "fallback"
	}
}

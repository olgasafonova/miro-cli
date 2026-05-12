package tags

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// listFlags captures the per-invocation knobs for `miro tags list`.
// Tags use offset-based pagination (not the cursor pagination most item
// endpoints use), so we expose --limit and --offset rather than
// --cursor.
type listFlags struct {
	boardID string
	limit   int
	offset  int
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tags on a board",
		Long: "Calls GET /v2/boards/{board_id}/tags. Tags use offset-based\n" +
			"pagination: pass --limit and --offset to page through.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "Starting offset (0 = first page)")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runList(ctx context.Context, g *clictx.Globals, f listFlags) error {
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	path := buildListPath(f)
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildListPath assembles the request URL for a tag list. Split out so
// tests can assert on query encoding without spinning a server.
func buildListPath(f listFlags) string {
	q := url.Values{}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.offset > 0 {
		q.Set("offset", strconv.Itoa(f.offset))
	}
	path := "/v2/boards/" + f.boardID + "/tags"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

package members

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// listFlags captures the per-invocation knobs for `miro members list`.
// Members are offset-paginated (limit + offset), not cursor-paginated;
// that's the one shape divergence from internal/tools/items/list.go.
type listFlags struct {
	boardID string
	limit   int
	offset  int
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List members of a board",
		Long: "Calls GET /v2/boards/{board_id}/members.\n\n" +
			"Offset-paginated: --limit sets page size (Miro default 20, max\n" +
			"50), --offset jumps into the result set. Unlike item lists,\n" +
			"members do not return a cursor.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default of 20)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "Zero-based offset of the first item to return")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runList(ctx context.Context, g *clictx.Globals, f listFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
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

// buildListPath assembles the request URL with optional limit/offset
// query params. Split out so tests can assert on the wire shape without
// spinning an httptest server.
func buildListPath(f listFlags) string {
	q := url.Values{}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.offset > 0 {
		q.Set("offset", strconv.Itoa(f.offset))
	}
	path := "/v2/boards/" + f.boardID + "/members"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

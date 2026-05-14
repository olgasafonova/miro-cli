package mindmap

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// listFlags captures the per-invocation knobs for `miro mindmap list`.
type listFlags struct {
	boardID string
	limit   int
	cursor  string
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List mind map nodes on a board",
		Long: "Calls GET /v2-experimental/boards/{board_id}/mindmap_nodes.\n\n" +
			"The response is cursor-paginated; pass --cursor on a follow-up\n" +
			"call to fetch the next page. --limit caps the page size.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().StringVar(&f.cursor, "cursor", "", "Cursor from a prior page")
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
	var resp listResponse
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildListPath assembles the request URL for a mindmap list. Split
// out so the unit test can assert the query string without spinning
// an httptest server.
func buildListPath(f listFlags) string {
	q := url.Values{}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.cursor != "" {
		q.Set("cursor", f.cursor)
	}
	path := "/v2-experimental/boards/" + f.boardID + "/mindmap_nodes"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

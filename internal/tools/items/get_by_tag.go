package items

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// getByTagFlags drives `items get-by-tag`. Limit/offset use the
// offset-pagination shape Miro returns for tag-filtered lookups,
// distinct from the cursor pagination on the generic list endpoint.
type getByTagFlags struct {
	boardID string
	tagID   string
	limit   int
	offset  int
}

func newGetByTagCmd(g *clictx.Globals) *cobra.Command {
	var f getByTagFlags
	cmd := &cobra.Command{
		Use:   "get-by-tag",
		Short: "List items that carry a specific tag",
		Long: "Calls GET /v2/boards/{board_id}/items?tag_id=X. Returns the\n" +
			"items that have the given tag attached. Uses offset pagination\n" +
			"(--limit / --offset), not the cursor pagination of `items list`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetByTag(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.tagID, "tag-id", "", "Tag ID to filter by (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "Offset for paging")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("tag-id")
	return cmd
}

func runGetByTag(ctx context.Context, g *clictx.Globals, f getByTagFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("tag_id", f.tagID); err != nil {
		return err
	}
	path := buildGetByTagPath(f)
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

// buildGetByTagPath assembles the URL with offset/limit/tag_id query
// params. Split out so tests can assert on the wire shape.
func buildGetByTagPath(f getByTagFlags) string {
	q := url.Values{}
	q.Set("tag_id", f.tagID)
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.offset > 0 {
		q.Set("offset", strconv.Itoa(f.offset))
	}
	return "/v2/boards/" + f.boardID + "/items?" + q.Encode()
}

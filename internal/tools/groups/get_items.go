package groups

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// getItemsFlags captures the per-invocation knobs for `miro groups
// get-items`. Note the REST path is /v2/boards/{board_id}/groups/items
// (no group_id segment); the group is identified by the required
// `group_item_id` query param.
type getItemsFlags struct {
	boardID string
	groupID string
	limit   int
	cursor  string
}

func newGetItemsCmd(g *clictx.Globals) *cobra.Command {
	var f getItemsFlags
	cmd := &cobra.Command{
		Use:   "get-items",
		Short: "Get items belonging to a group",
		Long: "Calls GET /v2/boards/{board_id}/groups/items?group_item_id=X\n" +
			"and prints the cursor-paginated response. Each entry is one\n" +
			"of the group's member items in its full typed shape.\n\n" +
			"Note: the API names the query parameter `group_item_id`; the\n" +
			"CLI flag is `--group-id` for consistency with the other group\n" +
			"verbs. Both refer to the same identifier.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetItems(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.groupID, "group-id", "", "Group ID (required; sent as group_item_id query param)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default of 10, max 50)")
	cmd.Flags().StringVar(&f.cursor, "cursor", "", "Cursor from a prior page")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("group-id")
	return cmd
}

func runGetItems(ctx context.Context, g *clictx.Globals, f getItemsFlags) error {
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	if f.groupID == "" {
		return errors.New("--group-id is required")
	}
	path := buildGetItemsPath(f)
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

// buildGetItemsPath assembles the path + query string. Split out for
// tests so we can assert on the wire shape without an httptest server.
// The CLI's --group-id is mapped onto the API's group_item_id query.
func buildGetItemsPath(f getItemsFlags) string {
	q := url.Values{}
	q.Set("group_item_id", f.groupID)
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.cursor != "" {
		q.Set("cursor", f.cursor)
	}
	return "/v2/boards/" + f.boardID + "/groups/items?" + q.Encode()
}

package items

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// getWithinFrameFlags drives `items get-within-frame`. --frame-id maps
// to the API's parent_item_id query parameter; we expose the friendlier
// name on the CLI since frames are the only items that can be parents
// in practice.
type getWithinFrameFlags struct {
	boardID  string
	frameID  string
	itemType string
	limit    int
	cursor   string
}

func newGetWithinFrameCmd(g *clictx.Globals) *cobra.Command {
	var f getWithinFrameFlags
	cmd := &cobra.Command{
		Use:   "get-within-frame",
		Short: "List items that live inside a frame",
		Long: "Calls GET /v2/boards/{board_id}/items?parent_item_id=X. Uses\n" +
			"cursor pagination — pass the response cursor on a follow-up\n" +
			"call with --cursor to fetch the next page.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetWithinFrame(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.frameID, "frame-id", "", "Frame ID (parent_item_id; required)")
	cmd.Flags().StringVar(&f.itemType, "type", "", "Filter by item type")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().StringVar(&f.cursor, "cursor", "", "Cursor from a prior page")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("frame-id")
	return cmd
}

func runGetWithinFrame(ctx context.Context, g *clictx.Globals, f getWithinFrameFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("frame_id", f.frameID); err != nil {
		return err
	}
	path := buildGetWithinFramePath(f)
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

func buildGetWithinFramePath(f getWithinFrameFlags) string {
	q := url.Values{}
	q.Set("parent_item_id", f.frameID)
	if f.itemType != "" {
		q.Set("type", f.itemType)
	}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.cursor != "" {
		q.Set("cursor", f.cursor)
	}
	return "/v2/boards/" + f.boardID + "/items?" + q.Encode()
}

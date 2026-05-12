package items

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `items` parent command. Phase 3a's boards
// composites import List/Fetch directly; the CLI surface (subcommands
// below) gets fleshed out in Phase 3c.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "items",
		Short: "Manage items on a Miro board",
	}
	cmd.AddCommand(newListCmd(g))
	return cmd
}

// ListFlags is exported so cross-package composites (e.g. boards
// search/summary/content) can drive an items list with the same shape
// the CLI uses, without re-implementing the path/query builder.
type ListFlags struct {
	BoardID  string
	ItemType string // sticky_note, shape, text, connector, frame, doc, image, embed, card, app_card
	Limit    int
	Cursor   string
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var lf ListFlags
	cmd := &cobra.Command{
		Use:   "list <board_id>",
		Short: "List items on a board",
		Long: "Calls GET /v2/boards/{board_id}/items.\n\n" +
			"--type narrows to one item flavour; omit it for all items. The\n" +
			"response is cursor-paginated; pass --cursor on a follow-up call\n" +
			"to fetch the next page.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lf.BoardID = args[0]
			return runList(cmd.Context(), g, lf)
		},
	}
	cmd.Flags().StringVar(&lf.ItemType, "type", "", "Filter by item type")
	cmd.Flags().IntVar(&lf.Limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().StringVar(&lf.Cursor, "cursor", "", "Cursor from a prior page")
	return cmd
}

func runList(ctx context.Context, g *clictx.Globals, lf ListFlags) error {
	if lf.BoardID == "" {
		return errors.New("board_id is required")
	}
	path := BuildListPath(lf)
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
	return g.EmitJSON(resp)
}

// BuildListPath assembles the request URL for an items list. Exported
// because boards composites (search/summary/content) call it directly
// to keep their HTTP behaviour identical to the items list verb.
func BuildListPath(lf ListFlags) string {
	q := url.Values{}
	if lf.ItemType != "" {
		q.Set("type", lf.ItemType)
	}
	if lf.Limit > 0 {
		q.Set("limit", strconv.Itoa(lf.Limit))
	}
	if lf.Cursor != "" {
		q.Set("cursor", lf.Cursor)
	}
	path := "/v2/boards/" + lf.BoardID + "/items"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

// Fetch is the cross-package primitive: GET the items list, return
// the decoded response. boards.search/summary/content call this
// instead of duplicating client.Get plumbing.
func Fetch(ctx context.Context, client *miro.Client, lf ListFlags) (ListResponse, error) {
	var resp ListResponse
	if err := client.Get(ctx, BuildListPath(lf), &resp); err != nil {
		return ListResponse{}, err
	}
	return resp, nil
}

// DefaultFetchAllCap is the safety ceiling on FetchAll's accumulator —
// we won't fetch more than this many items in a single FetchAll call.
// Boards in the wild can have tens of thousands of items; an unbounded
// iteration would burn the user's rate-limit quota and the agent's
// context window. Callers that genuinely need more pass a higher
// MaxItems via FetchAllOptions.
const DefaultFetchAllCap = 5000

// FetchAllOptions caps a paginate-everything traversal. MaxItems<=0
// uses DefaultFetchAllCap. PageSize<=0 lets Miro pick (currently 50).
type FetchAllOptions struct {
	MaxItems int
	PageSize int
}

// FetchAll iterates through cursor-paginated items, accumulating up to
// MaxItems items. Stops on empty cursor (end of list) or when the cap
// is hit; returns whatever it has plus a Truncated flag so callers can
// surface "we stopped early" to the user.
//
// Used by boards.summary and boards.content. Phase 4 (perf) will swap
// this for a bounded-concurrency fan-out across pages once we've
// understood Miro's rate-limit headers; today it's a serial loop, which
// is the right default for correctness.
func FetchAll(ctx context.Context, client *miro.Client, lf ListFlags, opts FetchAllOptions) (allItems []map[string]any, truncated bool, err error) {
	cap := opts.MaxItems
	if cap <= 0 {
		cap = DefaultFetchAllCap
	}
	if opts.PageSize > 0 {
		lf.Limit = opts.PageSize
	}

	for {
		if err := ctx.Err(); err != nil {
			return allItems, false, err
		}
		resp, err := Fetch(ctx, client, lf)
		if err != nil {
			return allItems, false, err
		}
		allItems = append(allItems, resp.Data...)
		if len(allItems) >= cap {
			return allItems[:cap], true, nil
		}
		if resp.Cursor == "" {
			return allItems, false, nil
		}
		lf.Cursor = resp.Cursor
	}
}

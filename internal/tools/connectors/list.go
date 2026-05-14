package connectors

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ListFlags drives a connectors list request. Exported so future
// cross-package callers (e.g. boards composites) can paginate connectors
// with the same path/query builder the CLI uses.
type ListFlags struct {
	BoardID string
	Limit   int
	Cursor  string
}

// ListResponse mirrors the cursor-paginated envelope Miro returns from
// GET /v2/boards/{board_id}/connectors. data is []map[string]any rather
// than a typed Connector because the response is wide (startItem, endItem,
// shape, style, captions, plus envelope fields) and callers branch on
// fields that vary by connector configuration.
type ListResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int              `json:"total,omitempty"`
	Size   int              `json:"size,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var lf ListFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List connectors on a board",
		Long: "Calls GET /v2/boards/{board_id}/connectors.\n\n" +
			"The response is cursor-paginated; pass --cursor on a follow-up\n" +
			"call to fetch the next page.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, lf)
		},
	}
	cmd.Flags().StringVar(&lf.BoardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&lf.Limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().StringVar(&lf.Cursor, "cursor", "", "Cursor from a prior page")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runList(ctx context.Context, g *clictx.Globals, lf ListFlags) error {
	if err := miro.ValidateID("board_id", lf.BoardID); err != nil {
		return err
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

// BuildListPath assembles the request URL for a connectors list.
// Exported so cross-package callers can keep their HTTP behaviour
// identical to the list verb.
func BuildListPath(lf ListFlags) string {
	q := url.Values{}
	if lf.Limit > 0 {
		q.Set("limit", strconv.Itoa(lf.Limit))
	}
	if lf.Cursor != "" {
		q.Set("cursor", lf.Cursor)
	}
	path := "/v2/boards/" + lf.BoardID + "/connectors"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

// Fetch is the cross-package primitive: GET the connectors list, return
// the decoded response. Mirrors items.Fetch for consistency.
func Fetch(ctx context.Context, client *miro.Client, lf ListFlags) (ListResponse, error) {
	var resp ListResponse
	if err := client.Get(ctx, BuildListPath(lf), &resp); err != nil {
		return ListResponse{}, err
	}
	return resp, nil
}

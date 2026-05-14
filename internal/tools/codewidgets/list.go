package codewidgets

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ListFlags carries the query parameters for GET
// /v2-experimental/boards/{board_id}/code_widgets. BoardID is required
// (path param). Limit/Cursor drive cursor pagination; Miro caps Limit
// to 50 and defaults to 10.
type ListFlags struct {
	BoardID string
	Limit   int
	Cursor  string
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var lf ListFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List code widget items on a board",
		Long: "Calls GET /v2-experimental/boards/{board_id}/code_widgets.\n\n" +
			"The response is cursor-paginated; pass --cursor on a follow-up\n" +
			"call to fetch the next page. Miro caps --limit at 50.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, lf)
		},
	}
	cmd.Flags().StringVar(&lf.BoardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&lf.Limit, "limit", 0, "Page size (10-50; 0 = API default)")
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

// BuildListPath assembles the request URL with query parameters in a
// stable, sorted order (url.Values.Encode does the sorting).
func BuildListPath(lf ListFlags) string {
	q := url.Values{}
	if lf.Limit > 0 {
		q.Set("limit", strconv.Itoa(lf.Limit))
	}
	if lf.Cursor != "" {
		q.Set("cursor", lf.Cursor)
	}
	path := "/v2-experimental/boards/" + lf.BoardID + "/code_widgets"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

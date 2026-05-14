package tables

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// listFlags drives `miro tables list`. The wire endpoint paginates with
// cursor + limit, same shape as groups list.
type listFlags struct {
	boardID string
	limit   int
	cursor  string
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List data-table items on a board",
		Long: "Calls GET /v2/boards/{board_id}/data_table_formats and prints\n" +
			"the cursor-paginated response. Each entry includes the table\n" +
			"id, position, geometry, and audit timestamps.\n\n" +
			"Pass --limit to control page size (default 10, max 50) and\n" +
			"--cursor to fetch the next page from a prior response.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default of 10, max 50)")
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
	var resp map[string]any
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildListPath assembles the cursor-paginated URL. Split out for tests.
func buildListPath(f listFlags) string {
	q := url.Values{}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.cursor != "" {
		q.Set("cursor", f.cursor)
	}
	path := "/v2/boards/" + f.boardID + "/data_table_formats"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

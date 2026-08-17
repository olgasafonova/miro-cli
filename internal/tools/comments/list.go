package comments

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// ListFlags carries the query parameters for GET
// /v2-experimental/boards/{board_id}/comments. Unlike most item
// endpoints, comments use offset pagination: pass --offset on a
// follow-up call to fetch the next page. Miro defaults --limit to 20
// and caps it at 50.
type ListFlags struct {
	BoardID string
	Limit   int
	Offset  int
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var lf ListFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List comment threads on a board",
		Long: "Calls GET /v2-experimental/boards/{board_id}/comments.\n\n" +
			"The response is offset-paginated ({data, total, offset, size});\n" +
			"pass --offset on a follow-up call to fetch the next page. Miro\n" +
			"defaults --limit to 20 and caps it at 50. Each thread carries a\n" +
			"messages[] array with the full conversation.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, lf)
		},
	}
	cmd.Flags().StringVar(&lf.BoardID, "board-id", "", "Board ID (required)")
	cmd.Flags().IntVar(&lf.Limit, "limit", 0, "Page size (1-50; 0 = API default 20)")
	cmd.Flags().IntVar(&lf.Offset, "offset", 0, "Number of threads to skip")
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
		return wrapExperimentalErr(err)
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
	if lf.Offset > 0 {
		q.Set("offset", strconv.Itoa(lf.Offset))
	}
	path := basePath(lf.BoardID)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

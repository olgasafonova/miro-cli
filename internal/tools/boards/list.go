package boards

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `boards` parent command. Phase 2 shipped `list`;
// Phase 3a layers in the CRUD verbs. The rest of the boards surface
// (find/search/share/content/summary/picture/audit/diagram) lands in
// follow-up commits — see bead miro-cli-8fq.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "boards",
		Short: "Manage Miro boards",
	}
	cmd.AddCommand(
		newListCmd(g),
		newGetCmd(g),
		newCreateCmd(g),
		newCopyCmd(g),
		newUpdateCmd(g),
		newDeleteCmd(g),
		newShareCmd(g),
		newFindCmd(g),
		newPictureCmd(g),
		newSearchCmd(g),
		newSummaryCmd(g),
		newContentCmd(g),
		newAuditCmd(g),
		newDiagramCmd(g),
	)
	return cmd
}

// listFlags captures the per-invocation filters and pagination knobs for
// `miro boards list`. Kept as a named struct so the cobra wiring stays
// flat and the RunE can hand the same struct to a testable helper.
type listFlags struct {
	teamID    string
	projectID string
	query     string
	owner     string
	sort      string
	limit     int
	offset    int
}

func newListCmd(g *clictx.Globals) *cobra.Command {
	var lf listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List boards accessible to the access token",
		Long: "Calls GET /v2/boards and prints the response.\n\n" +
			"Pagination uses offset/limit. Pass --limit and --offset to step\n" +
			"through results; --select narrows the JSON output to the fields\n" +
			"you care about.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), g, lf)
		},
	}
	cmd.Flags().StringVar(&lf.teamID, "team-id", "", "Filter by team ID")
	cmd.Flags().StringVar(&lf.projectID, "project-id", "", "Filter by project ID")
	cmd.Flags().StringVar(&lf.query, "query", "", "Substring match on board name")
	cmd.Flags().StringVar(&lf.owner, "owner", "", "Filter by owner user ID")
	cmd.Flags().StringVar(&lf.sort, "sort", "", "Sort order (default|last_modified|last_opened|last_created|alphabetically)")
	cmd.Flags().IntVar(&lf.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().IntVar(&lf.offset, "offset", 0, "Page offset")
	return cmd
}

// runList builds the request URL, honors --dry-run, and otherwise issues
// the GET and emits the response through Globals.
//
// Exported indirectly through newListCmd; kept as a package-private
// function whose signature is friendly to table-driven tests that call
// it directly against an httptest-backed *miro.Client.
func runList(ctx context.Context, g *clictx.Globals, lf listFlags) error {
	path := buildListPath(lf)
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

func buildListPath(lf listFlags) string {
	path := "/v2/boards"
	if encoded := listQueryValues(lf).Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

// listQueryValues maps the set flags to their /v2/boards query
// parameters. Unset filters (empty strings, non-positive pagination
// values) are omitted so the API applies its own defaults.
func listQueryValues(lf listFlags) url.Values {
	q := url.Values{}
	filters := map[string]string{
		"team_id":    lf.teamID,
		"project_id": lf.projectID,
		"query":      lf.query,
		"owner":      lf.owner,
		"sort":       lf.sort,
	}
	for key, value := range filters {
		if value != "" {
			q.Set(key, value)
		}
	}
	if lf.limit > 0 {
		q.Set("limit", strconv.Itoa(lf.limit))
	}
	if lf.offset > 0 {
		q.Set("offset", strconv.Itoa(lf.offset))
	}
	return q
}

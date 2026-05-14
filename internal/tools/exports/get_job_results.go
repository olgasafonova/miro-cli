package exports

import (
	"context"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

type getJobResultsFlags struct {
	orgID  string
	jobID  string
	limit  int
	offset int
}

func newGetJobResultsCmd(g *clictx.Globals) *cobra.Command {
	var f getJobResultsFlags
	cmd := &cobra.Command{
		Use:   "get-job-results",
		Short: "Get results for a finished export job",
		Long: "Calls GET /v2/orgs/{org_id}/boards/export/jobs/{job_id}/results.\n\n" +
			"Returns the per-board export records (including S3 links) for a\n" +
			"FINISHED job. --limit and --offset paginate when the job covers\n" +
			"many boards.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetJobResults(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.orgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&f.jobID, "job-id", "", "Export job ID (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "Offset into the results list")
	_ = cmd.MarkFlagRequired("org-id")
	_ = cmd.MarkFlagRequired("job-id")
	return cmd
}

func runGetJobResults(ctx context.Context, g *clictx.Globals, f getJobResultsFlags) error {
	if err := miro.ValidateID("org_id", f.orgID); err != nil {
		return err
	}
	if err := miro.ValidateID("job_id", f.jobID); err != nil {
		return err
	}
	path := buildResultsPath(f)
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

// buildResultsPath assembles the GET path with optional limit/offset
// query parameters. Split out for table-driven tests.
func buildResultsPath(f getJobResultsFlags) string {
	q := url.Values{}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.offset > 0 {
		q.Set("offset", strconv.Itoa(f.offset))
	}
	path := "/v2/orgs/" + f.orgID + "/boards/export/jobs/" + f.jobID + "/results"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

package exports

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

type listJobTasksFlags struct {
	orgID  string
	jobID  string
	limit  int
	offset int
}

func newListJobTasksCmd(g *clictx.Globals) *cobra.Command {
	var f listJobTasksFlags
	cmd := &cobra.Command{
		Use:   "list-job-tasks",
		Short: "List per-board tasks for an export job",
		Long: "Calls GET /v2/orgs/{org_id}/boards/export/jobs/{job_id}/tasks.\n\n" +
			"Each task corresponds to one board in the job. --limit and\n" +
			"--offset paginate when the job covers many boards.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListJobTasks(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.orgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&f.jobID, "job-id", "", "Export job ID (required)")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "Offset into the tasks list")
	_ = cmd.MarkFlagRequired("org-id")
	_ = cmd.MarkFlagRequired("job-id")
	return cmd
}

func runListJobTasks(ctx context.Context, g *clictx.Globals, f listJobTasksFlags) error {
	if f.orgID == "" {
		return errors.New("--org-id is required")
	}
	if f.jobID == "" {
		return errors.New("--job-id is required")
	}
	path := buildTasksPath(f)
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

// buildTasksPath assembles the GET path with optional limit/offset
// query parameters. Split out for table-driven tests.
func buildTasksPath(f listJobTasksFlags) string {
	q := url.Values{}
	if f.limit > 0 {
		q.Set("limit", strconv.Itoa(f.limit))
	}
	if f.offset > 0 {
		q.Set("offset", strconv.Itoa(f.offset))
	}
	path := "/v2/orgs/" + f.orgID + "/boards/export/jobs/" + f.jobID + "/tasks"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

package exports

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

func newGetJobStatusCmd(g *clictx.Globals) *cobra.Command {
	var (
		orgID string
		jobID string
	)
	cmd := &cobra.Command{
		Use:   "get-job-status",
		Short: "Get the status of an export job",
		Long: "Calls GET /v2/orgs/{org_id}/boards/export/jobs/{job_id}.\n\n" +
			"The response carries the current ExportJobStatus: CREATED,\n" +
			"IN_PROGRESS, CANCELLED, or FINISHED. Poll until FINISHED before\n" +
			"calling `get-job-results`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetJobStatus(cmd.Context(), g, orgID, jobID)
		},
	}
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&jobID, "job-id", "", "Export job ID (required)")
	_ = cmd.MarkFlagRequired("org-id")
	_ = cmd.MarkFlagRequired("job-id")
	return cmd
}

func runGetJobStatus(ctx context.Context, g *clictx.Globals, orgID, jobID string) error {
	if orgID == "" {
		return errors.New("--org-id is required")
	}
	if jobID == "" {
		return errors.New("--job-id is required")
	}
	path := "/v2/orgs/" + orgID + "/boards/export/jobs/" + jobID
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

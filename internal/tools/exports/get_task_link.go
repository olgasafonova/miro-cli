package exports

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newGetTaskLinkCmd(g *clictx.Globals) *cobra.Command {
	var (
		orgID  string
		jobID  string
		taskID string
	)
	cmd := &cobra.Command{
		Use:   "get-task-link",
		Short: "Create a download link for an export task",
		Long: "Calls POST /v2/orgs/{org_id}/boards/export/jobs/{job_id}/tasks/{task_id}/export-link.\n\n" +
			"The response carries a time-limited URL pointing at the\n" +
			"task's export artifact. Each task in a job has its own link.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetTaskLink(cmd.Context(), g, orgID, jobID, taskID)
		},
	}
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&jobID, "job-id", "", "Export job ID (required)")
	cmd.Flags().StringVar(&taskID, "task-id", "", "Export task ID (required)")
	_ = cmd.MarkFlagRequired("org-id")
	_ = cmd.MarkFlagRequired("job-id")
	_ = cmd.MarkFlagRequired("task-id")
	return cmd
}

func runGetTaskLink(ctx context.Context, g *clictx.Globals, orgID, jobID, taskID string) error {
	if err := miro.ValidateID("org_id", orgID); err != nil {
		return err
	}
	if err := miro.ValidateID("job_id", jobID); err != nil {
		return err
	}
	if err := miro.ValidateID("task_id", taskID); err != nil {
		return err
	}
	path := "/v2/orgs/" + orgID + "/boards/export/jobs/" + jobID + "/tasks/" + taskID + "/export-link"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Post(ctx, path, nil, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

package exports

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// updateJobFlags exists only so the runUpdateJob signature stays
// table-test-friendly. Today the API only supports one mutation
// (cancellation); when more land, additional bool flags slot in here.
type updateJobFlags struct {
	orgID  string
	jobID  string
	cancel bool
}

func newUpdateJobCmd(g *clictx.Globals) *cobra.Command {
	var f updateJobFlags
	cmd := &cobra.Command{
		Use:   "update-job",
		Short: "Update an export job (currently: cancel only)",
		Long: "Calls PUT /v2/orgs/{org_id}/boards/export/jobs/{job_id}/status\n" +
			"with {\"status\":\"CANCELLED\"}.\n\n" +
			"Moderately destructive: cancellation is reversible only on jobs\n" +
			"that haven't started producing artifacts. Refuses without --yes\n" +
			"(or --agent, which implies --yes). Use --dry-run to preview.\n\n" +
			"Only --cancel is currently accepted — the API supports no other\n" +
			"update operation today.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateJob(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.orgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&f.jobID, "job-id", "", "Export job ID (required)")
	cmd.Flags().BoolVar(&f.cancel, "cancel", false, "Cancel the job (the only supported mutation)")
	_ = cmd.MarkFlagRequired("org-id")
	_ = cmd.MarkFlagRequired("job-id")
	return cmd
}

func runUpdateJob(ctx context.Context, g *clictx.Globals, f updateJobFlags) error {
	if err := miro.ValidateID("org_id", f.orgID); err != nil {
		return err
	}
	if err := miro.ValidateID("job_id", f.jobID); err != nil {
		return err
	}
	if !f.cancel {
		return errors.New("--cancel is required; no other update operation is supported")
	}
	path := "/v2/orgs/" + f.orgID + "/boards/export/jobs/" + f.jobID + "/status"
	if g.DryRun {
		return g.EmitDryRun("PUT", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to cancel export job without --yes; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	req := updateRequest{Status: "CANCELLED"}
	var resp map[string]any
	if err := client.Put(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

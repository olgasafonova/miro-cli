package exports

import (
	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// NewCmd returns the `exports` parent command. Phase 3c ships
// create-job/get-job-status/get-job-results/list-job-tasks/get-task-link/
// update-job against /v2/orgs/{org_id}/boards/export/*. All endpoints
// are enterprise-only and require eDiscovery to be enabled.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exports",
		Short: "Orchestrate enterprise board export jobs (org-scoped, eDiscovery)",
	}
	cmd.AddCommand(
		newCreateJobCmd(g),
		newGetJobStatusCmd(g),
		newGetJobResultsCmd(g),
		newListJobTasksCmd(g),
		newGetTaskLinkCmd(g),
		newUpdateJobCmd(g),
	)
	return cmd
}

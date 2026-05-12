package exports

import (
	"context"
	"errors"
	"net/url"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// createJobFlags captures the per-invocation knobs for `miro exports
// create-job`. --request-id is caller-generated (typically a UUID) and
// gives the API an idempotency key — retries with the same key won't
// spawn duplicate jobs.
type createJobFlags struct {
	orgID     string
	requestID string
	boardIDs  []string
	format    string
}

func newCreateJobCmd(g *clictx.Globals) *cobra.Command {
	var f createJobFlags
	cmd := &cobra.Command{
		Use:   "create-job",
		Short: "Create an enterprise board export job",
		Long: "Calls POST /v2/orgs/{org_id}/boards/export/jobs?request_id=X.\n\n" +
			"Required: --org-id, --request-id (caller-generated UUID),\n" +
			"at least one --board-id (repeatable, max 1000), and --format\n" +
			"(SVG, HTML, or PDF). The response contains the new job's id;\n" +
			"poll with `get-job-status` until status is FINISHED.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateJob(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.orgID, "org-id", "", "Organization ID (required)")
	cmd.Flags().StringVar(&f.requestID, "request-id", "", "Caller-generated UUID for idempotency (required)")
	cmd.Flags().StringArrayVar(&f.boardIDs, "board-id", nil, "Board ID to export (repeat for multiple, at least one required)")
	cmd.Flags().StringVar(&f.format, "format", "", "Export format: SVG, HTML, or PDF (required)")
	_ = cmd.MarkFlagRequired("org-id")
	_ = cmd.MarkFlagRequired("request-id")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("format")
	return cmd
}

func runCreateJob(ctx context.Context, g *clictx.Globals, f createJobFlags) error {
	if f.orgID == "" {
		return errors.New("--org-id is required")
	}
	if f.requestID == "" {
		return errors.New("--request-id is required")
	}
	if len(f.boardIDs) == 0 {
		return errors.New("at least one --board-id is required")
	}
	if f.format == "" {
		return errors.New("--format is required")
	}
	if err := validateFormat(f.format); err != nil {
		return err
	}
	req := createRequest{BoardIDs: f.boardIDs, BoardFormat: f.format}
	path := "/v2/orgs/" + f.orgID + "/boards/export/jobs?request_id=" + url.QueryEscape(f.requestID)
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Post(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

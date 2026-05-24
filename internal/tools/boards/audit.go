package boards

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// auditFlags captures the query parameters Miro's /v2/audit/logs
// endpoint accepts. createdAfter/createdBefore are RFC3339 timestamps
// per the Miro API contract; the CLI validates the format up-front so
// users see a "bad timestamp" error before they pay an API round-trip.
type auditFlags struct {
	createdAfter  string
	createdBefore string
	limit         int
	cursor        string
}

func newAuditCmd(g *clictx.Globals) *cobra.Command {
	var af auditFlags
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Query Miro's enterprise audit log",
		Long: "Calls GET /v2/audit/logs. Enterprise-only endpoint — requires\n" +
			"the auditlogs:read scope and an enterprise plan. Returns the\n" +
			"last 90 days of events; for older data Miro provides the CSV\n" +
			"export feature in the web app.\n\n" +
			"--created-after and --created-before take RFC3339 timestamps\n" +
			"(e.g. 2026-05-01T00:00:00Z). The endpoint is cursor-paginated.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudit(cmd.Context(), g, af)
		},
	}
	cmd.Flags().StringVar(&af.createdAfter, "created-after", "", "RFC3339 lower bound (inclusive)")
	cmd.Flags().StringVar(&af.createdBefore, "created-before", "", "RFC3339 upper bound (inclusive)")
	cmd.Flags().IntVar(&af.limit, "limit", 0, "Page size (0 = API default)")
	cmd.Flags().StringVar(&af.cursor, "cursor", "", "Cursor from a prior page")
	return cmd
}

func runAudit(ctx context.Context, g *clictx.Globals, af auditFlags) error {
	if err := validateAuditTimestamps(af); err != nil {
		return err
	}
	path := buildAuditPath(af)
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

// validateAuditTimestamps catches malformed RFC3339 input before we
// pay an API round-trip. Empty values are fine — the endpoint applies
// its own defaults.
func validateAuditTimestamps(af auditFlags) error {
	if af.createdAfter != "" {
		if _, err := time.Parse(time.RFC3339, af.createdAfter); err != nil {
			return errors.New("--created-after must be RFC3339 (e.g. 2026-05-01T00:00:00Z): " + err.Error())
		}
	}
	if af.createdBefore != "" {
		if _, err := time.Parse(time.RFC3339, af.createdBefore); err != nil {
			return errors.New("--created-before must be RFC3339 (e.g. 2026-05-01T00:00:00Z): " + err.Error())
		}
	}
	return nil
}

func buildAuditPath(af auditFlags) string {
	q := url.Values{}
	if af.createdAfter != "" {
		q.Set("createdAfter", af.createdAfter)
	}
	if af.createdBefore != "" {
		q.Set("createdBefore", af.createdBefore)
	}
	if af.limit > 0 {
		q.Set("limit", strconv.Itoa(af.limit))
	}
	if af.cursor != "" {
		q.Set("cursor", af.cursor)
	}
	path := "/v2/audit/logs"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

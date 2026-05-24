package audit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// ListLogsFlags carries the query parameters for GET /v2/audit/logs.
// CreatedAfter / CreatedBefore are required RFC3339 timestamps; the
// API rejects anything older than 90 days. Cursor / Limit drive
// pagination. Sorting and UserID are optional filters.
type ListLogsFlags struct {
	CreatedAfter  string
	CreatedBefore string
	Cursor        string
	Limit         int
	Sorting       string
	UserID        string
}

func newListLogsCmd(g *clictx.Globals) *cobra.Command {
	var lf ListLogsFlags
	cmd := &cobra.Command{
		Use:   "list-logs",
		Short: "List Enterprise audit log events",
		Long: "Calls GET /v2/audit/logs.\n\n" +
			"--created-after and --created-before are required RFC3339\n" +
			"timestamps (UTC, ISO 8601). The Enterprise audit API only\n" +
			"returns events from the last 90 days; use the CSV export for\n" +
			"older data. The response is cursor-paginated; pass --cursor on\n" +
			"a follow-up call to fetch the next page.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListLogs(cmd.Context(), g, lf)
		},
	}
	cmd.Flags().StringVar(&lf.CreatedAfter, "created-after", "", "Lower bound for createdAt (RFC3339, required)")
	cmd.Flags().StringVar(&lf.CreatedBefore, "created-before", "", "Upper bound for createdAt (RFC3339, required)")
	cmd.Flags().StringVar(&lf.Cursor, "cursor", "", "Cursor from a prior page")
	cmd.Flags().IntVar(&lf.Limit, "limit", 100, "Page size (max per Miro: 100)")
	cmd.Flags().StringVar(&lf.Sorting, "sorting", "", "Sort order: ASC or DESC")
	cmd.Flags().StringVar(&lf.UserID, "user-id", "", "Scope results to one user (optional)")
	return cmd
}

func runListLogs(ctx context.Context, g *clictx.Globals, lf ListLogsFlags) error {
	if lf.CreatedAfter == "" {
		return errors.New("--created-after is required")
	}
	if lf.CreatedBefore == "" {
		return errors.New("--created-before is required")
	}
	if _, err := time.Parse(time.RFC3339, lf.CreatedAfter); err != nil {
		return fmt.Errorf("--created-after must be RFC3339 (e.g. 2026-04-01T00:00:00Z): %w", err)
	}
	if _, err := time.Parse(time.RFC3339, lf.CreatedBefore); err != nil {
		return fmt.Errorf("--created-before must be RFC3339 (e.g. 2026-05-12T23:59:59Z): %w", err)
	}
	path := BuildListLogsPath(lf)
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp ListLogsResponse
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// BuildListLogsPath assembles the request URL with query parameters
// in a stable, sorted order (url.Values.Encode does the sorting).
func BuildListLogsPath(lf ListLogsFlags) string {
	q := url.Values{}
	if lf.CreatedAfter != "" {
		q.Set("createdAfter", lf.CreatedAfter)
	}
	if lf.CreatedBefore != "" {
		q.Set("createdBefore", lf.CreatedBefore)
	}
	if lf.Cursor != "" {
		q.Set("cursor", lf.Cursor)
	}
	if lf.Limit > 0 {
		q.Set("limit", strconv.Itoa(lf.Limit))
	}
	if lf.Sorting != "" {
		q.Set("sorting", lf.Sorting)
	}
	if lf.UserID != "" {
		q.Set("userId", lf.UserID)
	}
	path := "/v2/audit/logs"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path
}

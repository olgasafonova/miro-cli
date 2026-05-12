// Package exports holds the hand-authored Cobra subcommands for the
// /v2/orgs/{org_id}/boards/export/* family of Miro REST endpoints.
//
// These are enterprise-only org-scoped board export jobs that orchestrate
// an async export workflow: create job, poll status, fetch results,
// list tasks, and download via per-task export-links. Cancellation is
// the only mutation currently supported on a started job.
//
// The REST contract is documented in spec.json under operationIds with
// the `enterprise-*board-export-*` prefix. HTTP verbs follow the spec
// (PUT for status update, POST for task export-link), not the more
// uniform CRUD verbs the rest of the CLI uses.
package exports

import "fmt"

// createRequest is the POST /v2/orgs/{org_id}/boards/export/jobs body.
// boardFormat must be one of SVG / HTML / PDF per CreateBoardExportRequest
// in spec.json; we validate the enum client-side so the user gets a
// clear error before the API rejects it.
type createRequest struct {
	BoardIDs    []string `json:"boardIds"`
	BoardFormat string   `json:"boardFormat"`
}

// updateRequest is the PUT /v2/orgs/{org_id}/boards/export/jobs/{job_id}/status
// body. The spec says only "CANCELLED" is currently accepted; we expose
// only the --cancel flag and emit that literal so callers can't drift.
type updateRequest struct {
	Status string `json:"status"`
}

// validateFormat enforces the BoardFormat enum from spec.json. Empty
// strings fail because --format is required (the API would default to
// SVG, but the agent UX is better when intent is explicit).
func validateFormat(f string) error {
	switch f {
	case "SVG", "HTML", "PDF":
		return nil
	default:
		return fmt.Errorf("invalid --format %q: must be SVG, HTML, or PDF", f)
	}
}

// Package members holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/members/* family of Miro REST endpoints. Phase
// 3c ships list/get/update/remove on the same pattern as
// internal/tools/embeds/ — one file per verb, table-driven tests
// against httptest, JSON output through clictx.Globals. Members are
// offset-paginated (not cursor-paginated) and carry a single mutable
// field on update: role.
package members

import "fmt"

// updateRequest is the PATCH body for /v2/boards/{board_id}/members/{board_member_id}.
// Mirrors Miro's BoardMemberChanges schema. Role is the only mutable
// field today; if Miro adds more, extend this struct with omitempty
// fields and the *Set bool pattern from embeds/update.go.
type updateRequest struct {
	Role string `json:"role,omitempty"`
}

// removeResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `removed` rather than inspecting exit codes. Named
// `removed` (not `deleted`) because the API operation removes a user's
// access to the board, not the user themselves.
type removeResult struct {
	Removed bool   `json:"removed"`
	ID      string `json:"id"`
}

// validRoles is the closed enum from the Miro spec plus "guest", which
// the task brief flagged as a real role in the wild even though the
// public OpenAPI schema does not list it. Keeping it client-side means
// callers see a clear error rather than letting the API 400 on values
// like "admin" or "viewer-only" that look plausible but aren't accepted.
var validRoles = map[string]struct{}{
	"viewer":    {},
	"commenter": {},
	"editor":    {},
	"coowner":   {},
	"owner":     {},
	"guest":     {},
}

// validateRole rejects any --role value that isn't one of the strings
// Miro's API accepts. Empty passes (caller didn't set it, runUpdate
// gates that separately).
func validateRole(r string) error {
	if r == "" {
		return nil
	}
	if _, ok := validRoles[r]; !ok {
		return fmt.Errorf("invalid --role %q: must be one of viewer, commenter, editor, coowner, owner, guest", r)
	}
	return nil
}

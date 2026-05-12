// Package tags holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/tags/* family of Miro REST endpoints. Phase 3c
// ships list/create/get/update/delete on the same pattern as
// internal/tools/embeds/ — one file per verb, table-driven tests
// against httptest, JSON output through clictx.Globals. Tags are a
// flat resource: a title + a fillColor enum; no position, geometry, or
// parent envelopes.
package tags

import "fmt"

// createRequest is the POST /v2/boards/{board_id}/tags body. Both
// fields are required by the spec; we still emit only the ones the
// caller set so empty strings don't override server-side defaults on
// optional callers (e.g. fillColor defaults to "red" when omitted).
type createRequest struct {
	Title     string `json:"title,omitempty"`
	FillColor string `json:"fillColor,omitempty"`
}

// updateRequest is the PATCH body. Both fields are pointers so we can
// distinguish "field omitted" from "field set to empty string"; only
// the fields the user touched are emitted on the wire.
type updateRequest struct {
	Title     *string `json:"title,omitempty"`
	FillColor *string `json:"fillColor,omitempty"`
}

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// validateFillColor rejects any --fill-color value that isn't one of
// the twelve strings Miro's API accepts. Empty passes (caller didn't
// set it; server applies its own default of "red").
func validateFillColor(c string) error {
	switch c {
	case "",
		"red",
		"magenta",
		"violet",
		"light_green",
		"green",
		"dark_green",
		"cyan",
		"blue",
		"dark_blue",
		"black",
		"gray",
		"yellow":
		return nil
	default:
		return fmt.Errorf("invalid --fill-color %q: must be one of red, magenta, violet, light_green, green, dark_green, cyan, blue, dark_blue, black, gray, yellow", c)
	}
}

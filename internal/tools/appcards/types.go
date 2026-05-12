// Package appcards holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/app_cards/* family of Miro REST endpoints.
// Phase 3b ships create/get/update/delete on the same pattern as
// internal/tools/stickies/ — one file per verb, table-driven tests
// against httptest, JSON output through clictx.Globals.
//
// The first cut intentionally omits the `data.fields` array (list of
// custom field objects rendered on the card). Adding a typed surface for
// nested fields is a Phase 4 follow-up — callers needing it today can
// fall back to the generic items command.
package appcards

import "fmt"

// createRequest is the POST /v2/boards/{board_id}/app_cards body.
// Built per-call from CLI flags. Miro wraps each section in a typed
// envelope; we mirror that shape so the JSON encoder emits the exact
// payload the API expects.
type createRequest struct {
	Data     dataField     `json:"data"`
	Style    *styleField   `json:"style,omitempty"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

// updateRequest is the PATCH body. All sections are pointers because the
// API treats absent sections as "leave alone"; emitting an empty object
// rewrites the section to defaults.
type updateRequest struct {
	Data     *dataField    `json:"data,omitempty"`
	Style    *styleField   `json:"style,omitempty"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

type dataField struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	Owned       *bool  `json:"owned,omitempty"`
}

type styleField struct {
	FillColor string `json:"fillColor,omitempty"`
}

type positionData struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin,omitempty"`
}

type geometryData struct {
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// parentRef mirrors Miro's parent envelope. Setting ID="" with the
// envelope present is how the API expresses "detach from frame";
// callers signal that by setting parent to a non-nil *parentRef with an
// empty ID. Omitting the parent envelope entirely leaves the existing
// parent untouched.
type parentRef struct {
	ID string `json:"id"`
}

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// validateStatus checks that --status is empty (meaning "don't send") or
// one of the three values Miro accepts. Unknown values are rejected
// early so the API never sees a malformed request and callers get a
// clear local error.
func validateStatus(s string) error {
	switch s {
	case "", "disconnected", "connected", "disabled":
		return nil
	default:
		return fmt.Errorf("invalid --status %q: must be one of disconnected, connected, disabled", s)
	}
}

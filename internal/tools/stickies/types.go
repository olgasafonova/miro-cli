// Package stickies holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/sticky_notes/* family of Miro REST endpoints.
// Phase 3b ships create/get/update/delete on the same pattern as
// internal/tools/boards/ — one file per verb, table-driven tests against
// httptest, JSON output through clictx.Globals.
package stickies

import "strings"

// createRequest is the POST /v2/boards/{board_id}/sticky_notes body.
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
	Content string `json:"content,omitempty"`
	Shape   string `json:"shape,omitempty"`
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
	Width float64 `json:"width,omitempty"`
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

// normalizeStickyColor maps colloquial names ("yellow", "purple") to the
// exact strings Miro's API accepts. Unknown values pass through so
// callers can ship hex codes or future-added named colors without
// blocking on this layer.
func normalizeStickyColor(color string) string {
	switch strings.ToLower(color) {
	case "yellow":
		return "light_yellow"
	case "green":
		return "light_green"
	case "blue":
		return "light_blue"
	case "pink":
		return "light_pink"
	case "purple":
		return "violet"
	case "grey":
		return "gray"
	default:
		return color
	}
}

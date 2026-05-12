// Package documents holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/documents/* family of Miro REST endpoints. Phase
// 3b ships create/get/update/delete on the same pattern as
// internal/tools/embeds/ — one file per verb, table-driven tests
// against httptest, JSON output through clictx.Globals. Documents carry
// no style envelope; the data section holds url/title.
//
// Scope note: Miro's POST /v2/boards/{board_id}/documents accepts two
// content types — application/json (this port; URL referenced by
// data.url) and multipart/form-data (uploaded file bytes). This package
// covers ONLY the JSON / URL-based variant. A file-upload subcommand is
// deferred to Phase 4.
package documents

// createRequest is the POST /v2/boards/{board_id}/documents body. Built
// per-call from CLI flags. Miro wraps each section in a typed envelope;
// we mirror that shape so the JSON encoder emits the exact payload the
// API expects. Documents have no style envelope.
type createRequest struct {
	Data     dataField     `json:"data"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

// updateRequest is the PATCH body. All sections are pointers because
// the API treats absent sections as "leave alone"; emitting an empty
// object rewrites the section to defaults.
type updateRequest struct {
	Data     *dataField    `json:"data,omitempty"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

type dataField struct {
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
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

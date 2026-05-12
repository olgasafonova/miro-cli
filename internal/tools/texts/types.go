// Package texts holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/texts/* family of Miro REST endpoints.
// Phase 3b ships create/get/update/delete on the same pattern as
// internal/tools/stickies/.
package texts

import "strconv"

// createRequest is the POST /v2/boards/{board_id}/texts body.
type createRequest struct {
	Data     dataField     `json:"data"`
	Style    *styleField   `json:"style,omitempty"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

// updateRequest is the PATCH body. Sections are pointers because the
// API treats absent sections as "leave alone".
type updateRequest struct {
	Data     *dataField    `json:"data,omitempty"`
	Style    *styleField   `json:"style,omitempty"`
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

type dataField struct {
	Content string `json:"content,omitempty"`
}

// styleField is the text-specific style envelope. Miro's REST API
// expresses fontSize as a stringified integer (e.g. "14"); the field
// is typed string here so the JSON encoder produces the exact shape
// Miro accepts without a custom Marshaler.
type styleField struct {
	Color      string `json:"color,omitempty"`
	FillColor  string `json:"fillColor,omitempty"`
	FontSize   string `json:"fontSize,omitempty"`
	TextAlign  string `json:"textAlign,omitempty"`
	FontFamily string `json:"fontFamily,omitempty"`
}

type positionData struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin,omitempty"`
}

type geometryData struct {
	Width    float64 `json:"width,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
}

type parentRef struct {
	ID string `json:"id"`
}

type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// fontSizeString stringifies a non-zero font size for the API. Zero
// returns "" so the omitempty serializer drops the field.
func fontSizeString(size int) string {
	if size <= 0 {
		return ""
	}
	return strconv.Itoa(size)
}

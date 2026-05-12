// Package shapes holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/shapes/* family of Miro REST endpoints.
// Phase 3b ships create/get/update/delete on the same pattern as
// internal/tools/stickies/.
package shapes

// createRequest is the POST /v2/boards/{board_id}/shapes body.
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
	Shape   string `json:"shape,omitempty"`
}

// styleField mirrors the shape-style envelope Miro accepts. Color is
// the text color; FillColor is the background. Both accept hex or
// named colors; the API rejects unknown names so we don't normalize
// here.
type styleField struct {
	FillColor         string `json:"fillColor,omitempty"`
	Color             string `json:"color,omitempty"`
	TextAlign         string `json:"textAlign,omitempty"`
	TextAlignVertical string `json:"textAlignVertical,omitempty"`
	BorderColor       string `json:"borderColor,omitempty"`
	BorderStyle       string `json:"borderStyle,omitempty"`
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

type parentRef struct {
	ID string `json:"id"`
}

type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

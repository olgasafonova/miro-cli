package codewidgets

// ListResponse mirrors the cursor-paginated envelope returned by GET
// /v2-experimental/boards/{board_id}/code_widgets (spec schema:
// CodeWidgetCursorPaged). Items are kept as map[string]any because the
// v2-experimental schema is still subject to change.
type ListResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int              `json:"total,omitempty"`
	Size   int              `json:"size,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

// writeRequest is the POST / PATCH body shared by create and update.
// Every section is omitted when unset so a partial PATCH leaves the
// other fields alone.
type writeRequest struct {
	Data     map[string]any `json:"data,omitempty"`
	Position *positionData  `json:"position,omitempty"`
	Geometry *geometryData  `json:"geometry,omitempty"`
	Parent   *parentRef     `json:"parent,omitempty"`
}

// moveRequest is the PATCH .../code_widgets/{item_id}/position body.
type moveRequest struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin"`
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

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

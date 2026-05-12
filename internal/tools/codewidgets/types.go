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

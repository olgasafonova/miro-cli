package audit

// ListLogsResponse mirrors the cursor-paginated envelope returned by
// GET /v2/audit/logs (spec schema: AuditPage). Items are kept as
// map[string]any because audit events come in many event/category
// flavours and a typed schema here would either be wrong or huge.
type ListLogsResponse struct {
	Type   string           `json:"type,omitempty"`
	Data   []map[string]any `json:"data"`
	Size   int              `json:"size,omitempty"`
	Limit  int              `json:"limit,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
}

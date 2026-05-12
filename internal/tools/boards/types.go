// Package boards holds the hand-authored Cobra subcommands for the
// /v2/boards/* family of Miro REST endpoints. Phase 2 ships `list` as
// the reference shape; Phase 3a adds get/create/copy/update/delete/find
// /search/share/content/summary/picture/audit/diagram on the same
// pattern.
package boards

// Board is a minimal projection of Miro's board resource — only the
// fields we surface today. Additional fields are decoded into Raw so
// --select can pluck them without a schema change here.
type Board struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	ModifiedAt  string `json:"modifiedAt,omitempty"`
	ViewLink    string `json:"viewLink,omitempty"`
}

// ListResponse mirrors the offset-paginated envelope Miro returns from
// GET /v2/boards. We decode into a generic []map for the data slice so
// callers and --select can see every field the API returned, not just
// the ones in Board. Total/Size/Offset/Limit are typed because they
// drive pagination decisions in Phase 3.
type ListResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int              `json:"total,omitempty"`
	Size   int              `json:"size,omitempty"`
	Offset int              `json:"offset,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

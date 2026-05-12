// Package boards holds the hand-authored Cobra subcommands for the
// /v2/boards/* family of Miro REST endpoints. Phase 2 shipped `list`
// as the reference shape; Phase 3a layers in get/create/copy/update/
// delete/find/search/share/content/summary/picture/audit/diagram on
// the same pattern.
package boards

// Board is a minimal projection of Miro's board resource — only the
// fields we surface today. We decode the API response into the
// strongly-typed fields below and let json.Unmarshal silently drop
// fields we don't track. --select on the emitted JSON still lets
// callers pick from the full set the API returned, because we always
// re-encode through a map[string]any for list-shaped responses.
type Board struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	ModifiedAt  string `json:"modifiedAt,omitempty"`
	ViewLink    string `json:"viewLink,omitempty"`
}

// createRequest is the POST /v2/boards body. Built per-call from CLI
// flags. Pointer-free fields with omitempty so empty values don't get
// serialized; Miro rejects empty-string team IDs etc.
type createRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	TeamID      string `json:"teamId,omitempty"`
}

// updateRequest is the PATCH /v2/boards/{board_id} body. Same omitempty
// discipline — Miro treats an absent field as "leave it alone" and
// rejects empty strings.
type updateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// deleteResult is the JSON envelope emitted to stdout after a
// successful DELETE. The API returns an empty 204, so we synthesize a
// deterministic success signal that agents can branch on without
// inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
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

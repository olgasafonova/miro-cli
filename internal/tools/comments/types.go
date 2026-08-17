package comments

// createRequest is the POST /v2-experimental/boards/{board_id}/comments
// body. ItemID is omitted for board-level threads; the API anchors the
// thread to the item when it is present (position.type = "attached").
type createRequest struct {
	Content string `json:"content"`
	ItemID  string `json:"itemId,omitempty"`
}

// replyRequest is the POST .../comments/{comment_id}/messages body.
type replyRequest struct {
	Content string `json:"content"`
}

// resolveRequest is the PATCH .../comments/{comment_id} body. Resolved
// has no omitempty on purpose: false ("reopen") must reach the wire.
type resolveRequest struct {
	Resolved bool `json:"resolved"`
}

// ListResponse mirrors the offset-paginated envelope returned by GET
// /v2-experimental/boards/{board_id}/comments. Threads are kept as
// map[string]any because the v2-experimental schema is undocumented and
// subject to change; the CLI emits what the API returned, verbatim.
type ListResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int              `json:"total,omitempty"`
	Offset int              `json:"offset,omitempty"`
	Size   int              `json:"size,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

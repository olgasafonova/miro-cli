// Package mindmap holds the hand-authored Cobra subcommands for the
// /v2-experimental/boards/{board_id}/mindmap_nodes/* family of Miro
// endpoints. Phase 3c (Phase 3c, bead miro-cli mindmap port) ships
// list/create/get/delete on a 4-verb pattern modelled on
// internal/tools/embeds/ — one file per verb, table-driven tests
// against httptest, JSON output through clictx.Globals.
//
// Mindmap differs from the v2 typed-items family in three ways. First,
// the endpoint prefix is /v2-experimental/ rather than /v2/. Second,
// there is intentionally no update verb in the spec — Miro is still
// shipping that surface. Third, the data envelope nests one level
// deeper: the wire shape is data.nodeView.data.{type,content}, where
// nodeView.data.type is the literal string "text" today (the spec
// reserves room for other node types later but accepts only "text").
//
// Mindmap nodes are tree-structured. A node with no parent is the
// mind-map root; a node with parent.id set is a child of that node.
// Deleting a parent deletes its subtree, which is why the delete verb
// surfaces a heavier confirmation message than the v2 typed-items
// destructive verbs.
package mindmap

// createRequest is the POST /v2-experimental/boards/{board_id}/mindmap_nodes
// body. The shape mirrors MindmapCreateRequest from spec.json. Parent
// is a pointer so callers can omit the envelope entirely for a root
// node — an empty *parentRef would serialize as `"parent": {"id": ""}`,
// which the API would reject.
type createRequest struct {
	Data     mindmapData   `json:"data"`
	Position *positionData `json:"position,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

// mindmapData is the data section. nodeView wraps the actual content;
// the extra level of nesting matches Miro's MindmapDataForCreate schema.
type mindmapData struct {
	NodeView nodeView `json:"nodeView"`
}

// nodeView holds the text data for a mind-map node. The inner data
// field's type is always "text" today — the spec lists type as a
// required string and reserves room for other widget types later, but
// only "text" is accepted by the API today.
type nodeView struct {
	Data nodeTextData `json:"data"`
}

type nodeTextData struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

type positionData struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin,omitempty"`
}

// parentRef carries the parent node ID for a child mind-map node. For
// a root node, the entire parent envelope is omitted (createRequest.Parent
// stays nil). Setting ID="" with the envelope present is not a defined
// operation for mindmap — there is no reparent-detach today; that's
// items.update territory.
type parentRef struct {
	ID string `json:"id"`
}

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// listResponse mirrors the cursor-paginated envelope Miro returns from
// GET /v2-experimental/boards/{board_id}/mindmap_nodes. Data is kept
// as []map[string]any because the v2-experimental schema may still
// evolve and we don't want a typed mirror to drift from what the API
// actually returns.
type listResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int              `json:"total,omitempty"`
	Size   int              `json:"size,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

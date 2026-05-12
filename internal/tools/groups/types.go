// Package groups holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/groups/* family of Miro REST endpoints. Phase
// 3c ships list/create/get/get-items/update/delete on the same pattern
// as internal/tools/embeds/ — one file per verb, table-driven tests
// against httptest, JSON output through clictx.Globals. Groups carry
// no style envelope; the data section holds a list of item IDs.
package groups

// createRequest is the POST /v2/boards/{board_id}/groups body. The PUT
// (update) endpoint shares the same shape — it's a full replace, not a
// partial PATCH, so updateRequest is an alias rather than a separate
// pointer-laden type.
type createRequest struct {
	Data createData `json:"data"`
}

// updateRequest is the PUT body — identical to createRequest. Aliased
// so call sites read clearly ("buildUpdateRequest returns updateRequest")
// without inventing a second shape.
type updateRequest = createRequest

// createData wraps the items array. Miro wraps the items list inside a
// `data` envelope so the JSON encoder emits the exact payload the API
// expects.
type createData struct {
	Items []string `json:"items"`
}

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes. The
// wording reflects the API's actual behaviour: items survive; only the
// group association is removed (unless --delete-items is passed, in
// which case the items also go).
type deleteResult struct {
	Deleted     bool   `json:"deleted"`
	ID          string `json:"id"`
	DeleteItems bool   `json:"deleteItems,omitempty"`
}

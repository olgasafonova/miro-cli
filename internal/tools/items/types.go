// Package items holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/items family. Phase 3c (bead miro-cli-8jr)
// owns the full surface; Phase 3a's boards composites (search,
// summary, content) need the `list` primitive earlier than the rest
// of Phase 3c, so list lands here first and the remaining items verbs
// follow in their own turn.
package items

// ListResponse mirrors the cursor-paginated envelope Miro returns from
// GET /v2/boards/{board_id}/items. data is []map[string]any rather
// than a strongly-typed Item so callers see every field the API
// returned — items come in many flavours (sticky, shape, text,
// connector, frame, doc, image, embed, card, app_card) and a typed
// schema here would either be wrong or huge.
type ListResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int              `json:"total,omitempty"`
	Size   int              `json:"size,omitempty"`
	Cursor string           `json:"cursor,omitempty"`
	Limit  int              `json:"limit,omitempty"`
}

// updateRequest is the PATCH /v2/boards/{board_id}/items/{item_id}
// body. Only position / geometry / parent are mutable through this
// generic endpoint; typed data lives on the per-flavour endpoints
// (cards, stickies, embeds, etc.). All sections are pointers so an
// absent flag stays out of the payload.
type updateRequest struct {
	Position *positionData `json:"position,omitempty"`
	Geometry *geometryData `json:"geometry,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
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

// parentRef mirrors Miro's parent envelope. Setting ID="" with the
// envelope present is how the API expresses "detach from frame";
// omitting the envelope leaves the existing parent untouched.
type parentRef struct {
	ID string `json:"id"`
}

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// detachTagResult is the envelope emitted after DELETE /items/{id}?tag_id=X.
// Distinct from deleteResult so the shape signals "an association was
// removed" rather than "the item itself was deleted."
type detachTagResult struct {
	Detached bool   `json:"detached"`
	ItemID   string `json:"item_id"`
	TagID    string `json:"tag_id"`
}

// listAllResponse is the envelope `list-all` emits after a paginate-
// everything traversal. Mirrors what FetchAll returns to the boards
// composites, but as a JSON-friendly shape with explicit total and
// truncated flag.
type listAllResponse struct {
	Items     []map[string]any `json:"items"`
	Total     int              `json:"total"`
	Truncated bool             `json:"truncated,omitempty"`
}

// bulkOpResult is the per-item record emitted by bulk-delete and
// bulk-update. Status is "success" or "error"; Error carries the
// per-item failure message when set. Output is one of these per input ID.
type bulkOpResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// bulkOpResponse is the aggregate envelope returned by bulk-delete and
// bulk-update. Callers can branch on Failed > 0 instead of inspecting
// per-item Status, and Requested/Succeeded/Failed give a quick summary
// without iterating Results.
type bulkOpResponse struct {
	BoardID   string         `json:"board_id"`
	Requested int            `json:"requested"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Results   []bulkOpResult `json:"results"`
}

// tallyBulk builds the aggregate envelope from per-item results,
// counting success vs error. Results stay in the order miro.FanOut
// returned them, which is input order, so the envelope is identical
// whether the run was sequential (--concurrency=1) or fanned out.
func tallyBulk(boardID string, results []bulkOpResult) bulkOpResponse {
	out := bulkOpResponse{BoardID: boardID, Requested: len(results), Results: results}
	for _, r := range results {
		if r.Status == "success" {
			out.Succeeded++
		} else {
			out.Failed++
		}
	}
	return out
}

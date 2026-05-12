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

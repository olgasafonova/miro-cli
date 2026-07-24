// Package sync implements `miro sync` — an incremental downloader that
// walks /v2/boards and /v2/boards/{id}/items and upserts the responses
// into the local SQLite store. Incrementality is driven by a single
// watermark in sync_metadata ("boards.last_sync"); a board is re-fetched
// only when its modifiedAt has advanced past the watermark. The first
// run sees an empty watermark and performs a full sweep.
//
// Concurrency is deliberately serial: one board's items are downloaded
// before the next board starts. The CLI's rate limiter already throttles
// requests; bounded fan-out is a follow-up (sibling bead 54y) and only
// worth adding if a real account hits the wall.
//
// Conflict policy is last-write-wins. The API is the source of truth, so
// upsert-by-id over what the store already has is the only sensible
// merge — local edits to the store are not a use case.
package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/store"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
	"github.com/olgasafonova/miro-cli/internal/tools/items"
)

// boardsLastSyncKey is the sync_metadata key under which the watermark
// is stored. The value is an RFC3339 timestamp captured at the start of
// the sync run; the next run uses it to decide which boards to refetch.
const boardsLastSyncKey = "boards.last_sync"

// boardsPageSize controls the offset-pagination page size for the board
// list call. Miro's default is 20; we pick a larger page to cut the
// number of round trips during the initial sweep.
const boardsPageSize = 50

// Result is the JSON envelope `miro sync` emits to stdout on success.
// Counts let scripts assert "we did fetch something" without parsing
// logs; SkippedBoards is the count of boards whose modifiedAt was not
// newer than the watermark, so item-fetch was skipped.
type Result struct {
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	Watermark     string `json:"watermark,omitempty"`
	FullSweep     bool   `json:"full_sweep"`
	Boards        int    `json:"boards"`
	BoardsScanned int    `json:"boards_scanned"`
	SkippedBoards int    `json:"skipped_boards"`
	Items         int    `json:"items"`
}

// NewCmd returns the `sync` command. There are no resource-specific
// flags today; everything (token, rate-limit, store path) comes from
// persistent Globals. --since lets the user pin the watermark for the
// run — useful when a previous sync was interrupted and the watermark
// landed too late.
func NewCmd(g *clictx.Globals) *cobra.Command {
	var sinceOverride string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download boards and items into the local store",
		Long: "Walks /v2/boards and /v2/boards/{board_id}/items, upserting\n" +
			"each response into the local SQLite store. Incremental by\n" +
			"default: a board's items are re-downloaded only when the\n" +
			"board's modifiedAt is newer than the stored watermark.\n\n" +
			"--full forces a fetch of every board's items regardless of\n" +
			"the watermark. Use it after a schema change or when you\n" +
			"suspect the store has drifted from the API.\n\n" +
			"Conflict policy is last-write-wins (upsert by id). The API\n" +
			"is the source of truth.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			full, err := cmd.Flags().GetBool("full")
			if err != nil {
				return err
			}
			return run(cmd.Context(), g, runOptions{full: full, since: sinceOverride})
		},
	}
	cmd.Flags().Bool("full", false, "Re-fetch every board's items, ignoring the watermark")
	cmd.Flags().StringVar(&sinceOverride, "since", "", "Override the watermark for this run (RFC3339 timestamp)")
	return cmd
}

// runOptions captures the per-invocation knobs derived from CLI flags.
// Kept as a struct so the testable entry point doesn't grow positional
// arguments every time a flag is added.
type runOptions struct {
	full  bool
	since string
}

func run(ctx context.Context, g *clictx.Globals, opts runOptions) error {
	if g.DryRun {
		// Dry-run mode prints the request the command would send first
		// (boards list) and exits without touching the store. It would
		// be misleading to also print the per-board item URLs because
		// the list response drives them.
		return g.EmitDryRun("GET", "/v2/boards?limit="+strconv.Itoa(boardsPageSize))
	}

	s, err := openStore(ctx, g)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	startedAt := time.Now().UTC()
	watermark, err := resolveWatermark(ctx, s, opts)
	if err != nil {
		return err
	}
	fullSweep := isFullSweep(opts, watermark)

	result := Result{
		StartedAt: startedAt.Format(time.RFC3339),
		FullSweep: fullSweep,
		Watermark: watermark,
	}

	boards, err := fetchAllBoards(ctx, client)
	if err != nil {
		return err
	}
	result.BoardsScanned = len(boards)

	if err := syncBoards(ctx, client, s, boards, fullSweep, watermark, &result); err != nil {
		return err
	}

	// Stamp the new watermark only after every board has been processed
	// successfully. A mid-run failure leaves the previous watermark in
	// place, so the next run can resume by re-syncing the boards the
	// failed run didn't reach.
	if err := s.SetSyncMetadata(ctx, boardsLastSyncKey, startedAt.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("sync: stamp watermark: %w", err)
	}
	result.FinishedAt = time.Now().UTC().Format(time.RFC3339)

	return g.EmitJSON(result)
}

// openStore resolves the store location (--store-path or the package
// default) and opens it read-write for the sync run.
func openStore(ctx context.Context, g *clictx.Globals) (*store.Store, error) {
	path := g.StorePath
	if path == "" {
		p, err := store.DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	s, err := store.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	return s, nil
}

// isFullSweep reports whether this run must fetch every board's items:
// either the user forced it with --full, or there is no watermark yet
// (first run, or a --since override that resolved to empty).
func isFullSweep(opts runOptions, watermark string) bool {
	return opts.full || watermark == ""
}

// resolveWatermark determines the incremental watermark for this run.
// --full ignores it entirely (empty), --since pins it verbatim (RFC3339
// sorts lexically, which the comparison relies on), otherwise it comes
// from sync_metadata — a missing key is a first run, not an error.
func resolveWatermark(ctx context.Context, s *store.Store, opts runOptions) (string, error) {
	if opts.full {
		return "", nil
	}
	if opts.since != "" {
		return opts.since, nil
	}
	w, err := s.GetSyncMetadata(ctx, boardsLastSyncKey)
	if err == nil {
		return w, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return "", fmt.Errorf("sync: read watermark: %w", err)
}

// syncBoards upserts each board and, when it has changed since the
// watermark (or on a full sweep), downloads its items. Counts accumulate
// into result. Honours context cancellation between boards.
func syncBoards(ctx context.Context, client *miro.Client, s *store.Store, boards []map[string]any, fullSweep bool, watermark string, result *Result) error {
	for _, raw := range boards {
		if err := ctx.Err(); err != nil {
			return err
		}
		bp := projectBoard(raw)
		if bp.ID == "" {
			// A board the API returned without an id is degenerate; skip
			// rather than fail the whole run. The raw JSON is in the
			// response body for debugging if needed.
			continue
		}
		if err := upsertBoard(ctx, s, bp, raw); err != nil {
			return err
		}
		result.Boards++

		if !fullSweep && !boardChangedSince(bp.ModifiedAt, watermark) {
			result.SkippedBoards++
			continue
		}

		fetched, err := fetchAndStoreItems(ctx, client, s, bp.ID)
		if err != nil {
			return err
		}
		result.Items += fetched
	}
	return nil
}

// upsertBoard marshals the raw board JSON and upserts the denormalised
// row plus the verbatim blob into the store.
func upsertBoard(ctx context.Context, s *store.Store, bp boardProjection, raw map[string]any) error {
	rawJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("sync: marshal board %s: %w", bp.ID, err)
	}
	return s.UpsertBoard(ctx, store.Board{
		ID:         bp.ID,
		Name:       bp.Name,
		OwnerID:    bp.OwnerID,
		ModifiedAt: bp.ModifiedAt,
		RawJSON:    rawJSON,
	})
}

// boardProjection holds the denormalised fields lifted out of a board
// JSON blob. Mirrors the columns the store cares about; the verbatim
// JSON is preserved alongside, so callers reading via SQL can decode it
// when they need fields outside the denormalisation set.
type boardProjection struct {
	ID         string
	Name       string
	OwnerID    string
	ModifiedAt string
}

// projectBoard lifts the columns the store tracks out of a board JSON
// map. Miro's owner field is an object — accept both "owner.id" and the
// rare flat "owner" string for robustness against minor API drift.
func projectBoard(m map[string]any) boardProjection {
	out := boardProjection{
		ID:         asString(m["id"]),
		Name:       asString(m["name"]),
		ModifiedAt: asString(m["modifiedAt"]),
	}
	switch owner := m["owner"].(type) {
	case map[string]any:
		out.OwnerID = asString(owner["id"])
	case string:
		out.OwnerID = owner
	}
	return out
}

// projectItem lifts the denormalised columns out of an item JSON map.
// position.{x,y} are flattened into PositionX/PositionY; everything
// else lives in raw_json.
func projectItem(boardID string, m map[string]any) store.Item {
	out := store.Item{
		ID:         asString(m["id"]),
		BoardID:    boardID,
		Type:       asString(m["type"]),
		ModifiedAt: asString(m["modifiedAt"]),
	}
	if pos, ok := m["position"].(map[string]any); ok {
		out.PositionX = asFloat(pos["x"])
		out.PositionY = asFloat(pos["y"])
	}
	return out
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// boardChangedSince reports whether the board's modifiedAt is strictly
// after the watermark. RFC3339 is lexically ordered, so string compare
// is correct as long as both sides are in the same offset (Miro emits
// "...Z", and we store the watermark as UTC). Empty modifiedAt is
// treated as "unknown — re-fetch", so we don't silently skip a board
// the API didn't timestamp.
func boardChangedSince(modifiedAt, watermark string) bool {
	if modifiedAt == "" {
		return true
	}
	return modifiedAt > watermark
}

// fetchAllBoards walks the offset-paginated /v2/boards endpoint until
// the response runs out of data or returns fewer rows than requested.
// Miro's pagination envelope includes total/size; we stop on size<limit
// or empty data, whichever comes first.
func fetchAllBoards(ctx context.Context, client *miro.Client) ([]map[string]any, error) {
	var out []map[string]any
	offset := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, total, err := fetchBoardsPage(ctx, client, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, data...)
		if isLastBoardsPage(len(data)) {
			return out, nil
		}
		offset += len(data)
		// total==0 happens on accounts where the count field is omitted;
		// the page-size guard above is the real terminator. Belt-and-
		// braces: stop if total is present and we've reached it.
		if reachedReportedTotal(len(out), total) {
			return out, nil
		}
	}
}

// fetchBoardsPage requests one offset-paginated page of /v2/boards and
// returns its data rows plus the reported total (0 when omitted).
func fetchBoardsPage(ctx context.Context, client *miro.Client, offset int) ([]map[string]any, int, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(boardsPageSize))
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var resp struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
		Size  int              `json:"size"`
	}
	if err := client.Get(ctx, "/v2/boards?"+q.Encode(), &resp); err != nil {
		return nil, 0, fmt.Errorf("sync: list boards: %w", err)
	}
	return resp.Data, resp.Total, nil
}

// isLastBoardsPage reports whether the pagination loop can stop: an empty
// or short page means the server has run out of boards.
func isLastBoardsPage(pageLen int) bool {
	return pageLen == 0 || pageLen < boardsPageSize
}

// reachedReportedTotal is the belt-and-braces terminator for accounts
// that do report a total: stop once we've accumulated that many boards.
func reachedReportedTotal(fetched, total int) bool {
	return total > 0 && fetched >= total
}

// fetchAndStoreItems streams every item for a board into the store via
// cursor pagination. The whole page is upserted in a single transaction
// so a mid-page failure doesn't leave a half-applied page behind.
func fetchAndStoreItems(ctx context.Context, client *miro.Client, s *store.Store, boardID string) (int, error) {
	var total int
	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		resp, err := items.Fetch(ctx, client, items.ListFlags{BoardID: boardID, Cursor: cursor})
		if err != nil {
			return total, fmt.Errorf("sync: list items for %s: %w", boardID, err)
		}
		stored, err := storeItemPage(ctx, s, boardID, resp.Data)
		if err != nil {
			return total, err
		}
		total += stored
		if resp.Cursor == "" {
			return total, nil
		}
		cursor = resp.Cursor
	}
}

// storeItemPage projects one page of raw item JSON and upserts it in a
// single transaction, returning how many rows were written. An empty
// page (or one where every entry lacked an id) writes nothing.
func storeItemPage(ctx context.Context, s *store.Store, boardID string, rawItems []map[string]any) (int, error) {
	batch, err := projectItemBatch(boardID, rawItems)
	if err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}
	if err := s.UpsertItems(ctx, batch); err != nil {
		return 0, err
	}
	return len(batch), nil
}

// projectItemBatch converts a page of raw item JSON into store.Item rows,
// dropping entries without an id and attaching the verbatim blob to each.
func projectItemBatch(boardID string, rawItems []map[string]any) ([]store.Item, error) {
	batch := make([]store.Item, 0, len(rawItems))
	for _, raw := range rawItems {
		it := projectItem(boardID, raw)
		if it.ID == "" {
			continue
		}
		rawJSON, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("sync: marshal item %s on board %s: %w", it.ID, boardID, err)
		}
		it.RawJSON = rawJSON
		batch = append(batch, it)
	}
	return batch, nil
}

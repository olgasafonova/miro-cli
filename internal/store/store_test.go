package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// newStore opens a Store at a per-test tempdir path. The store is closed
// on Cleanup; tests don't need to defer.
func newStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenStampsSchemaVersion(t *testing.T) {
	s := newStore(t)
	v, err := s.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", v, SchemaVersion)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	for i := 0; i < 3; i++ {
		s, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("Open #%d: %v", i, err)
		}
		v, err := s.SchemaVersion(context.Background())
		if err != nil {
			t.Fatalf("SchemaVersion #%d: %v", i, err)
		}
		if v != SchemaVersion {
			t.Errorf("SchemaVersion #%d = %d, want %d", i, v, SchemaVersion)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i, err)
		}
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Stamp a future version to simulate a newer binary having written
	// the file.
	if _, err := s.db.ExecContext(context.Background(),
		fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion+1)); err != nil {
		t.Fatalf("bump user_version: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	_, err = Open(context.Background(), path)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("Open on newer schema = %v, want errors.Is ErrSchemaTooNew", err)
	}
}

func TestOpenCreatesParentDir(t *testing.T) {
	// Path with two nonexistent intermediate directories.
	path := filepath.Join(t.TempDir(), "nested", "dirs", "store.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open with missing parents: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestUpsertAndGetBoard(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	b := Board{
		ID:         "b1",
		Name:       "Roadmap",
		OwnerID:    "u1",
		ModifiedAt: "2026-05-14T10:00:00Z",
		RawJSON:    []byte(`{"id":"b1","name":"Roadmap"}`),
	}
	if err := s.UpsertBoard(ctx, b); err != nil {
		t.Fatalf("UpsertBoard: %v", err)
	}

	got, err := s.GetBoard(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	if got.Name != "Roadmap" || got.OwnerID != "u1" || string(got.RawJSON) != string(b.RawJSON) {
		t.Errorf("GetBoard = %+v, want %+v", got, b)
	}

	// Upsert again with a renamed board; the row should be replaced.
	b.Name = "Renamed"
	b.RawJSON = []byte(`{"id":"b1","name":"Renamed"}`)
	if err := s.UpsertBoard(ctx, b); err != nil {
		t.Fatalf("UpsertBoard (rename): %v", err)
	}
	got, err = s.GetBoard(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBoard (rename): %v", err)
	}
	if got.Name != "Renamed" {
		t.Errorf("after rename, GetBoard.Name = %q, want Renamed", got.Name)
	}
}

func TestGetBoardMissingIsErrNoRows(t *testing.T) {
	s := newStore(t)
	_, err := s.GetBoard(context.Background(), "nope")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetBoard on missing id = %v, want sql.ErrNoRows", err)
	}
}

func TestUpsertBoardsBatch(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	batch := []Board{
		{ID: "b1", Name: "A", RawJSON: []byte(`{}`)},
		{ID: "b2", Name: "B", RawJSON: []byte(`{}`)},
		{ID: "b3", Name: "C", RawJSON: []byte(`{}`)},
	}
	if err := s.UpsertBoards(ctx, batch); err != nil {
		t.Fatalf("UpsertBoards: %v", err)
	}

	list, err := s.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListBoards count = %d, want 3", len(list))
	}
	if list[0].ID != "b1" || list[2].ID != "b3" {
		t.Errorf("ListBoards ordering not stable: %+v", list)
	}
}

func TestUpsertBoardRejectsEmpty(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	cases := []struct {
		name string
		b    Board
	}{
		{"missing id", Board{RawJSON: []byte(`{}`)}},
		{"missing raw_json", Board{ID: "b1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.UpsertBoard(ctx, tc.b); err == nil {
				t.Error("UpsertBoard accepted invalid input")
			}
		})
	}
}

func TestUpsertAndListItems(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	// Items reference a board via FK; insert the board first.
	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}

	items := []Item{
		{ID: "i1", BoardID: "b1", Type: "sticky_note", PositionX: 10, PositionY: 20, RawJSON: []byte(`{}`)},
		{ID: "i2", BoardID: "b1", Type: "shape", PositionX: 30, PositionY: 40, RawJSON: []byte(`{}`)},
	}
	if err := s.UpsertItems(ctx, items); err != nil {
		t.Fatalf("UpsertItems: %v", err)
	}

	got, err := s.ListItemsByBoard(ctx, "b1")
	if err != nil {
		t.Fatalf("ListItemsByBoard: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListItemsByBoard count = %d, want 2", len(got))
	}
	if got[0].Type != "sticky_note" || got[1].Type != "shape" {
		t.Errorf("ListItemsByBoard types = [%s, %s], want [sticky_note, shape]", got[0].Type, got[1].Type)
	}
}

func TestItemForeignKeyCascade(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if err := s.UpsertItem(ctx, Item{ID: "i1", BoardID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	// Deleting the board should cascade — verifies the FK is on and the
	// pragma was applied at Open time.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM boards WHERE id = ?`, "b1"); err != nil {
		t.Fatalf("delete board: %v", err)
	}
	_, err := s.GetItem(ctx, "i1")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after board delete, GetItem(i1) = %v, want sql.ErrNoRows (FK cascade)", err)
	}
}

func TestSyncMetadataRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if _, err := s.GetSyncMetadata(ctx, "boards.last_sync"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unset key returned %v, want sql.ErrNoRows", err)
	}

	if err := s.SetSyncMetadata(ctx, "boards.last_sync", "2026-05-14T10:00:00Z"); err != nil {
		t.Fatalf("SetSyncMetadata: %v", err)
	}
	v, err := s.GetSyncMetadata(ctx, "boards.last_sync")
	if err != nil {
		t.Fatalf("GetSyncMetadata: %v", err)
	}
	if v != "2026-05-14T10:00:00Z" {
		t.Errorf("GetSyncMetadata = %q, want 2026-05-14T10:00:00Z", v)
	}

	// Overwrite — must replace, not append.
	if err := s.SetSyncMetadata(ctx, "boards.last_sync", "2026-05-15T10:00:00Z"); err != nil {
		t.Fatalf("SetSyncMetadata (overwrite): %v", err)
	}
	v, err = s.GetSyncMetadata(ctx, "boards.last_sync")
	if err != nil {
		t.Fatalf("GetSyncMetadata (after overwrite): %v", err)
	}
	if v != "2026-05-15T10:00:00Z" {
		t.Errorf("GetSyncMetadata after overwrite = %q, want 2026-05-15T10:00:00Z", v)
	}
}

func TestConcurrentUpserts(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b := Board{ID: fmt.Sprintf("b%d", n), RawJSON: []byte(`{}`)}
			for j := 0; j < 20; j++ {
				if err := s.UpsertBoard(ctx, b); err != nil {
					t.Errorf("concurrent UpsertBoard: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	list, err := s.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}
	if len(list) != 8 {
		t.Errorf("concurrent upserts produced %d boards, want 8", len(list))
	}
}

func TestOpenReadOnlyRejectsWrites(t *testing.T) {
	// Seed a store with one board, close it, reopen read-only.
	path := filepath.Join(t.TempDir(), "store.db")
	ctx := context.Background()
	w, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open (write): %v", err)
	}
	if err := w.UpsertBoard(ctx, Board{ID: "b1", Name: "seed", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close (write): %v", err)
	}

	r, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	// Read path works.
	got, err := r.GetBoard(ctx, "b1")
	if err != nil {
		t.Fatalf("GetBoard via RO: %v", err)
	}
	if got.Name != "seed" {
		t.Errorf("GetBoard.Name = %q, want seed", got.Name)
	}

	// Write attempts must fail.
	_, err = r.DB().ExecContext(ctx, `INSERT INTO boards (id, raw_json, synced_at) VALUES (?, ?, ?)`,
		"b2", `{}`, "2026-05-14T00:00:00Z")
	if err == nil {
		t.Error("INSERT against read-only handle succeeded; want error")
	}
}

func TestOpenReadOnlyMissingFile(t *testing.T) {
	_, err := OpenReadOnly(context.Background(), filepath.Join(t.TempDir(), "does-not-exist.db"))
	if err == nil {
		t.Error("OpenReadOnly on missing file succeeded; want error")
	}
}

func TestDefaultPathPrefersXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/xdg")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/custom/xdg", "miro-cli", "store.db")
	if got != want {
		t.Errorf("DefaultPath with XDG_DATA_HOME = %q, want %q", got, want)
	}
}

// stickyJSON builds a Miro-shaped sticky_note raw_json with the given
// content string, so FTS tests can drive realistic input through the
// trigger's json_extract path.
func stickyJSON(id, content string) []byte {
	return []byte(fmt.Sprintf(`{"id":%q,"type":"sticky_note","data":{"content":%q}}`, id, content))
}

// cardJSON builds a card-shaped raw_json with title + description, used
// to verify the trigger pulls in both fields and concatenates them.
func cardJSON(id, title, description string) []byte {
	return []byte(fmt.Sprintf(`{"id":%q,"type":"card","data":{"title":%q,"description":%q}}`, id, title, description))
}

// ftsMatches runs a MATCH against items_fts and returns the matching
// item_ids, alphabetically sorted for stable assertions.
func ftsMatches(t *testing.T, s *Store, query string) []string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT item_id FROM items_fts WHERE items_fts MATCH ? ORDER BY item_id`, query)
	if err != nil {
		t.Fatalf("items_fts MATCH %q: %v", query, err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	return ids
}

// requireMatch asserts that running query against items_fts yields
// exactly wantIDs (in id-ascending order). Call with no wantIDs to
// assert "no rows" — ftsMatches returns a nil slice for an empty result
// and the variadic with zero args is also nil, so DeepEqual works.
func requireMatch(t *testing.T, s *Store, query string, wantIDs ...string) {
	t.Helper()
	got := ftsMatches(t, s, query)
	if !reflect.DeepEqual(got, wantIDs) {
		t.Errorf("MATCH %q = %v, want %v", query, got, wantIDs)
	}
}

// rollbackStoreToV1 strips the FTS triggers + table and rolls
// user_version back to 1 so a follow-up Open() exercises the v1->v2
// migration on a store that already has rows. Batches the five DROP /
// PRAGMA calls behind a single error-handler.
func rollbackStoreToV1(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`DROP TRIGGER IF EXISTS items_au_fts`,
		`DROP TRIGGER IF EXISTS items_ad_fts`,
		`DROP TRIGGER IF EXISTS items_ai_fts`,
		`DROP TABLE items_fts`,
		`PRAGMA user_version = 1`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("rollbackStoreToV1 %q: %v", stmt, err)
		}
	}
}

func TestFTSInsertPopulatesIndex(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	items := []Item{
		{ID: "i1", BoardID: "b1", Type: "sticky_note", RawJSON: stickyJSON("i1", "the quick brown fox")},
		{ID: "i2", BoardID: "b1", Type: "sticky_note", RawJSON: stickyJSON("i2", "lazy dog naps")},
		{ID: "i3", BoardID: "b1", Type: "card", RawJSON: cardJSON("i3", "Fox Plan", "quarterly review")},
	}
	if err := s.UpsertItems(ctx, items); err != nil {
		t.Fatalf("UpsertItems: %v", err)
	}

	// unicode61 (the FTS5 default) does not stem, so "fox" matches the
	// sticky's "fox" and the card's "Fox" but would not match "foxes".
	// This is intentional — the bead's scope is basic MATCH only.
	requireMatch(t, s, "fox", "i1", "i3")
	// Phrase match across two tokens from the card's title — verifies
	// title is being pulled into the content column by the trigger.
	requireMatch(t, s, `"Fox Plan"`, "i3")
	// Word from the card description proves description is concatenated
	// into content alongside title.
	requireMatch(t, s, "quarterly", "i3")
}

func TestFTSUpdateReflectsNewContent(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if err := s.UpsertItem(ctx, Item{
		ID: "i1", BoardID: "b1", Type: "sticky_note", RawJSON: stickyJSON("i1", "alpha beta"),
	}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	requireMatch(t, s, "alpha", "i1")

	// Rewrite the same id with new content. The AFTER UPDATE trigger
	// must drop the old FTS row and insert the new one.
	if err := s.UpsertItem(ctx, Item{
		ID: "i1", BoardID: "b1", Type: "sticky_note", RawJSON: stickyJSON("i1", "gamma delta"),
	}); err != nil {
		t.Fatalf("rewrite item: %v", err)
	}
	requireMatch(t, s, "alpha")
	requireMatch(t, s, "gamma", "i1")
}

func TestFTSDeleteRemovesIndexRow(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if err := s.UpsertItem(ctx, Item{
		ID: "i1", BoardID: "b1", Type: "sticky_note", RawJSON: stickyJSON("i1", "ephemeral"),
	}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	requireMatch(t, s, "ephemeral", "i1")

	// Cascade via board delete — verifies the AFTER DELETE trigger on
	// items fires for cascaded rows, not just explicit DELETE FROM items.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM boards WHERE id = ?`, "b1"); err != nil {
		t.Fatalf("delete board: %v", err)
	}
	requireMatch(t, s, "ephemeral")
}

func TestFTSBackfillFromV1(t *testing.T) {
	// Build a v1-only store, write items into it, then reopen with the
	// current binary and verify the v1→v2 migration backfilled FTS.
	path := filepath.Join(t.TempDir(), "store.db")
	ctx := context.Background()

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if err := s.UpsertItem(ctx, Item{
		ID: "i1", BoardID: "b1", Type: "sticky_note", RawJSON: stickyJSON("i1", "needle haystack"),
	}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	// Drop the FTS table and triggers and roll the schema back to v1 to
	// simulate a store written by an older binary that didn't ship FTS.
	rollbackStoreToV1(t, s)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen — migrate should apply v2 (creates table, triggers,
	// backfills the existing row).
	s2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	v, err := s2.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if v != SchemaVersion {
		t.Errorf("post-migrate SchemaVersion = %d, want %d", v, SchemaVersion)
	}
	requireMatch(t, s2, "needle", "i1")
}

func TestFTSEmptyContentDoesNotMatch(t *testing.T) {
	// An item whose raw_json has no data.content / title / description
	// should yield an empty content string after trim(); the row goes
	// into FTS but matches nothing.
	s := newStore(t)
	ctx := context.Background()
	if err := s.UpsertBoard(ctx, Board{ID: "b1", RawJSON: []byte(`{}`)}); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if err := s.UpsertItem(ctx, Item{
		ID: "i1", BoardID: "b1", Type: "shape", RawJSON: []byte(`{"id":"i1","type":"shape"}`),
	}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	requireMatch(t, s, "anything")
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/fakehome")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join("/tmp/fakehome", ".local", "share", "miro-cli", "store.db")
	if got != want {
		t.Errorf("DefaultPath fallback = %q, want %q", got, want)
	}
}

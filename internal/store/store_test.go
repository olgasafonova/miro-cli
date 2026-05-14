package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
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

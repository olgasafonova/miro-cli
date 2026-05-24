package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/store"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// fakeAPI is a minimal Miro-shaped server: serves a configurable list
// of boards and a per-board item map. Cursor pagination on the items
// endpoint advances one page at a time so tests can exercise the
// pagination loop without writing dozens of pages of fixture JSON.
type fakeAPI struct {
	t       *testing.T
	boards  []map[string]any
	items   map[string][]map[string]any
	calls   atomic.Int64
	srv     *httptest.Server
	perPage int
}

func newFakeAPI(t *testing.T, boards []map[string]any, items map[string][]map[string]any) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t, boards: boards, items: items, perPage: 50}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(func() { f.srv.Close() })
	return f
}

func (f *fakeAPI) URL() string { return f.srv.URL }
func (f *fakeAPI) Calls() int  { return int(f.calls.Load()) }

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	switch {
	case r.URL.Path == "/v2/boards":
		f.handleBoards(w, r)
	case strings.HasPrefix(r.URL.Path, "/v2/boards/") && strings.HasSuffix(r.URL.Path, "/items"):
		f.handleItems(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAPI) handleBoards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := f.perPage
	if v := q.Get("limit"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &limit)
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &offset)
	}
	end := offset + limit
	if end > len(f.boards) {
		end = len(f.boards)
	}
	page := []map[string]any{}
	if offset < len(f.boards) {
		page = f.boards[offset:end]
	}
	body := map[string]any{
		"data":  page,
		"total": len(f.boards),
		"size":  len(page),
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (f *fakeAPI) handleItems(w http.ResponseWriter, r *http.Request) {
	// path is /v2/boards/{id}/items
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	boardID := parts[2]
	all := f.items[boardID]

	q := r.URL.Query()
	cursor := q.Get("cursor")
	// Cursor encodes the starting offset as a plain integer string for
	// test simplicity; real Miro cursors are opaque blobs.
	start := 0
	if cursor != "" {
		_, _ = fmt.Sscanf(cursor, "%d", &start)
	}
	pageSize := 2 // small page so multi-page tests are cheap
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	page := []map[string]any{}
	if start < len(all) {
		page = all[start:end]
	}
	next := ""
	if end < len(all) {
		next = fmt.Sprintf("%d", end)
	}
	body := map[string]any{
		"data":   page,
		"size":   len(page),
		"cursor": next,
	}
	_ = json.NewEncoder(w).Encode(body)
}

func newGlobals(t *testing.T, baseURL string) (*clictx.Globals, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.db")
	g := &clictx.Globals{
		StorePath: path,
		JSON:      true,
		Stdout:    &bytes.Buffer{},
		Stderr:    io.Discard,
		Client:    miro.New(&miro.Config{Token: "t", BaseURL: baseURL}),
	}
	return g, path
}

func decodeResult(t *testing.T, g *clictx.Globals) Result {
	t.Helper()
	buf := g.Stdout.(*bytes.Buffer)
	var r Result
	if err := json.Unmarshal(buf.Bytes(), &r); err != nil {
		t.Fatalf("decode result: %v\nraw: %s", err, buf.String())
	}
	return r
}

func TestRunFirstSweepDownloadsBoardsAndItems(t *testing.T) {
	boards := []map[string]any{
		{"id": "b1", "name": "Roadmap", "modifiedAt": "2026-05-14T10:00:00Z", "owner": map[string]any{"id": "u1"}},
		{"id": "b2", "name": "Retro", "modifiedAt": "2026-05-14T11:00:00Z"},
	}
	items := map[string][]map[string]any{
		"b1": {
			{"id": "i1", "type": "sticky_note", "modifiedAt": "2026-05-14T10:00:00Z", "position": map[string]any{"x": 1.5, "y": 2.5}},
			{"id": "i2", "type": "shape", "modifiedAt": "2026-05-14T10:01:00Z"},
			{"id": "i3", "type": "text", "modifiedAt": "2026-05-14T10:02:00Z"},
		},
		"b2": {
			{"id": "i4", "type": "frame", "modifiedAt": "2026-05-14T11:00:00Z"},
		},
	}
	api := newFakeAPI(t, boards, items)
	g, path := newGlobals(t, api.URL())

	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	r := decodeResult(t, g)
	if !r.FullSweep {
		t.Error("first run with empty watermark should report FullSweep")
	}
	if r.Boards != 2 {
		t.Errorf("Boards = %d, want 2", r.Boards)
	}
	if r.Items != 4 {
		t.Errorf("Items = %d, want 4", r.Items)
	}
	if r.SkippedBoards != 0 {
		t.Errorf("SkippedBoards = %d, want 0", r.SkippedBoards)
	}

	// Verify the store has the rows we expect.
	s, err := store.OpenReadOnly(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer func() { _ = s.Close() }()
	boardRows, err := s.ListBoards(context.Background())
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}
	if len(boardRows) != 2 {
		t.Errorf("stored boards = %d, want 2", len(boardRows))
	}
	got, err := s.GetBoard(context.Background(), "b1")
	if err != nil {
		t.Fatalf("GetBoard(b1): %v", err)
	}
	if got.Name != "Roadmap" || got.OwnerID != "u1" {
		t.Errorf("b1 projection = %+v", got)
	}
	b1Items, err := s.ListItemsByBoard(context.Background(), "b1")
	if err != nil {
		t.Fatalf("ListItemsByBoard(b1): %v", err)
	}
	if len(b1Items) != 3 {
		t.Errorf("b1 stored items = %d, want 3", len(b1Items))
	}
	// position should have been lifted out of raw json.
	for _, it := range b1Items {
		if it.ID == "i1" && (it.PositionX != 1.5 || it.PositionY != 2.5) {
			t.Errorf("i1 position = (%v,%v), want (1.5,2.5)", it.PositionX, it.PositionY)
		}
	}
}

func TestRunIncrementalSkipsUnchangedBoards(t *testing.T) {
	// Two boards: b1 modified after the first run, b2 modified before.
	boards := []map[string]any{
		{"id": "b1", "name": "Hot", "modifiedAt": "2026-05-15T10:00:00Z"},
		{"id": "b2", "name": "Cold", "modifiedAt": "2026-04-01T10:00:00Z"},
	}
	itemsMap := map[string][]map[string]any{
		"b1": {{"id": "i1", "type": "sticky_note", "modifiedAt": "2026-05-15T10:00:00Z"}},
		"b2": {{"id": "i2", "type": "shape", "modifiedAt": "2026-04-01T10:00:00Z"}},
	}
	api := newFakeAPI(t, boards, itemsMap)
	g, path := newGlobals(t, api.URL())

	// Pre-seed the store watermark so the run is incremental.
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	if err := s.SetSyncMetadata(context.Background(), boardsLastSyncKey, "2026-05-01T00:00:00Z"); err != nil {
		t.Fatalf("seed watermark: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("seed close: %v", err)
	}

	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	r := decodeResult(t, g)
	if r.FullSweep {
		t.Error("incremental run reported FullSweep=true")
	}
	if r.Boards != 2 {
		t.Errorf("Boards = %d, want 2 (both upserted)", r.Boards)
	}
	if r.Items != 1 {
		t.Errorf("Items = %d, want 1 (only the changed board)", r.Items)
	}
	if r.SkippedBoards != 1 {
		t.Errorf("SkippedBoards = %d, want 1", r.SkippedBoards)
	}
	if r.Watermark != "2026-05-01T00:00:00Z" {
		t.Errorf("Watermark in result = %q, want 2026-05-01T00:00:00Z", r.Watermark)
	}
}

func TestRunFullFlagForcesAllItemsFetch(t *testing.T) {
	boards := []map[string]any{
		{"id": "b1", "modifiedAt": "2026-04-01T10:00:00Z"},
	}
	itemsMap := map[string][]map[string]any{
		"b1": {{"id": "i1", "type": "sticky_note"}},
	}
	api := newFakeAPI(t, boards, itemsMap)
	g, path := newGlobals(t, api.URL())

	// Watermark says nothing has changed since the board's modifiedAt.
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	if err := s.SetSyncMetadata(context.Background(), boardsLastSyncKey, "2026-05-01T00:00:00Z"); err != nil {
		t.Fatalf("seed watermark: %v", err)
	}
	_ = s.Close()

	if err := run(context.Background(), g, runOptions{full: true}); err != nil {
		t.Fatalf("run: %v", err)
	}
	r := decodeResult(t, g)
	if !r.FullSweep {
		t.Error("--full should set FullSweep=true")
	}
	if r.Items != 1 {
		t.Errorf("Items = %d, want 1 (full sweep fetches the unchanged board)", r.Items)
	}
}

func TestRunSinceOverrideTakesPrecedenceOverStoredWatermark(t *testing.T) {
	boards := []map[string]any{
		{"id": "b1", "modifiedAt": "2026-05-10T10:00:00Z"},
	}
	itemsMap := map[string][]map[string]any{
		"b1": {{"id": "i1", "type": "sticky_note"}},
	}
	api := newFakeAPI(t, boards, itemsMap)
	g, path := newGlobals(t, api.URL())

	// Stored watermark would skip; --since pushes the threshold further
	// back so the board is treated as changed.
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	if err := s.SetSyncMetadata(context.Background(), boardsLastSyncKey, "2026-06-01T00:00:00Z"); err != nil {
		t.Fatalf("seed watermark: %v", err)
	}
	_ = s.Close()

	if err := run(context.Background(), g, runOptions{since: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	r := decodeResult(t, g)
	if r.Watermark != "2026-01-01T00:00:00Z" {
		t.Errorf("Watermark = %q, want override value", r.Watermark)
	}
	if r.Items != 1 {
		t.Errorf("Items = %d, want 1 (override should NOT skip)", r.Items)
	}
}

func TestRunWatermarkAdvancedAfterSuccess(t *testing.T) {
	boards := []map[string]any{
		{"id": "b1", "modifiedAt": "2026-05-14T10:00:00Z"},
	}
	api := newFakeAPI(t, boards, map[string][]map[string]any{"b1": nil})
	g, path := newGlobals(t, api.URL())

	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	s, err := store.OpenReadOnly(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer func() { _ = s.Close() }()
	v, err := s.GetSyncMetadata(context.Background(), boardsLastSyncKey)
	if err != nil {
		t.Fatalf("GetSyncMetadata: %v", err)
	}
	if v == "" {
		t.Error("watermark not stamped after successful run")
	}
}

func TestRunPaginatesBoards(t *testing.T) {
	// 75 boards > one page (50) so the second page is required.
	var boards []map[string]any
	for i := 0; i < 75; i++ {
		boards = append(boards, map[string]any{
			"id":         fmt.Sprintf("b%d", i),
			"modifiedAt": "2026-05-14T10:00:00Z",
		})
	}
	api := newFakeAPI(t, boards, map[string][]map[string]any{})
	g, _ := newGlobals(t, api.URL())

	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	r := decodeResult(t, g)
	if r.Boards != 75 {
		t.Errorf("Boards = %d, want 75", r.Boards)
	}
}

func TestRunPaginatesItemsViaCursor(t *testing.T) {
	// 5 items per board with the fake API's pageSize=2 → 3 pages.
	itemsB1 := make([]map[string]any, 5)
	for i := range itemsB1 {
		itemsB1[i] = map[string]any{"id": fmt.Sprintf("i%d", i), "type": "sticky_note"}
	}
	api := newFakeAPI(t,
		[]map[string]any{{"id": "b1", "modifiedAt": "2026-05-14T10:00:00Z"}},
		map[string][]map[string]any{"b1": itemsB1},
	)
	g, _ := newGlobals(t, api.URL())

	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	r := decodeResult(t, g)
	if r.Items != 5 {
		t.Errorf("Items = %d, want 5 (cursor pagination)", r.Items)
	}
}

func TestRunDryRunEmitsRequestAndSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	g, _ := newGlobals(t, srv.URL)
	g.DryRun = true

	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(g.Stdout.(*bytes.Buffer).String(), "DRY-RUN GET /v2/boards?limit=50") {
		t.Errorf("dry-run stdout = %q", g.Stdout.(*bytes.Buffer).String())
	}
}

func TestRunFailureLeavesWatermarkUnchanged(t *testing.T) {
	// Boards endpoint succeeds; items endpoint 500s. The watermark must
	// stay at the seeded value so the next run reprocesses the failed board.
	calls := atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/items") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"b1","modifiedAt":"2026-05-14T10:00:00Z"}],"total":1,"size":1}`))
	}))
	defer srv.Close()

	g, path := newGlobals(t, srv.URL)
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	if err := s.SetSyncMetadata(context.Background(), boardsLastSyncKey, "2026-04-01T00:00:00Z"); err != nil {
		t.Fatalf("seed watermark: %v", err)
	}
	_ = s.Close()

	err = run(context.Background(), g, runOptions{})
	if err == nil {
		t.Fatal("run succeeded despite 500 on items endpoint")
	}

	// Watermark must not have been stamped.
	s2, err := store.OpenReadOnly(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer func() { _ = s2.Close() }()
	v, err := s2.GetSyncMetadata(context.Background(), boardsLastSyncKey)
	if err != nil {
		t.Fatalf("GetSyncMetadata: %v", err)
	}
	if v != "2026-04-01T00:00:00Z" {
		t.Errorf("watermark mutated after failure: %q, want 2026-04-01T00:00:00Z", v)
	}
}

func TestRunSerialOrderingOfItemRequests(t *testing.T) {
	// Sanity check the "one board at a time" promise: with two boards
	// and a server that records request arrival order, we expect all
	// items for board b1 to arrive before any items for b2.
	type rec struct {
		path string
	}
	var got []rec
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, rec{path: r.URL.Path})
		switch r.URL.Path {
		case "/v2/boards":
			_, _ = w.Write([]byte(`{"data":[{"id":"b1","modifiedAt":"2026-05-14T10:00:00Z"},{"id":"b2","modifiedAt":"2026-05-14T10:00:00Z"}],"total":2,"size":2}`))
		case "/v2/boards/b1/items":
			_, _ = w.Write([]byte(`{"data":[{"id":"i1","type":"sticky_note"}],"size":1,"cursor":""}`))
		case "/v2/boards/b2/items":
			_, _ = w.Write([]byte(`{"data":[{"id":"i2","type":"shape"}],"size":1,"cursor":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g, _ := newGlobals(t, srv.URL)
	if err := run(context.Background(), g, runOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	// expected order: boards list, b1 items, b2 items
	if len(got) < 3 {
		t.Fatalf("got %d calls, want 3+", len(got))
	}
	if got[0].path != "/v2/boards" || got[1].path != "/v2/boards/b1/items" || got[2].path != "/v2/boards/b2/items" {
		paths := make([]string, len(got))
		for i, r := range got {
			paths[i] = r.path
		}
		t.Errorf("request order = %v, want boards, b1/items, b2/items", paths)
	}
}

func TestProjectBoardOwnerVariants(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want string
	}{
		{"object form", map[string]any{"owner": map[string]any{"id": "u1"}}, "u1"},
		{"flat string", map[string]any{"owner": "u2"}, "u2"},
		{"missing", map[string]any{}, ""},
		{"object without id", map[string]any{"owner": map[string]any{"name": "X"}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := projectBoard(tc.in).OwnerID
			if got != tc.want {
				t.Errorf("OwnerID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBoardChangedSince(t *testing.T) {
	cases := []struct {
		name      string
		modified  string
		watermark string
		want      bool
	}{
		{"after", "2026-05-15T00:00:00Z", "2026-05-14T00:00:00Z", true},
		{"equal", "2026-05-14T00:00:00Z", "2026-05-14T00:00:00Z", false},
		{"before", "2026-05-13T00:00:00Z", "2026-05-14T00:00:00Z", false},
		{"empty modified treated as changed", "", "2026-05-14T00:00:00Z", true},
		{"empty watermark treated as before", "2026-05-13T00:00:00Z", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := boardChangedSince(tc.modified, tc.watermark); got != tc.want {
				t.Errorf("boardChangedSince(%q,%q) = %v, want %v", tc.modified, tc.watermark, got, tc.want)
			}
		})
	}
}

func TestNewCmdRegistersSync(t *testing.T) {
	cmd := NewCmd(clictx.New())
	if cmd.Use != "sync" {
		t.Errorf("Use = %q, want sync", cmd.Use)
	}
	if f := cmd.Flags().Lookup("full"); f == nil {
		t.Error("--full flag not registered")
	}
	if f := cmd.Flags().Lookup("since"); f == nil {
		t.Error("--since flag not registered")
	}
}

package stickies

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ----- layout ---------------------------------------------------------------

func TestBuildGridItemsLaysOutRowMajor(t *testing.T) {
	t.Parallel()
	items := buildGridItems(
		createGridFlags{columns: 3, spacing: 220, startX: 100, startY: 50},
		[]string{"a", "b", "c", "d", "e"},
	)
	if len(items) != 5 {
		t.Fatalf("len = %d, want 5", len(items))
	}
	wantPositions := []struct{ x, y float64 }{
		{100, 50},  // row 0 col 0
		{320, 50},  // row 0 col 1
		{540, 50},  // row 0 col 2
		{100, 270}, // row 1 col 0
		{320, 270}, // row 1 col 1
	}
	for i, w := range wantPositions {
		if items[i].Position == nil {
			t.Fatalf("item[%d] position nil", i)
		}
		if items[i].Position.X != w.x || items[i].Position.Y != w.y {
			t.Errorf("item[%d] position = (%v,%v), want (%v,%v)", i, items[i].Position.X, items[i].Position.Y, w.x, w.y)
		}
		if items[i].Position.Origin != "center" {
			t.Errorf("item[%d] origin = %q, want center", i, items[i].Position.Origin)
		}
		if items[i].Type != "sticky_note" {
			t.Errorf("item[%d] type = %q, want sticky_note", i, items[i].Type)
		}
		if items[i].Data == nil || items[i].Data.Content != []string{"a", "b", "c", "d", "e"}[i] {
			t.Errorf("item[%d] data = %+v", i, items[i].Data)
		}
	}
}

func TestBuildGridItemsDefaultsColumnsAndSpacing(t *testing.T) {
	t.Parallel()
	// columns=0 -> 3, spacing=0 -> 220
	items := buildGridItems(createGridFlags{}, []string{"a", "b", "c", "d"})
	// 4th item should be on the second row (row=1, col=0)
	if items[3].Position.X != 0 || items[3].Position.Y != 220 {
		t.Errorf("item[3] position = (%v,%v), want (0,220)", items[3].Position.X, items[3].Position.Y)
	}
}

func TestBuildGridItemsAppliesColorAndParent(t *testing.T) {
	t.Parallel()
	items := buildGridItems(
		createGridFlags{color: "yellow", parentID: "frame-1", columns: 2},
		[]string{"a", "b"},
	)
	for i, item := range items {
		if item.Style == nil || item.Style.FillColor != "light_yellow" {
			t.Errorf("item[%d] style = %+v, want fillColor=light_yellow", i, item.Style)
		}
		if item.Parent == nil || item.Parent.ID != "frame-1" {
			t.Errorf("item[%d] parent = %+v, want id=frame-1", i, item.Parent)
		}
	}
}

func TestBuildGridItemsOmitsStyleAndParentWhenUnset(t *testing.T) {
	t.Parallel()
	items := buildGridItems(createGridFlags{columns: 1}, []string{"only"})
	if items[0].Style != nil {
		t.Errorf("style should be nil when --color unset, got %+v", items[0].Style)
	}
	if items[0].Parent != nil {
		t.Errorf("parent should be nil when --parent-id unset, got %+v", items[0].Parent)
	}
}

// ----- contents loading -----------------------------------------------------

func TestLoadGridContentsFromJSON(t *testing.T) {
	t.Parallel()
	got, err := loadGridContents(createGridFlags{contentsJSON: `["one","two","three"]`})
	if err != nil {
		t.Fatalf("loadGridContents: %v", err)
	}
	if len(got) != 3 || got[0] != "one" || got[2] != "three" {
		t.Errorf("contents = %v", got)
	}
}

func TestLoadGridContentsFromFileSkipsBlankLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "stickies.txt")
	if err := os.WriteFile(p, []byte("alpha\r\n\nbeta\n  \ngamma\n"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	got, err := loadGridContents(createGridFlags{contentsFile: p})
	if err != nil {
		t.Fatalf("loadGridContents: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("contents[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestLoadGridContentsRejectsBothFlags(t *testing.T) {
	t.Parallel()
	_, err := loadGridContents(createGridFlags{contentsFile: "x", contentsJSON: `["a"]`})
	if err == nil {
		t.Error("both flags set should error")
	}
}

func TestLoadGridContentsRejectsNeitherFlag(t *testing.T) {
	t.Parallel()
	if _, err := loadGridContents(createGridFlags{}); err == nil {
		t.Error("no flags set should error")
	}
}

func TestLoadGridContentsRejectsEmptyArray(t *testing.T) {
	t.Parallel()
	if _, err := loadGridContents(createGridFlags{contentsJSON: `[]`}); err == nil {
		t.Error("empty array should error")
	}
}

func TestLoadGridContentsRejectsNonStringArray(t *testing.T) {
	t.Parallel()
	if _, err := loadGridContents(createGridFlags{contentsJSON: `[1,2,3]`}); err == nil {
		t.Error("non-string array should error")
	}
}

func TestLoadGridContentsRejectsOverCap(t *testing.T) {
	t.Parallel()
	arr := make([]string, gridMaxStickies+1)
	for i := range arr {
		arr[i] = "x"
	}
	js, _ := json.Marshal(arr)
	_, err := loadGridContents(createGridFlags{contentsJSON: string(js)})
	if err == nil {
		t.Errorf("over-cap (%d > %d) should error", len(arr), gridMaxStickies)
	}
}

// ----- run ------------------------------------------------------------------

func TestRunCreateGridPostsBulkArray(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   []bulkItem
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"data":[{"id":"s1"},{"id":"s2"},{"id":"s3"}]}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCreateGrid(context.Background(), g, createGridFlags{
		boardID:      "uXjV1",
		contentsJSON: `["a","b","c"]`,
		columns:      3,
		color:        "yellow",
	})
	if err != nil {
		t.Fatalf("runCreateGrid: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/items/bulk" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/items/bulk", gotPath)
	}
	if len(gotBody) != 3 {
		t.Fatalf("body len = %d, want 3", len(gotBody))
	}
	if gotBody[0].Type != "sticky_note" || gotBody[0].Style == nil || gotBody[0].Style.FillColor != "light_yellow" {
		t.Errorf("first item: type=%q style=%+v", gotBody[0].Type, gotBody[0].Style)
	}
	var resp gridResult
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if resp.Created != 3 || resp.Rows != 1 || resp.Columns != 3 {
		t.Errorf("envelope = %+v, want created=3 rows=1 columns=3", resp)
	}
}

func TestRunCreateGridBatchesAt20(t *testing.T) {
	t.Parallel()
	// 25 items should fan out into 2 requests: 20 then 5.
	var (
		mu      sync.Mutex
		batches [][]bulkItem
		callIdx = 0
		respID  = func(i int) string { return "id-" + string(rune('A'+i)) }
		bodies  = make([]string, 0, 2)
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body []bulkItem
		_ = json.NewDecoder(r.Body).Decode(&body)
		batches = append(batches, body)

		// Build a 'data' array sized to match the batch so the response shape
		// matches Miro's behavior under partial-success.
		ids := make([]map[string]string, len(body))
		for i := range body {
			ids[i] = map[string]string{"id": respID(callIdx*20 + i)}
		}
		raw, _ := json.Marshal(map[string]any{"data": ids})
		bodies = append(bodies, string(raw))
		_, _ = w.Write(raw)
		callIdx++
	}))
	defer srv.Close()

	arr := make([]string, 25)
	for i := range arr {
		arr[i] = "s"
	}
	js, _ := json.Marshal(arr)

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCreateGrid(context.Background(), g, createGridFlags{
		boardID:      "b",
		contentsJSON: string(js),
		columns:      5,
	})
	if err != nil {
		t.Fatalf("runCreateGrid: %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2 (20+5)", len(batches))
	}
	if len(batches[0]) != 20 || len(batches[1]) != 5 {
		t.Errorf("batch sizes = (%d,%d), want (20,5)", len(batches[0]), len(batches[1]))
	}
	var resp gridResult
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if resp.Created != 25 || resp.Columns != 5 || resp.Rows != 5 {
		t.Errorf("envelope = %+v, want created=25 columns=5 rows=5", resp)
	}
}

func TestRunCreateGridRequiresBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateGrid(context.Background(), g, createGridFlags{contentsJSON: `["a"]`}); err == nil {
		t.Fatal("missing --board-id should error")
	}
}

func TestRunCreateGridRequiresContent(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateGrid(context.Background(), g, createGridFlags{boardID: "b"}); err == nil {
		t.Fatal("missing contents should error")
	}
}

func TestRunCreateGridDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCreateGrid(context.Background(), g, createGridFlags{boardID: "b", contentsJSON: `["a","b"]`}); err != nil {
		t.Fatalf("runCreateGrid: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/items/bulk") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersCreateGrid(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	for _, sub := range cmd.Commands() {
		if sub.Name() == "create-grid" {
			return
		}
	}
	t.Error("`stickies` parent missing create-grid subcommand")
}

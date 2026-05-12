package boards

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ----- scanItems (pure projection) ------------------------------------------

func TestScanItemsFindsContentMatch(t *testing.T) {
	rawItems := []map[string]any{
		{"id": "1", "type": "sticky_note", "data": map[string]any{"content": "Sprint planning notes"}},
		{"id": "2", "type": "shape", "data": map[string]any{"content": "Architecture diagram"}},
	}
	matches := scanItems(rawItems, "sprint")
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1: %+v", len(matches), matches)
	}
	if matches[0].ID != "1" {
		t.Errorf("matched id = %q, want 1", matches[0].ID)
	}
	if matches[0].Type != "sticky_note" {
		t.Errorf("matched type = %q", matches[0].Type)
	}
}

func TestScanItemsFallsBackToTitle(t *testing.T) {
	rawItems := []map[string]any{
		{"id": "card-1", "type": "card", "data": map[string]any{"title": "Sprint goals"}},
	}
	matches := scanItems(rawItems, "sprint")
	if len(matches) != 1 {
		t.Fatalf("title fallback failed: %+v", matches)
	}
	if matches[0].Content != "Sprint goals" {
		t.Errorf("content = %q, want 'Sprint goals' (from title)", matches[0].Content)
	}
}

func TestScanItemsCaseInsensitive(t *testing.T) {
	rawItems := []map[string]any{
		{"id": "1", "data": map[string]any{"content": "SPRINT NOTES"}},
	}
	if len(scanItems(rawItems, "sprint")) != 1 {
		t.Error("scanItems didn't match case-insensitively")
	}
}

func TestScanItemsSkipsItemsWithoutContent(t *testing.T) {
	rawItems := []map[string]any{
		{"id": "1", "type": "connector"}, // no data field
		{"id": "2", "type": "frame", "data": map[string]any{}},
		{"id": "3", "data": map[string]any{"content": "yes sprint here"}},
	}
	matches := scanItems(rawItems, "sprint")
	if len(matches) != 1 || matches[0].ID != "3" {
		t.Errorf("expected only id=3 to match, got: %+v", matches)
	}
}

func TestScanItemsExtractsPosition(t *testing.T) {
	rawItems := []map[string]any{
		{
			"id":       "1",
			"data":     map[string]any{"content": "find me"},
			"position": map[string]any{"x": 100.5, "y": 200.0},
		},
	}
	matches := scanItems(rawItems, "find")
	if len(matches) != 1 {
		t.Fatalf("expected match")
	}
	if matches[0].X != 100.5 || matches[0].Y != 200.0 {
		t.Errorf("position = (%v, %v), want (100.5, 200)", matches[0].X, matches[0].Y)
	}
}

func TestScanItemsTolerantOfBrokenPosition(t *testing.T) {
	// Defensive: API might return position as null or a string. Don't panic.
	rawItems := []map[string]any{
		{"id": "1", "data": map[string]any{"content": "find me"}, "position": "weird"},
		{"id": "2", "data": map[string]any{"content": "find me"}, "position": nil},
	}
	matches := scanItems(rawItems, "find")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	for _, m := range matches {
		if m.X != 0 || m.Y != 0 {
			t.Errorf("broken position should produce zero coords, got (%v, %v)", m.X, m.Y)
		}
	}
}

// ----- makeSnippet ----------------------------------------------------------

func TestMakeSnippet(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		window  int
		want    string
	}{
		{
			name:    "match in middle",
			content: "the quick brown fox jumps over the lazy dog",
			query:   "fox",
			window:  10,
			// "fox" is at index 16. start=6, end=29. content[6:29]
			// is "ick brown fox jumps ove". Both sides get ellipsed.
			want: "...ick brown fox jumps ove...",
		},
		{
			name:    "match at start (no leading ellipsis)",
			content: "fox runs fast",
			query:   "fox",
			window:  10,
			want:    "fox runs fast",
		},
		{
			name:    "no match returns full content",
			content: "no match here",
			query:   "xyz",
			window:  10,
			want:    "no match here",
		},
		{
			name:    "preserves original case in content",
			content: "Sprint Planning",
			query:   "sprint",
			window:  20,
			want:    "Sprint Planning",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := makeSnippet(tc.content, tc.query, tc.window)
			if got != tc.want {
				t.Errorf("makeSnippet(%q, %q, %d) = %q, want %q",
					tc.content, tc.query, tc.window, got, tc.want)
			}
		})
	}
}

// ----- runSearch end-to-end -------------------------------------------------

func TestRunSearchHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/boards/abc/items" {
			t.Errorf("server saw path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"1","type":"sticky_note","data":{"content":"sprint planning"}},
				{"id":"2","type":"shape","data":{"content":"architecture"}},
				{"id":"3","type":"sticky_note","data":{"content":"sprint retro"}}
			]
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runSearch(context.Background(), g, "abc", "sprint", "", 50); err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	var out searchResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Total != 2 {
		t.Errorf("total = %d, want 2", out.Total)
	}
	if out.BoardID != "abc" || out.Query != "sprint" {
		t.Errorf("envelope wrong: %+v", out)
	}
}

func TestRunSearchEmptyQueryIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runSearch(context.Background(), g, "abc", "   ", "", 50); err == nil {
		t.Fatal("runSearch with blank query returned nil, want error")
	}
}

func TestRunSearchEmptyBoardIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runSearch(context.Background(), g, "", "sprint", "", 50); err == nil {
		t.Fatal("runSearch with empty board_id returned nil, want error")
	}
}

func TestRunSearchAppliesTypeFilter(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runSearch(context.Background(), g, "abc", "x", "sticky_note", 25); err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	if !strings.Contains(gotQuery, "type=sticky_note") {
		t.Errorf("server query %q missing type filter", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=25") {
		t.Errorf("server query %q missing limit", gotQuery)
	}
}

func TestRunSearchDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runSearch(context.Background(), g, "abc", "alpha", "", 0); err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/abc/items") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunSearchZeroLimitFallsBackToDefault(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runSearch(context.Background(), g, "abc", "x", "", 0); err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	if !strings.Contains(gotQuery, "limit=50") {
		t.Errorf("expected limit=50 default, got query %q", gotQuery)
	}
}

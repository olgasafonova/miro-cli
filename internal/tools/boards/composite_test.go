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

// ----- countItemsByType (pure) ----------------------------------------------

func TestCountItemsByType(t *testing.T) {
	in := []map[string]any{
		{"type": "sticky_note"},
		{"type": "shape"},
		{"type": "sticky_note"},
		{"type": "frame"},
		{"type": "sticky_note"},
	}
	got := countItemsByType(in)
	if got["sticky_note"] != 3 {
		t.Errorf("sticky_note count = %d, want 3", got["sticky_note"])
	}
	if got["shape"] != 1 {
		t.Errorf("shape count = %d, want 1", got["shape"])
	}
	if got["frame"] != 1 {
		t.Errorf("frame count = %d, want 1", got["frame"])
	}
}

func TestCountItemsByTypeTolerantOfMissingType(t *testing.T) {
	in := []map[string]any{
		{"id": "1"}, // no type
		{"type": "sticky_note"},
	}
	got := countItemsByType(in)
	if got[""] != 1 {
		t.Errorf("untyped items should bucket into \"\", got %d", got[""])
	}
	if got["sticky_note"] != 1 {
		t.Errorf("sticky_note count = %d, want 1", got["sticky_note"])
	}
}

// ----- buildFrameSummaries (pure) -------------------------------------------

func TestBuildFrameSummariesGroupsChildren(t *testing.T) {
	in := []map[string]any{
		{"id": "frame-a", "type": "frame", "data": map[string]any{"title": "Backlog"}},
		{"id": "frame-b", "type": "frame", "data": map[string]any{"title": "Done"}},
		{"id": "child-1", "type": "sticky_note", "parent": map[string]any{"id": "frame-a"}},
		{"id": "child-2", "type": "shape", "parent": map[string]any{"id": "frame-a"}},
		{"id": "child-3", "type": "sticky_note", "parent": map[string]any{"id": "frame-b"}},
		{"id": "loose-1", "type": "sticky_note"}, // top-level, no parent
	}
	got := buildFrameSummaries(in)
	if len(got) != 2 {
		t.Fatalf("got %d frames, want 2: %+v", len(got), got)
	}

	byID := map[string]frameSummary{got[0].ID: got[0], got[1].ID: got[1]}
	a := byID["frame-a"]
	if a.Title != "Backlog" {
		t.Errorf("frame-a title = %q, want Backlog", a.Title)
	}
	if len(a.ItemIDs) != 2 || a.ItemIDs[0] != "child-1" || a.ItemIDs[1] != "child-2" {
		t.Errorf("frame-a item_ids = %v, want [child-1 child-2]", a.ItemIDs)
	}
	b := byID["frame-b"]
	if len(b.ItemIDs) != 1 || b.ItemIDs[0] != "child-3" {
		t.Errorf("frame-b item_ids = %v, want [child-3]", b.ItemIDs)
	}
}

func TestBuildFrameSummariesTolerantOfBrokenInput(t *testing.T) {
	in := []map[string]any{
		{"id": "frame-a", "type": "frame"},                  // no data
		{"id": "frame-b", "type": "frame", "data": "wrong"}, // data wrong type
		{"id": "lonely-frame", "type": "frame"},
		{"id": "x", "type": "sticky_note", "parent": "wrong-type"},                        // parent wrong type
		{"id": "y", "type": "sticky_note", "parent": map[string]any{"id": "ghost-frame"}}, // dangling
	}
	got := buildFrameSummaries(in)
	if len(got) != 3 {
		t.Errorf("got %d frames, want 3 (broken-data shouldn't drop frames): %+v", len(got), got)
	}
	for _, fs := range got {
		if len(fs.ItemIDs) != 0 {
			t.Errorf("frame %s should have no children (all parents are broken/dangling), got %v", fs.ID, fs.ItemIDs)
		}
	}
}

// ----- summary verb end-to-end ----------------------------------------------

func TestRunSummaryHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/boards/abc":
			_, _ = w.Write([]byte(`{"id":"abc","name":"Sprint"}`))
		case "/v2/boards/abc/items":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"1","type":"sticky_note"},
				{"id":"2","type":"sticky_note"},
				{"id":"3","type":"shape"}
			]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runSummary(context.Background(), g, "abc", 5000); err != nil {
		t.Fatalf("runSummary: %v", err)
	}
	var out summaryResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.TotalItems != 3 {
		t.Errorf("total_items = %d, want 3", out.TotalItems)
	}
	if out.CountsByType["sticky_note"] != 2 || out.CountsByType["shape"] != 1 {
		t.Errorf("counts wrong: %+v", out.CountsByType)
	}
	if out.Board["id"] != "abc" {
		t.Errorf("board id missing: %+v", out.Board)
	}
}

func TestRunSummaryEmptyBoardIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runSummary(context.Background(), g, "", 5000); err == nil {
		t.Fatal("runSummary with empty board_id returned nil, want error")
	}
}

func TestRunSummaryDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runSummary(context.Background(), g, "abc", 5000); err != nil {
		t.Fatalf("runSummary: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- content verb end-to-end ----------------------------------------------

func TestRunContentEmitsItemsAndFrames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/boards/abc":
			_, _ = w.Write([]byte(`{"id":"abc","name":"Sprint"}`))
		case "/v2/boards/abc/items":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"f-1","type":"frame","data":{"title":"Backlog"}},
				{"id":"s-1","type":"sticky_note","parent":{"id":"f-1"}},
				{"id":"s-2","type":"sticky_note"}
			]}`))
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runContent(context.Background(), g, "abc", 5000); err != nil {
		t.Fatalf("runContent: %v", err)
	}
	var out contentResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Items) != 3 {
		t.Errorf("items count = %d, want 3", len(out.Items))
	}
	if len(out.Frames) != 1 {
		t.Errorf("frames count = %d, want 1", len(out.Frames))
	}
	if len(out.Frames) > 0 {
		if out.Frames[0].Title != "Backlog" {
			t.Errorf("frame title = %q, want Backlog", out.Frames[0].Title)
		}
		if len(out.Frames[0].ItemIDs) != 1 || out.Frames[0].ItemIDs[0] != "s-1" {
			t.Errorf("frame children = %v, want [s-1]", out.Frames[0].ItemIDs)
		}
	}
	if out.CountsByType["sticky_note"] != 2 || out.CountsByType["frame"] != 1 {
		t.Errorf("counts wrong: %+v", out.CountsByType)
	}
}

func TestRunContentEmptyBoardIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runContent(context.Background(), g, "", 5000); err == nil {
		t.Fatal("runContent with empty board_id returned nil, want error")
	}
}

// ----- audit verb -----------------------------------------------------------

func TestBuildAuditPath(t *testing.T) {
	tests := []struct {
		name string
		in   auditFlags
		want string
	}{
		{
			name: "minimal",
			in:   auditFlags{},
			want: "/v2/audit/logs",
		},
		{
			name: "all params",
			in: auditFlags{
				createdAfter:  "2026-05-01T00:00:00Z",
				createdBefore: "2026-05-12T00:00:00Z",
				limit:         100,
				cursor:        "c-1",
			},
			// url.Values.Encode sorts keys alphabetically.
			want: "/v2/audit/logs?createdAfter=2026-05-01T00%3A00%3A00Z&createdBefore=2026-05-12T00%3A00%3A00Z&cursor=c-1&limit=100",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildAuditPath(tc.in); got != tc.want {
				t.Errorf("buildAuditPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAuditTimestampsRejectsBadRFC3339(t *testing.T) {
	cases := []auditFlags{
		{createdAfter: "yesterday"},
		{createdBefore: "2026/05/01"},
		{createdAfter: "2026-05-01"}, // missing time portion
	}
	for _, af := range cases {
		if err := validateAuditTimestamps(af); err == nil {
			t.Errorf("validateAuditTimestamps(%+v) returned nil, want error", af)
		}
	}
}

func TestValidateAuditTimestampsAcceptsEmpty(t *testing.T) {
	if err := validateAuditTimestamps(auditFlags{}); err != nil {
		t.Errorf("empty timestamps rejected: %v", err)
	}
}

func TestRunAuditHappyPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"data":[{"event":"board.deleted"}],"cursor":""}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runAudit(context.Background(), g, auditFlags{limit: 50}); err != nil {
		t.Fatalf("runAudit: %v", err)
	}
	if gotPath != "/v2/audit/logs?limit=50" {
		t.Errorf("server saw path %q", gotPath)
	}
	if !strings.Contains(stdout.String(), "board.deleted") {
		t.Errorf("stdout missing event: %q", stdout.String())
	}
}

func TestRunAuditRejectsBadTimestampBeforeHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("bad timestamp shouldn't hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runAudit(context.Background(), g, auditFlags{createdAfter: "yesterday"}); err == nil {
		t.Fatal("runAudit with bad timestamp returned nil, want error")
	}
}

func TestRunAuditDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runAudit(context.Background(), g, auditFlags{limit: 10}); err != nil {
		t.Fatalf("runAudit: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/audit/logs?limit=10") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

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

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// ----- match resolution (pure function) -------------------------------------

func TestResolveFindMatchExactWinsOverPrefix(t *testing.T) {
	boards := []map[string]any{
		{"id": "1", "name": "Sprint Planning Q1"},
		{"id": "2", "name": "Sprint"}, // exact match for query "sprint"
		{"id": "3", "name": "Sprint Retro"},
	}
	got, kind := resolveFindMatch(boards, "sprint")
	if kind != "exact" {
		t.Errorf("kind = %q, want exact", kind)
	}
	if got["id"] != "2" {
		t.Errorf("got id %v, want 2 (exact match)", got["id"])
	}
}

func TestResolveFindMatchPrefixBeatsContains(t *testing.T) {
	boards := []map[string]any{
		{"id": "1", "name": "Bug-fix sprint"},  // contains
		{"id": "2", "name": "Sprint Planning"}, // prefix
		{"id": "3", "name": "Old Sprint"},      // contains
	}
	got, kind := resolveFindMatch(boards, "sprint")
	if kind != "prefix" {
		t.Errorf("kind = %q, want prefix", kind)
	}
	if got["id"] != "2" {
		t.Errorf("got id %v, want 2 (prefix winner)", got["id"])
	}
}

func TestResolveFindMatchContains(t *testing.T) {
	boards := []map[string]any{
		{"id": "1", "name": "Quarterly Roadmap"},
		{"id": "2", "name": "Marketing Backlog"},
	}
	got, kind := resolveFindMatch(boards, "back")
	if kind != "contains" {
		t.Errorf("kind = %q, want contains", kind)
	}
	if got["id"] != "2" {
		t.Errorf("got id %v, want 2 (contains)", got["id"])
	}
}

func TestResolveFindMatchFallback(t *testing.T) {
	boards := []map[string]any{
		{"id": "1", "name": "Alpha"},
		{"id": "2", "name": "Beta"},
	}
	got, kind := resolveFindMatch(boards, "gamma")
	if kind != "fallback" {
		t.Errorf("kind = %q, want fallback", kind)
	}
	if got["id"] != "1" {
		t.Errorf("got id %v, want 1 (first as fallback)", got["id"])
	}
}

func TestResolveFindMatchCaseInsensitive(t *testing.T) {
	boards := []map[string]any{
		{"id": "1", "name": "SPRINT PLANNING"},
	}
	_, kind := resolveFindMatch(boards, "sprint")
	if kind != "prefix" {
		t.Errorf("kind = %q, want prefix (case-insensitive match)", kind)
	}
}

func TestResolveFindMatchMissingNameKey(t *testing.T) {
	boards := []map[string]any{
		{"id": "1"}, // no name field
		{"id": "2", "name": "Sprint"},
	}
	got, kind := resolveFindMatch(boards, "sprint")
	if kind != "exact" {
		t.Errorf("kind = %q, want exact (missing-name entries shouldn't blow up)", kind)
	}
	if got["id"] != "2" {
		t.Errorf("got id %v, want 2", got["id"])
	}
}

// ----- runFind end-to-end ---------------------------------------------------

func TestRunFindHappyPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "a", "name": "Sprint Planning"},
				{"id": "b", "name": "Sprint"}
			],
			"total": 2
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}

	if err := runFind(context.Background(), g, "Sprint"); err != nil {
		t.Fatalf("runFind: %v", err)
	}
	if gotPath != "/v2/boards?limit=20&query=Sprint" {
		t.Errorf("server saw path %q, want /v2/boards?limit=20&query=Sprint", gotPath)
	}

	var out findResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if out.Match.Kind != "exact" {
		t.Errorf("match.kind = %q, want exact", out.Match.Kind)
	}
	if out.Board["id"] != "b" {
		t.Errorf("emitted board id = %v, want b", out.Board["id"])
	}
	if out.NumPeers != 1 {
		t.Errorf("num_peers = %d, want 1", out.NumPeers)
	}
}

func TestRunFindEmptyResultIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": [], "total": 0}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runFind(context.Background(), g, "ghost")
	if err == nil {
		t.Fatal("runFind with no matches returned nil, want error")
	}
	if !strings.Contains(err.Error(), "no board found") {
		t.Errorf("error %q didn't say no board found", err.Error())
	}
}

func TestRunFindBlankQueryIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runFind(context.Background(), g, "  "); err == nil {
		t.Fatal("runFind with whitespace-only query returned nil, want error")
	}
}

func TestRunFindDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runFind(context.Background(), g, "alpha"); err != nil {
		t.Fatalf("runFind: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards?limit=20&query=alpha") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

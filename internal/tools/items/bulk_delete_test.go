package items

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

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func TestSplitTrim(t *testing.T) {
	t.Parallel()
	got := splitTrim(" a, b ,c,,  ,d")
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadIDsExclusivity(t *testing.T) {
	t.Parallel()
	if _, err := loadIDs(bulkDeleteFlags{}); err == nil {
		t.Error("loadIDs with no flags returned nil")
	}
	if _, err := loadIDs(bulkDeleteFlags{ids: "a,b", idsJSON: "[]"}); err == nil {
		t.Error("loadIDs with two flags returned nil")
	}
}

func TestLoadIDsCommaSeparated(t *testing.T) {
	t.Parallel()
	got, err := loadIDs(bulkDeleteFlags{ids: "a,b,c"})
	if err != nil {
		t.Fatalf("loadIDs: %v", err)
	}
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("got = %v", got)
	}
}

func TestLoadIDsJSON(t *testing.T) {
	t.Parallel()
	got, err := loadIDs(bulkDeleteFlags{idsJSON: `["x","y"]`})
	if err != nil {
		t.Fatalf("loadIDs: %v", err)
	}
	if len(got) != 2 || got[0] != "x" {
		t.Errorf("got = %v", got)
	}
}

func TestLoadIDsFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ids.json")
	if err := os.WriteFile(path, []byte(`["k1","k2"]`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := loadIDs(bulkDeleteFlags{idsFile: path})
	if err != nil {
		t.Fatalf("loadIDs: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got = %v", got)
	}
}

func TestLoadIDsRejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := loadIDs(bulkDeleteFlags{idsJSON: `[]`}); err == nil {
		t.Error("loadIDs with empty array returned nil")
	}
	if _, err := loadIDs(bulkDeleteFlags{ids: " , , "}); err == nil {
		t.Error("loadIDs with whitespace-only ids returned nil")
	}
}

func TestRunBulkDeleteRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runBulkDelete(context.Background(), g, bulkDeleteFlags{boardID: "b1", ids: "a,b"})
	if err == nil {
		t.Fatal("runBulkDelete without --yes returned nil")
	}
	if !strings.Contains(err.Error(), "without --yes") {
		t.Errorf("error %q does not mention --yes", err)
	}
}

func TestRunBulkDeleteDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API")
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		DryRun: true,
	}
	err := runBulkDelete(context.Background(), g, bulkDeleteFlags{boardID: "b1", ids: "a,b,c"})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b1/items/{item_id} x 3") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunBulkDeleteHappyPath(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("server saw %s, want DELETE", r.Method)
		}
		mu.Lock()
		seen = append(seen, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Yes:    true,
	}
	err := runBulkDelete(context.Background(), g, bulkDeleteFlags{boardID: "b1", ids: "a,b,c"})
	if err != nil {
		t.Fatalf("runBulkDelete: %v", err)
	}

	if len(seen) != 3 {
		t.Errorf("server saw %d requests, want 3", len(seen))
	}
	for i, want := range []string{"/v2/boards/b1/items/a", "/v2/boards/b1/items/b", "/v2/boards/b1/items/c"} {
		if seen[i] != want {
			t.Errorf("seen[%d] = %q, want %q", i, seen[i], want)
		}
	}

	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}
	if out.Requested != 3 || out.Succeeded != 3 || out.Failed != 0 {
		t.Errorf("summary = %+v, want requested=3 succeeded=3 failed=0", out)
	}
}

func TestRunBulkDeletePartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One specific ID 404s; others succeed.
		if strings.HasSuffix(r.URL.Path, "/missing") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Yes:    true,
	}
	err := runBulkDelete(context.Background(), g, bulkDeleteFlags{boardID: "b1", ids: "ok-1,missing,ok-2"})
	if err != nil {
		t.Fatalf("runBulkDelete: %v", err)
	}
	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Requested != 3 || out.Succeeded != 2 || out.Failed != 1 {
		t.Errorf("summary = %+v, want requested=3 succeeded=2 failed=1", out)
	}
	if len(out.Results) != 3 || out.Results[1].Status != "error" || out.Results[1].ID != "missing" {
		t.Errorf("results = %+v", out.Results)
	}
	if out.Results[0].Status != "success" || out.Results[2].Status != "success" {
		t.Errorf("expected sibling successes, got %+v", out.Results)
	}
}

// TestRunBulkDeleteUnsafeIDIsPerItemError proves a path-traversal item ID
// is rejected before any HTTP request is built for it, while sibling
// deletes still go through. Companion to the board_id ValidateID gate;
// per-item IDs are spliced into the URL path the same way.
func TestRunBulkDeleteUnsafeIDIsPerItemError(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Yes:    true,
	}
	err := runBulkDelete(context.Background(), g, bulkDeleteFlags{boardID: "b1", ids: "ok-1,../../evil,ok-2"})
	if err != nil {
		t.Fatalf("runBulkDelete: %v", err)
	}
	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Requested != 3 || out.Succeeded != 2 || out.Failed != 1 {
		t.Errorf("summary = %+v, want requested=3 succeeded=2 failed=1", out)
	}
	if out.Results[1].Status != "error" || !strings.Contains(out.Results[1].Error, "ids[1]:") {
		t.Errorf("results[1] = %+v", out.Results[1])
	}
	for _, p := range seen {
		if strings.Contains(p, "evil") {
			t.Errorf("unsafe ID reached the server: %s", p)
		}
	}
}

// TestRunBulkDeleteConcurrentPreservesOrder drives the real fan-out path
// (Concurrency=8) through the actual client + an httptest server and
// asserts the order-sensitive envelope still maps Results[i] back to the
// i-th input ID. The injected client has no rate limiter, so the workers
// genuinely overlap; -race guards the disjoint-index writes in FanOut.
func TestRunBulkDeleteConcurrentPreservesOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/gone") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Deliberately unsorted, with the failing ID mid-stream, so a pool
	// that reordered completions would scramble the positional mapping.
	ids := []string{"z9", "a1", "m4", "gone", "q7", "b2", "x0", "c3", "n5", "d6"}

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout:      &stdout,
		Client:      miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Yes:         true,
		Concurrency: 8,
	}
	if err := runBulkDelete(context.Background(), g, bulkDeleteFlags{boardID: "b1", ids: strings.Join(ids, ",")}); err != nil {
		t.Fatalf("runBulkDelete: %v", err)
	}
	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertSummary(t, out, bulkOpResponse{Requested: len(ids), Succeeded: len(ids) - 1, Failed: 1})
	assertOrderedResults(t, out.Results, ids, "gone")
}

// assertSummary checks the aggregate counts in one place so the calling
// test stays free of a multi-branch conditional. Only the count fields
// of want are consulted.
func assertSummary(t *testing.T, got, want bulkOpResponse) {
	t.Helper()
	if got.Requested != want.Requested {
		t.Errorf("requested=%d, want %d", got.Requested, want.Requested)
	}
	if got.Succeeded != want.Succeeded {
		t.Errorf("succeeded=%d, want %d", got.Succeeded, want.Succeeded)
	}
	if got.Failed != want.Failed {
		t.Errorf("failed=%d, want %d", got.Failed, want.Failed)
	}
}

// assertOrderedResults verifies Results[i] maps back to wantIDs[i] (the
// order-preservation guarantee) and that the one ID equal to failID is
// the only error. Carrying the loop here keeps the test body flat.
func assertOrderedResults(t *testing.T, results []bulkOpResult, wantIDs []string, failID string) {
	t.Helper()
	if len(results) != len(wantIDs) {
		t.Fatalf("len(results)=%d, want %d", len(results), len(wantIDs))
	}
	for i, id := range wantIDs {
		if results[i].ID != id {
			t.Fatalf("results[%d].ID=%q, want %q (order not preserved under fan-out)", i, results[i].ID, id)
		}
		wantStatus := statusFor(id, failID)
		if results[i].Status != wantStatus {
			t.Errorf("results[%d] (%s) status=%q, want %q", i, id, results[i].Status, wantStatus)
		}
	}
}

func statusFor(id, failID string) string {
	if id == failID {
		return "error"
	}
	return "success"
}

func TestRunBulkDeleteRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runBulkDelete(context.Background(), g, bulkDeleteFlags{ids: "a"}); err == nil {
		t.Fatal("runBulkDelete with empty board ID returned nil")
	}
}

func TestNewBulkDeleteCmdRegistered(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{"bulk-delete": false, "bulk-update": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("items parent missing subcommand %q", verb)
		}
	}
}

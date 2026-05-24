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

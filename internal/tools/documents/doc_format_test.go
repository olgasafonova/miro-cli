package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// newDocTestGlobals wires a Globals at the given httptest server with a
// stdout buffer for output assertions. --yes is set because the doc
// verbs under test include destructive ones.
func newDocTestGlobals(srvURL string) (*clictx.Globals, *bytes.Buffer) {
	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srvURL}),
		Yes:    true,
	}
	return g, &stdout
}

// wireDoc is the GET /docs/{id} fixture.
func wireDoc(id, content string, x, y float64) map[string]any {
	return map[string]any{
		"id":       id,
		"data":     map[string]any{"contentType": "markdown", "content": content},
		"position": map[string]any{"x": x, "y": y},
	}
}

func TestRunGetDocHitsDocsPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewEncoder(w).Encode(wireDoc("doc-1", "# Hello", 10, 20))
	}))
	defer srv.Close()

	g, stdout := newDocTestGlobals(srv.URL)
	if err := runGetDoc(context.Background(), g, "abc", "doc-1"); err != nil {
		t.Fatalf("runGetDoc: %v", err)
	}
	if want := "GET /v2/boards/abc/docs/doc-1"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if docContent(out) != "# Hello" {
		t.Errorf("emitted content = %q, want '# Hello'", docContent(out))
	}
}

func TestRunDeleteDocRefusesWithoutYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDeleteDoc(context.Background(), g, "abc", "doc-1")
	var cfgErr *miro.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *miro.ConfigError, got %T: %v", err, err)
	}
}

func TestRunDeleteDocWithYes(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	g, stdout := newDocTestGlobals(srv.URL)
	if err := runDeleteDoc(context.Background(), g, "abc", "doc-1"); err != nil {
		t.Fatalf("runDeleteDoc: %v", err)
	}
	if want := "DELETE /v2/boards/abc/docs/doc-1"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	var out deleteResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if !out.Deleted || out.ID != "doc-1" {
		t.Errorf("envelope = %+v, want deleted doc-1", out)
	}
}

// docUpdateServer answers the three-step update flow (GET doc, DELETE
// item, POST docs) and records the requests.
func docUpdateServer(t *testing.T, content string, failCreate bool) (*httptest.Server, *[]string, *map[string]any) {
	t.Helper()
	var calls []string
	var createBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(wireDoc("doc-1", content, 50, 60))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			if failCreate {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"boom"}`))
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&createBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "doc-2"})
		}
	}))
	return srv, &calls, &createBody
}

func TestRunUpdateDocFullReplace(t *testing.T) {
	srv, calls, createBody := docUpdateServer(t, "old text", false)
	defer srv.Close()

	g, stdout := newDocTestGlobals(srv.URL)
	err := runUpdateDoc(context.Background(), g, updateDocFlags{
		boardID: "abc", itemID: "doc-1", content: "# New",
	})
	if err != nil {
		t.Fatalf("runUpdateDoc: %v", err)
	}
	want := []string{
		"GET /v2/boards/abc/docs/doc-1",
		"DELETE /v2/boards/abc/items/doc-1",
		"POST /v2/boards/abc/docs",
	}
	if len(*calls) != 3 || (*calls)[0] != want[0] || (*calls)[1] != want[1] || (*calls)[2] != want[2] {
		t.Errorf("call sequence = %v, want %v", *calls, want)
	}
	data, _ := (*createBody)["data"].(map[string]any)
	if data["content"] != "# New" || data["contentType"] != "markdown" {
		t.Errorf("recreate data = %v", data)
	}
	position, _ := (*createBody)["position"].(map[string]any)
	if position["x"] != 50.0 || position["y"] != 60.0 {
		t.Errorf("recreate position = %v, want the original (50,60)", position)
	}
	var out updateDocResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.ID != "doc-2" || out.OldID != "doc-1" || out.Replaced != 0 {
		t.Errorf("envelope = %+v, want doc-2/doc-1/0", out)
	}
}

func TestRunUpdateDocFindReplaceAll(t *testing.T) {
	srv, _, createBody := docUpdateServer(t, "foo bar foo", false)
	defer srv.Close()

	g, stdout := newDocTestGlobals(srv.URL)
	err := runUpdateDoc(context.Background(), g, updateDocFlags{
		boardID: "abc", itemID: "doc-1", oldContent: "foo", newContent: "baz", replaceAll: true,
	})
	if err != nil {
		t.Fatalf("runUpdateDoc: %v", err)
	}
	data, _ := (*createBody)["data"].(map[string]any)
	if data["content"] != "baz bar baz" {
		t.Errorf("recreated content = %q, want 'baz bar baz'", data["content"])
	}
	var out updateDocResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Replaced != 2 {
		t.Errorf("replaced = %d, want 2", out.Replaced)
	}
}

func TestRunUpdateDocOldContentMissingAborts(t *testing.T) {
	srv, calls, _ := docUpdateServer(t, "actual text", false)
	defer srv.Close()

	g, _ := newDocTestGlobals(srv.URL)
	err := runUpdateDoc(context.Background(), g, updateDocFlags{
		boardID: "abc", itemID: "doc-1", oldContent: "absent needle",
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing needle: err = %v, want not-found error", err)
	}
	// The doc must survive: only the read happened, no delete.
	if len(*calls) != 1 {
		t.Errorf("calls = %v, want the GET only", *calls)
	}
}

func TestRunUpdateDocRecreateFailureEmitsRecovery(t *testing.T) {
	srv, _, _ := docUpdateServer(t, "precious content", true)
	defer srv.Close()

	g, stdout := newDocTestGlobals(srv.URL)
	err := runUpdateDoc(context.Background(), g, updateDocFlags{
		boardID: "abc", itemID: "doc-1", content: "# Replacement",
	})
	if err == nil {
		t.Fatal("expected error from failed recreate")
	}
	var out updateDocRecovery
	if jsonErr := json.Unmarshal(stdout.Bytes(), &out); jsonErr != nil {
		t.Fatalf("recovery envelope not emitted: %v\n%s", jsonErr, stdout.String())
	}
	if out.Content != "# Replacement" || out.OldID != "doc-1" {
		t.Errorf("recovery = %+v, want the resolved content and old id", out)
	}
}

func TestRunUpdateDocRefusesWithoutYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("update-doc without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runUpdateDoc(context.Background(), g, updateDocFlags{boardID: "abc", itemID: "doc-1", content: "x"})
	var cfgErr *miro.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *miro.ConfigError, got %T: %v", err, err)
	}
}

func TestRunUpdateDocValidation(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdateDoc(context.Background(), g, updateDocFlags{boardID: "abc", itemID: "doc-1"}); err == nil {
		t.Error("no content mode accepted")
	}
	if err := runUpdateDoc(context.Background(), g, updateDocFlags{itemID: "doc-1", content: "x"}); err == nil {
		t.Error("empty --board-id accepted")
	}
}

func TestDocVerbsDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	tests := []struct {
		name string
		run  func(g *clictx.Globals) error
		want string
	}{
		{
			name: "get-doc",
			run: func(g *clictx.Globals) error {
				return runGetDoc(context.Background(), g, "abc", "doc-1")
			},
			want: "DRY-RUN GET /v2/boards/abc/docs/doc-1",
		},
		{
			name: "update-doc",
			run: func(g *clictx.Globals) error {
				return runUpdateDoc(context.Background(), g, updateDocFlags{boardID: "abc", itemID: "doc-1", content: "x"})
			},
			want: "DRY-RUN GET+DELETE+POST /v2/boards/abc/docs/doc-1",
		},
		{
			name: "delete-doc",
			run: func(g *clictx.Globals) error {
				return runDeleteDoc(context.Background(), g, "abc", "doc-1")
			},
			want: "DRY-RUN DELETE /v2/boards/abc/docs/doc-1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			g := &clictx.Globals{Stdout: &stdout, Client: client, DryRun: true}
			if err := tc.run(g); err != nil {
				t.Fatalf("dry run: %v", err)
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Errorf("dry-run output %q, want substring %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestNewCmdRegistersDocFormatVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	got := map[string]bool{}
	for _, sub := range cmd.Commands() {
		got[sub.Name()] = true
	}
	for _, want := range []string{"create-doc", "get-doc", "update-doc", "delete-doc"} {
		if !got[want] {
			t.Errorf("documents parent did not register %q", want)
		}
	}
}

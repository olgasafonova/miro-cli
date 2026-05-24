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

func TestRunCopyHappyPath(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotQuery  string
		gotBody   createRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"copy-1","name":"Copy"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCopy(context.Background(), g, "src-1", createRequest{Name: "Copy"})
	if err != nil {
		t.Fatalf("runCopy: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("server saw method %q, want PUT", gotMethod)
	}
	if gotPath != "/v2/boards" {
		t.Errorf("server saw path %q, want /v2/boards", gotPath)
	}
	if gotQuery != "copy_from=src-1" {
		t.Errorf("server saw query %q, want copy_from=src-1", gotQuery)
	}
	if gotBody.Name != "Copy" {
		t.Errorf("server saw body %+v, want name=Copy", gotBody)
	}
}

func TestRunCopyEscapesBoardID(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	// Miro board IDs end in '=' (base64-ish); confirm it survives the
	// PUT path construction without re-encoding chaos.
	if err := runCopy(context.Background(), g, "uXjVG34x8Cg=", createRequest{}); err != nil {
		t.Fatalf("runCopy: %v", err)
	}
	// QueryEscape turns "=" into "%3D".
	if gotQuery != "copy_from=uXjVG34x8Cg%3D" {
		t.Errorf("server saw query %q, want copy_from=uXjVG34x8Cg%%3D", gotQuery)
	}
}

func TestRunCopyDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCopy(context.Background(), g, "src", createRequest{Name: "X"}); err != nil {
		t.Fatalf("runCopy: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PUT /v2/boards?copy_from=src") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunCopyEmptyBoardIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCopy(context.Background(), g, "", createRequest{}); err == nil {
		t.Fatal("runCopy with empty board_id returned nil, want error")
	}
}

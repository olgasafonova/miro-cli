package images

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
	"testing"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func TestRunUpdateFromFileHappyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.png")
	if err := os.WriteFile(path, []byte("newbytes"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	var (
		gotMethod string
		gotPath   string
		gotCT     string
		gotBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"id":"img-1","type":"image"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}

	err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{
		boardID: "b1",
		itemID:  "img-1",
		file:    path,
		title:   "Replacement",
	})
	if err != nil {
		t.Fatalf("runUpdateFromFile: %v", err)
	}
	if gotMethod != "PATCH" || gotPath != "/v2/boards/b1/images/img-1" {
		t.Errorf("server saw %s %s", gotMethod, gotPath)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Errorf("Content-Type = %q", gotCT)
	}

	parts := parseMultipartParts(t, bytes.NewReader(gotBody), gotCT)
	var data map[string]any
	if err := json.Unmarshal(parts["data"], &data); err != nil {
		t.Fatalf("decode data field: %v", err)
	}
	if data["title"] != "Replacement" {
		t.Errorf("data.title = %v, want Replacement", data["title"])
	}
	if string(parts["resource"]) != "newbytes" {
		t.Errorf("resource bytes = %q, want newbytes", parts["resource"])
	}
}

func TestRunUpdateFromFileRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t"})}
	if err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{itemID: "i", file: "/x.png"}); err == nil {
		t.Error("runUpdateFromFile with empty board returned nil")
	}
	if err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{boardID: "b", file: "/x.png"}); err == nil {
		t.Error("runUpdateFromFile with empty item returned nil")
	}
}

func TestRunUpdateFromFileDryRunSkipsHTTP(t *testing.T) {
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
	err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{
		boardID: "b1", itemID: "i1", file: "/no/such.png",
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PATCH /v2/boards/b1/images/i1") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

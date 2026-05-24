package documents

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

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func TestRunUpdateFromFileHappyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7 new"), 0o600); err != nil {
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
		_, _ = w.Write([]byte(`{"id":"doc-1","type":"document"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}

	err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{
		boardID: "b1",
		itemID:  "doc-1",
		file:    path,
		x:       42,
		y:       7,
	})
	if err != nil {
		t.Fatalf("runUpdateFromFile: %v", err)
	}
	if gotMethod != "PATCH" || gotPath != "/v2/boards/b1/documents/doc-1" {
		t.Errorf("server saw %s %s", gotMethod, gotPath)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Errorf("Content-Type = %q", gotCT)
	}

	parts := parseMultipartParts(t, bytes.NewReader(gotBody), gotCT)
	var data map[string]any
	if err := json.Unmarshal(parts["data"], &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	pos, ok := data["position"].(map[string]any)
	if !ok {
		t.Fatalf("position not a map: %T", data["position"])
	}
	if pos["x"].(float64) != 42 || pos["y"].(float64) != 7 {
		t.Errorf("position = %+v, want x=42 y=7", pos)
	}
	if !strings.HasPrefix(string(parts["resource"]), "%PDF-1.7") {
		t.Errorf("resource bytes did not pass through: %q", parts["resource"])
	}
}

func TestRunUpdateFromFileRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t"})}
	if err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{itemID: "i", file: "/x.pdf"}); err == nil {
		t.Error("runUpdateFromFile with empty board returned nil")
	}
	if err := runUpdateFromFile(context.Background(), g, updateFromFileFlags{boardID: "b", file: "/x.pdf"}); err == nil {
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
		boardID: "b1", itemID: "i1", file: "/no/file.pdf",
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PATCH /v2/boards/b1/documents/i1") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

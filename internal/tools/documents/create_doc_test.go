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

// ----- buildCreateDocRequest -----------------------------------------------

func TestBuildCreateDocRequestSetsContentType(t *testing.T) {
	t.Parallel()
	req := buildCreateDocRequest(createDocFlags{}, "# Hello\n\nbody")
	if req.Data.ContentType != "markdown" {
		t.Errorf("contentType = %q, want markdown", req.Data.ContentType)
	}
	if req.Data.Content != "# Hello\n\nbody" {
		t.Errorf("content = %q", req.Data.Content)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
	if req.Parent != nil {
		t.Errorf("parent should be nil when --parent-id unset: %+v", req.Parent)
	}
}

func TestBuildCreateDocRequestAppliesParentAndPosition(t *testing.T) {
	t.Parallel()
	req := buildCreateDocRequest(createDocFlags{x: 100, y: -50, parentID: "frame-1"}, "x")
	if req.Position == nil || req.Position.X != 100 || req.Position.Y != -50 {
		t.Errorf("position = %+v, want x=100 y=-50", req.Position)
	}
	if req.Parent == nil || req.Parent.ID != "frame-1" {
		t.Errorf("parent = %+v, want id=frame-1", req.Parent)
	}
}

// ----- loadDocContent -------------------------------------------------------

func TestLoadDocContentInline(t *testing.T) {
	t.Parallel()
	got, err := loadDocContent(createDocFlags{content: "hi"})
	if err != nil {
		t.Fatalf("loadDocContent: %v", err)
	}
	if got != "hi" {
		t.Errorf("content = %q", got)
	}
}

func TestLoadDocContentFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "body.md")
	if err := os.WriteFile(p, []byte("# heading\n\nparagraph"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	got, err := loadDocContent(createDocFlags{contentFile: p})
	if err != nil {
		t.Fatalf("loadDocContent: %v", err)
	}
	if !strings.HasPrefix(got, "# heading") {
		t.Errorf("content = %q", got)
	}
}

func TestLoadDocContentRejectsBothFlags(t *testing.T) {
	t.Parallel()
	if _, err := loadDocContent(createDocFlags{content: "a", contentFile: "p"}); err == nil {
		t.Error("both flags set should error")
	}
}

func TestLoadDocContentRejectsNeitherFlag(t *testing.T) {
	t.Parallel()
	if _, err := loadDocContent(createDocFlags{}); err == nil {
		t.Error("neither flag should error")
	}
}

func TestLoadDocContentRejectsEmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(p, []byte(""), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	if _, err := loadDocContent(createDocFlags{contentFile: p}); err == nil {
		t.Error("empty file should error")
	}
}

// ----- run ------------------------------------------------------------------

func TestRunCreateDocPostsToDocsEndpoint(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   createDocRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"doc-1","data":{"contentType":"markdown"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCreateDoc(context.Background(), g, createDocFlags{
		boardID: "uXjV1",
		content: "# Hello\n\nMarkdown body.",
	})
	if err != nil {
		t.Fatalf("runCreateDoc: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	// Distinct endpoint from /documents — must be /docs.
	if gotPath != "/v2/boards/uXjV1/docs" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/docs", gotPath)
	}
	if gotBody.Data.ContentType != "markdown" {
		t.Errorf("body data.contentType = %q, want markdown", gotBody.Data.ContentType)
	}
	if !strings.Contains(gotBody.Data.Content, "Markdown body") {
		t.Errorf("body data.content = %q", gotBody.Data.Content)
	}
	if !strings.Contains(stdout.String(), `"doc-1"`) {
		t.Errorf("stdout missing new doc id: %q", stdout.String())
	}
}

func TestRunCreateDocRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateDoc(context.Background(), g, createDocFlags{content: "x"}); err == nil {
		t.Fatal("empty board ID should error")
	}
}

func TestRunCreateDocRequiresContent(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateDoc(context.Background(), g, createDocFlags{boardID: "b"}); err == nil {
		t.Fatal("missing content should error")
	}
}

func TestRunCreateDocDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCreateDoc(context.Background(), g, createDocFlags{boardID: "b", content: "hi"}); err != nil {
		t.Fatalf("runCreateDoc: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/docs") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestNewCmdRegistersCreateDoc(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	for _, sub := range cmd.Commands() {
		if sub.Name() == "create-doc" {
			return
		}
	}
	t.Error("`documents` parent missing create-doc subcommand")
}

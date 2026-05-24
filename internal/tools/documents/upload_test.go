package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func TestRunUploadHappyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 dummy"), 0o600); err != nil {
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
		_, _ = w.Write([]byte(`{"id":"doc-new","type":"document"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}

	err := runUpload(context.Background(), g, uploadFlags{
		boardID:  "b1",
		file:     path,
		title:    "Q1 Report",
		x:        0,
		y:        0,
		parentID: "frame-1",
	})
	if err != nil {
		t.Fatalf("runUpload: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/v2/boards/b1/documents" {
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
	if data["title"] != "Q1 Report" {
		t.Errorf("data.title = %v, want Q1 Report", data["title"])
	}
	parent, _ := data["parent"].(map[string]any)
	if parent == nil || parent["id"] != "frame-1" {
		t.Errorf("data.parent = %v, want id=frame-1", data["parent"])
	}
	if _, has := data["position"]; has {
		t.Errorf("position should be omitted at 0,0, got %v", data["position"])
	}
	if !strings.HasPrefix(string(parts["resource"]), "%PDF-1.4") {
		t.Errorf("resource bytes did not pass through: %q", parts["resource"])
	}
}

func TestRunUploadRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpload(context.Background(), g, uploadFlags{file: "/x.pdf"}); err == nil {
		t.Fatal("runUpload with empty board ID returned nil")
	}
}

func TestRunUploadRejectsBadExtension(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "thing.exe")
	if err := os.WriteFile(path, []byte{0x00}, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t"})}
	err := runUpload(context.Background(), g, uploadFlags{boardID: "b1", file: path})
	if err == nil {
		t.Fatal("runUpload with .exe returned nil")
	}
	if !strings.Contains(err.Error(), "unsupported document") {
		t.Errorf("error %q does not mention unsupported document", err)
	}
}

func TestRunUploadRejectsTooLargeFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "big.pdf")
	big := bytes.Repeat([]byte("x"), 6*1024*1024+1)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t"})}
	err := runUpload(context.Background(), g, uploadFlags{boardID: "b1", file: path})
	if err == nil {
		t.Fatal("runUpload over cap returned nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q does not mention size cap", err)
	}
}

func TestRunUploadDryRunSkipsHTTP(t *testing.T) {
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
	err := runUpload(context.Background(), g, uploadFlags{boardID: "b1", file: "/no/file.pdf"})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b1/documents") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestNewUploadCmdRegistered(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{"upload": false, "update-from-file": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("documents parent missing subcommand %q", verb)
		}
	}
}

// ----- helpers --------------------------------------------------------------

func parseMultipartParts(t *testing.T, r io.Reader, contentType string) map[string][]byte {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("no boundary in Content-Type")
	}
	reader := multipart.NewReader(r, boundary)
	out := map[string][]byte{}
	for {
		p, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		body, err := io.ReadAll(p)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		out[p.FormName()] = body
	}
	return out
}

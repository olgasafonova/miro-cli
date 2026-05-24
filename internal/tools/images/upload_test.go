package images

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
	path := filepath.Join(t.TempDir(), "art.png")
	if err := os.WriteFile(path, []byte("imagebytes"), 0o600); err != nil {
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
		_, _ = w.Write([]byte(`{"id":"img-new","type":"image"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}

	err := runUpload(context.Background(), g, uploadFlags{
		boardID:  "b1",
		file:     path,
		title:    "Diagram",
		x:        100,
		y:        50,
		parentID: "frame-1",
	})
	if err != nil {
		t.Fatalf("runUpload: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/v2/boards/b1/images" {
		t.Errorf("server saw %s %s", gotMethod, gotPath)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", gotCT)
	}

	parts := parseMultipartParts(t, bytes.NewReader(gotBody), gotCT)
	var data map[string]any
	if err := json.Unmarshal(parts["data"], &data); err != nil {
		t.Fatalf("decode data field: %v", err)
	}
	if data["title"] != "Diagram" {
		t.Errorf("data.title = %v, want Diagram", data["title"])
	}
	if string(parts["resource"]) != "imagebytes" {
		t.Errorf("resource bytes = %q, want imagebytes", parts["resource"])
	}

	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}
	if out["id"] != "img-new" {
		t.Errorf("emitted id = %v, want img-new", out["id"])
	}
}

func TestRunUploadRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpload(context.Background(), g, uploadFlags{file: "/tmp/x.png"})
	if err == nil {
		t.Fatal("runUpload with empty board ID returned nil")
	}
}

func TestRunUploadRejectsMissingFile(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t"})}
	err := runUpload(context.Background(), g, uploadFlags{
		boardID: "b1",
		file:    filepath.Join(t.TempDir(), "missing.png"),
	})
	if err == nil {
		t.Fatal("runUpload with missing file returned nil")
	}
}

func TestRunUploadRejectsBadExtension(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "x.exe")
	if err := os.WriteFile(path, []byte{0x00}, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t"})}
	err := runUpload(context.Background(), g, uploadFlags{boardID: "b1", file: path})
	if err == nil {
		t.Fatal("runUpload with .exe returned nil")
	}
	if !strings.Contains(err.Error(), "unsupported image") {
		t.Errorf("error %q does not mention unsupported image", err)
	}
}

func TestRunUploadDryRunSkipsHTTPAndFile(t *testing.T) {
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
	// Note: missing file path — dry-run should not validate or open it.
	err := runUpload(context.Background(), g, uploadFlags{boardID: "b1", file: "/no/such/file.png"})
	if err != nil {
		t.Fatalf("runUpload dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b1/images") {
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
			t.Errorf("images parent missing subcommand %q", verb)
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

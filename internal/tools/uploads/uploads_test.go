package uploads

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAndValidateMissingFlag(t *testing.T) {
	t.Parallel()
	if _, err := OpenAndValidate("", ImageValidation); err == nil {
		t.Fatal("OpenAndValidate(\"\") returned nil, want error")
	}
}

func TestOpenAndValidateMissingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nope.png")
	_, err := OpenAndValidate(path, ImageValidation)
	if err == nil {
		t.Fatal("OpenAndValidate on missing file returned nil")
	}
	if !strings.Contains(err.Error(), "cannot access") {
		t.Errorf("error %q does not mention access", err)
	}
}

func TestOpenAndValidateRejectsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := OpenAndValidate(dir, ImageValidation); err == nil {
		t.Fatal("OpenAndValidate on a directory returned nil")
	}
}

func TestOpenAndValidateRejectsBadExtension(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "secret.exe")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := OpenAndValidate(path, ImageValidation)
	if err == nil {
		t.Fatal("OpenAndValidate on .exe returned nil")
	}
	if !strings.Contains(err.Error(), "unsupported image") {
		t.Errorf("error %q does not name the kind", err)
	}
}

func TestOpenAndValidateRejectsOverSize(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "big.pdf")
	// Just barely over the 6 MB cap.
	big := bytes.Repeat([]byte{0x55}, int(MaxDocumentSize)+1)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	_, err := OpenAndValidate(path, DocumentValidation)
	if err == nil {
		t.Fatal("OpenAndValidate past cap returned nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error %q does not mention size cap", err)
	}
}

func TestOpenAndValidateImageHappy(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "fine.PNG") // mixed-case extension
	if err := os.WriteFile(path, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	f, err := OpenAndValidate(path, ImageValidation)
	if err != nil {
		t.Fatalf("OpenAndValidate: %v", err)
	}
	defer func() { _ = f.Close() }()
}

func TestOpenAndValidateDocumentHappy(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 dummy"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	f, err := OpenAndValidate(path, DocumentValidation)
	if err != nil {
		t.Fatalf("OpenAndValidate: %v", err)
	}
	defer func() { _ = f.Close() }()
}

func TestBuildMultipartBodyMinimal(t *testing.T) {
	t.Parallel()
	body, err := BuildMultipartBody(bytes.NewReader([]byte("file-bytes")), "f.png", Form{})
	if err != nil {
		t.Fatalf("BuildMultipartBody: %v", err)
	}
	parts := parseMultipart(t, body.Body, body.ContentType)
	data := parts["data"]
	if data == nil {
		t.Fatal("missing data field")
	}
	if got := string(data.body); got != "{}" {
		t.Errorf("data JSON = %q, want %q", got, "{}")
	}
	resource := parts["resource"]
	if resource == nil {
		t.Fatal("missing resource field")
	}
	if string(resource.body) != "file-bytes" {
		t.Errorf("resource body = %q, want %q", string(resource.body), "file-bytes")
	}
	if resource.filename != "f.png" {
		t.Errorf("filename = %q, want %q", resource.filename, "f.png")
	}
}

func TestBuildMultipartBodyFullForm(t *testing.T) {
	t.Parallel()
	body, err := BuildMultipartBody(strings.NewReader("xx"), "doc.pdf", Form{
		Title:    "Report",
		X:        100,
		Y:        -50,
		ParentID: "frame-1",
	})
	if err != nil {
		t.Fatalf("BuildMultipartBody: %v", err)
	}
	parts := parseMultipart(t, body.Body, body.ContentType)
	var got map[string]any
	if err := json.Unmarshal(parts["data"].body, &got); err != nil {
		t.Fatalf("decode data JSON: %v", err)
	}
	if got["title"] != "Report" {
		t.Errorf("title = %v, want Report", got["title"])
	}
	pos, ok := got["position"].(map[string]any)
	if !ok {
		t.Fatalf("position not a map: %T", got["position"])
	}
	if pos["x"].(float64) != 100 || pos["y"].(float64) != -50 || pos["origin"] != "center" {
		t.Errorf("position = %+v, want x=100 y=-50 origin=center", pos)
	}
	parent, ok := got["parent"].(map[string]any)
	if !ok {
		t.Fatalf("parent not a map: %T", got["parent"])
	}
	if parent["id"] != "frame-1" {
		t.Errorf("parent.id = %v, want frame-1", parent["id"])
	}
}

func TestBuildMultipartBodyZeroPositionIsOmitted(t *testing.T) {
	t.Parallel()
	body, err := BuildMultipartBody(strings.NewReader("x"), "f.png", Form{Title: "Only Title"})
	if err != nil {
		t.Fatalf("BuildMultipartBody: %v", err)
	}
	parts := parseMultipart(t, body.Body, body.ContentType)
	var got map[string]any
	if err := json.Unmarshal(parts["data"].body, &got); err != nil {
		t.Fatalf("decode data JSON: %v", err)
	}
	if _, has := got["position"]; has {
		t.Errorf("zero position should be omitted, got %v", got["position"])
	}
	if got["title"] != "Only Title" {
		t.Errorf("title = %v, want Only Title", got["title"])
	}
}

// ----- helpers --------------------------------------------------------------

type parsedPart struct {
	body     []byte
	filename string
}

// parseMultipart parses buf (a multipart body) according to the boundary
// in contentType and returns a name->part map for test assertions.
func parseMultipart(t *testing.T, buf *bytes.Buffer, contentType string) map[string]*parsedPart {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("no boundary in Content-Type")
	}
	reader := multipart.NewReader(buf, boundary)
	out := map[string]*parsedPart{}
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
			t.Fatalf("ReadAll part: %v", err)
		}
		out[p.FormName()] = &parsedPart{body: body, filename: p.FileName()}
	}
	return out
}

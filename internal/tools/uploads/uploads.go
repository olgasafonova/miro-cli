// Package uploads provides shared helpers for the file-upload variants
// of the images and documents tool groups. Both packages POST/PATCH
// multipart/form-data with two parts: a "data" JSON field (title /
// position / parent) and a "resource" file part (the actual file).
//
// The CLI's threat model is operator-driven: the caller types the path,
// so we don't enforce a symlink-allowlist (the way miro-mcp-server does
// for its LLM-driven threat model). We do enforce extension allowlists
// and a 6 MB cap on documents, matching the Miro API's own behaviour and
// surfacing a clear pre-flight error instead of an opaque server 400.
package uploads

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"miro-cli/internal/miro"
)

// Allowlists for the two supported upload kinds. Keys are lowercase
// extensions including the leading dot. Mirrors miro-mcp-server's
// validImageExts / validDocumentExts so the two surfaces agree on what
// a "valid" upload looks like before the API gets involved.
var (
	ImageExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true,
		".gif": true, ".webp": true, ".svg": true,
	}
	DocumentExts = map[string]bool{
		".pdf": true, ".doc": true, ".docx": true,
		".ppt": true, ".pptx": true,
		".xls": true, ".xlsx": true,
		".txt": true, ".rtf": true, ".csv": true,
	}
)

const (
	imageExtsHint    = "supported: png, jpg, jpeg, gif, webp, svg"
	documentExtsHint = "supported: pdf, doc, docx, ppt, pptx, xls, xlsx, txt, rtf, csv"

	// MaxDocumentSize matches Miro's documented 6 MB cap. Images have no
	// hard cap in the API today; we leave them uncapped here so a future
	// Miro change doesn't strand us with a hardcoded ceiling.
	MaxDocumentSize int64 = 6 * 1024 * 1024
)

// Form is the optional metadata that goes into the multipart "data"
// JSON field. Empty Title / zero position / empty ParentID are omitted
// from the JSON, matching miro-mcp-server's UploadImage / UploadDocument
// shape (no width/height: the Miro upload API doesn't accept geometry).
type Form struct {
	Title    string
	X, Y     float64
	ParentID string
}

// ValidationOpts configures OpenAndValidate. Pre-built ImageValidation
// and DocumentValidation values cover the two callers we have today.
type ValidationOpts struct {
	ValidExts map[string]bool
	Kind      string // "image" | "document" — used in the error message
	Hint      string // human-readable list of allowed extensions
	MaxSize   int64  // 0 means no cap
}

// ImageValidation rejects anything outside ImageExts and applies no
// size cap (Miro publishes no firm limit for image uploads).
var ImageValidation = ValidationOpts{
	ValidExts: ImageExts,
	Kind:      "image",
	Hint:      imageExtsHint,
}

// DocumentValidation rejects anything outside DocumentExts and caps the
// file at 6 MB to match Miro's documented document-upload limit.
var DocumentValidation = ValidationOpts{
	ValidExts: DocumentExts,
	Kind:      "document",
	Hint:      documentExtsHint,
	MaxSize:   MaxDocumentSize,
}

// OpenAndValidate stats the path, checks it's a regular file with an
// allowed extension and (optionally) within a size cap, then opens it
// for reading. Callers must Close the returned *os.File.
func OpenAndValidate(filePath string, opts ValidationOpts) (*os.File, error) {
	if filePath == "" {
		return nil, fmt.Errorf("--file is required")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot access --file %q: %w", filePath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("--file %q is a directory, not a file", filePath)
	}
	if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
		return nil, fmt.Errorf("--file size %d bytes exceeds %d MB limit", info.Size(), opts.MaxSize/(1024*1024))
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	if !opts.ValidExts[ext] {
		return nil, fmt.Errorf("unsupported %s format %q (%s)", opts.Kind, ext, opts.Hint)
	}
	file, err := os.Open(filePath) //nolint:gosec // G304: path is operator-supplied; CLI exists to load operator-chosen files
	if err != nil {
		return nil, fmt.Errorf("open --file %q: %w", filePath, err)
	}
	return file, nil
}

// BuildMultipartBody assembles the "data" + "resource" multipart body
// the Miro upload endpoints expect, returning a *miro.MultipartBody that
// can be passed directly to client.Post / client.Patch.
//
// filename is the basename Miro records on the uploaded item; callers
// typically pass filepath.Base of the operator-supplied path.
func BuildMultipartBody(file io.Reader, filename string, form Form) (*miro.MultipartBody, error) {
	dataBytes, err := buildDataJSON(form)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	dataPart, err := writer.CreateFormField("data")
	if err != nil {
		return nil, fmt.Errorf("create data field: %w", err)
	}
	if _, err := dataPart.Write(dataBytes); err != nil {
		return nil, fmt.Errorf("write data field: %w", err)
	}

	resourcePart, err := writer.CreateFormFile("resource", filename)
	if err != nil {
		return nil, fmt.Errorf("create resource field: %w", err)
	}
	if _, err := io.Copy(resourcePart, file); err != nil {
		return nil, fmt.Errorf("write resource file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	return &miro.MultipartBody{
		Body:        &buf,
		ContentType: writer.FormDataContentType(),
	}, nil
}

// buildDataJSON serializes the optional metadata into the JSON payload
// the multipart "data" field carries. Empty / zero fields are omitted so
// the API sees only the values the operator actually set.
func buildDataJSON(form Form) ([]byte, error) {
	payload := map[string]any{}
	if form.Title != "" {
		payload["title"] = form.Title
	}
	if form.X != 0 || form.Y != 0 {
		payload["position"] = map[string]any{
			"x":      form.X,
			"y":      form.Y,
			"origin": "center",
		}
	}
	if form.ParentID != "" {
		payload["parent"] = map[string]any{"id": form.ParentID}
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal data JSON: %w", err)
	}
	return buf, nil
}

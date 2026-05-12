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

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func TestExtractPictureURLPresent(t *testing.T) {
	resp := map[string]any{
		"id": "abc",
		"picture": map[string]any{
			"imageUrl": "https://miro.com/picture/abc.png",
		},
	}
	got := extractPictureURL(resp)
	if got != "https://miro.com/picture/abc.png" {
		t.Errorf("extractPictureURL = %q, want full URL", got)
	}
}

func TestExtractPictureURLMissingPictureKey(t *testing.T) {
	resp := map[string]any{"id": "abc"}
	if got := extractPictureURL(resp); got != "" {
		t.Errorf("extractPictureURL on missing picture = %q, want empty", got)
	}
}

func TestExtractPictureURLPictureNotAnObject(t *testing.T) {
	// Defensive: API drift might return picture as a bare string. The
	// verb must not panic — return "" so the caller sees a clean
	// "no picture" envelope instead of crashing.
	resp := map[string]any{"id": "abc", "picture": "not-an-object"}
	if got := extractPictureURL(resp); got != "" {
		t.Errorf("extractPictureURL on string-picture = %q, want empty", got)
	}
}

func TestExtractPictureURLMissingImageURL(t *testing.T) {
	resp := map[string]any{
		"id": "abc",
		"picture": map[string]any{
			// no imageUrl field
			"someOtherField": "x",
		},
	}
	if got := extractPictureURL(resp); got != "" {
		t.Errorf("extractPictureURL with no imageUrl = %q, want empty", got)
	}
}

func TestRunPictureHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/boards/abc" {
			t.Errorf("server saw path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id": "abc",
			"name": "Board",
			"picture": {"imageUrl": "https://miro.com/p/abc.png"}
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runPicture(context.Background(), g, "abc"); err != nil {
		t.Fatalf("runPicture: %v", err)
	}
	var out pictureResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.BoardID != "abc" {
		t.Errorf("board_id = %q, want abc", out.BoardID)
	}
	if out.ImageURL != "https://miro.com/p/abc.png" {
		t.Errorf("image_url = %q", out.ImageURL)
	}
	if out.Message != "" {
		t.Errorf("happy-path message = %q, want empty", out.Message)
	}
}

func TestRunPictureBoardWithoutPicture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"empty-board","name":"Fresh"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runPicture(context.Background(), g, "empty-board"); err != nil {
		t.Fatalf("runPicture: %v", err)
	}
	var out pictureResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ImageURL != "" {
		t.Errorf("image_url for board without picture = %q, want empty", out.ImageURL)
	}
	if !strings.Contains(out.Message, "no picture") {
		t.Errorf("message did not signal missing picture: %q", out.Message)
	}
}

func TestRunPictureDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runPicture(context.Background(), g, "abc"); err != nil {
		t.Fatalf("runPicture: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/abc") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunPictureEmptyIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runPicture(context.Background(), g, ""); err == nil {
		t.Fatal("runPicture with empty board_id returned nil, want error")
	}
}

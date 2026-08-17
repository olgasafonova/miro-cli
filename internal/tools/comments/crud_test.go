package comments

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

// wireComment builds the API's comment-thread shape as captured live on
// 13-08-2026 (see the package doc for the probe notes).
func wireComment(id string, resolved bool, itemID string, messages ...string) map[string]any {
	msgs := make([]map[string]any, len(messages))
	for i, content := range messages {
		msgs[i] = map[string]any{
			"id": "msg-" + content, "type": "message", "content": content,
			"createdAt": "2026-08-12T22:44:21Z",
			"createdBy": map[string]any{"id": "u1", "type": "user", "name": "Olga Safonova"},
		}
	}
	c := map[string]any{
		"id": id, "type": "comment", "resolved": resolved,
		"createdAt": "2026-08-12T22:44:21Z",
		"createdBy": map[string]any{"id": "u1", "type": "user", "name": "Olga Safonova"},
		"messages":  msgs,
		"position":  map[string]any{"type": "canvas", "x": 0.0, "y": 0.0},
	}
	if itemID != "" {
		c["position"] = map[string]any{"type": "attached", "x": 0.0, "y": 0.0, "itemId": itemID}
	}
	return c
}

// newTestGlobals wires a Globals at the given httptest server with a
// stdout buffer for output assertions.
func newTestGlobals(srvURL string) (*clictx.Globals, *bytes.Buffer) {
	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srvURL})}
	return g, &stdout
}

func TestBuildListPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   ListFlags
		want string
	}{
		{
			name: "minimal",
			in:   ListFlags{BoardID: "uXjV-board-1"},
			want: "/v2-experimental/boards/uXjV-board-1/comments",
		},
		{
			name: "with limit",
			in:   ListFlags{BoardID: "abc", Limit: 25},
			want: "/v2-experimental/boards/abc/comments?limit=25",
		},
		{
			name: "with limit + offset",
			in:   ListFlags{BoardID: "abc", Limit: 50, Offset: 40},
			want: "/v2-experimental/boards/abc/comments?limit=50&offset=40",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildListPath(tc.in); got != tc.want {
				t.Errorf("BuildListPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunCreateBoardLevel(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(wireComment("c1", false, "", "First!"))
	}))
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runCreate(context.Background(), g, createFlags{boardID: "abc", content: "First!"})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if want := "POST /v2-experimental/boards/abc/comments"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	if gotBody["content"] != "First!" {
		t.Errorf("content = %v, want 'First!'", gotBody["content"])
	}
	if _, has := gotBody["itemId"]; has {
		t.Error("itemId sent for a board-level comment")
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if out["id"] != "c1" {
		t.Errorf("emitted id = %v, want c1", out["id"])
	}
}

func TestRunCreateAttachedSendsItemID(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(wireComment("c2", false, "item42", "On the sticky"))
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runCreate(context.Background(), g, createFlags{boardID: "abc", content: "On the sticky", itemID: "item42"})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotBody["itemId"] != "item42" {
		t.Errorf("itemId = %v, want 'item42'", gotBody["itemId"])
	}
}

func TestRunCreateValidation(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "abc"}); err == nil {
		t.Error("empty --content accepted")
	}
	if err := runCreate(context.Background(), g, createFlags{content: "x"}); err == nil {
		t.Error("empty --board-id accepted")
	}
	if err := runCreate(context.Background(), g, createFlags{boardID: "abc", content: "x", itemID: "a/b"}); err == nil {
		t.Error("malformed --item-id accepted")
	}
}

func TestRunListEmitsThreads(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				wireComment("c1", true, "", "resolved thread", "with a reply"),
				wireComment("c2", false, "item9", "open thread"),
			},
			"total": 2, "offset": 0, "size": 2, "limit": 20,
		})
	}))
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	if err := runList(context.Background(), g, ListFlags{BoardID: "abc", Limit: 20}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if want := "/v2-experimental/boards/abc/comments?limit=20"; gotPath != want {
		t.Errorf("server saw path %q, want %q", gotPath, want)
	}
	var out ListResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 || out.Total != 2 {
		t.Errorf("emitted %d threads (total %d), want 2/2", len(out.Data), out.Total)
	}
	// Thread payloads pass through verbatim: messages[] and the attached
	// position must survive the round trip.
	msgs, ok := out.Data[0]["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Errorf("first thread messages = %v, want 2 entries", out.Data[0]["messages"])
	}
	pos, ok := out.Data[1]["position"].(map[string]any)
	if !ok || pos["itemId"] != "item9" {
		t.Errorf("attached position = %v, want itemId item9", out.Data[1]["position"])
	}
}

func TestRunGetHitsThreadPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewEncoder(w).Encode(wireComment("c1", false, "", "original"))
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	if err := runGet(context.Background(), g, "abc", "c1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if want := "GET /v2-experimental/boards/abc/comments/c1"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
}

func TestRunReplyPostsToMessages(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(wireComment("c1", false, "", "original", "the reply"))
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runReply(context.Background(), g, replyFlags{boardID: "abc", commentID: "c1", content: "the reply"})
	if err != nil {
		t.Fatalf("runReply: %v", err)
	}
	if want := "POST /v2-experimental/boards/abc/comments/c1/messages"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	if gotBody["content"] != "the reply" {
		t.Errorf("content = %v, want 'the reply'", gotBody["content"])
	}
}

func TestRunResolveSendsBothDirections(t *testing.T) {
	tests := []struct {
		name   string
		reopen bool
		want   bool
	}{
		{name: "resolve", reopen: false, want: true},
		{name: "reopen", reopen: true, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				_ = json.NewEncoder(w).Encode(wireComment("c1", tc.want, "", "msg"))
			}))
			defer srv.Close()

			g, _ := newTestGlobals(srv.URL)
			err := runResolve(context.Background(), g, resolveFlags{boardID: "abc", commentID: "c1", reopen: tc.reopen})
			if err != nil {
				t.Fatalf("runResolve: %v", err)
			}
			if gotMethod != http.MethodPatch {
				t.Errorf("method = %s, want PATCH", gotMethod)
			}
			if gotBody["resolved"] != tc.want {
				t.Errorf("sent resolved = %v, want %v", gotBody["resolved"], tc.want)
			}
		})
	}
}

func TestDryRunSkipsHTTP(t *testing.T) {
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
			name: "create",
			run: func(g *clictx.Globals) error {
				return runCreate(context.Background(), g, createFlags{boardID: "abc", content: "x"})
			},
			want: "DRY-RUN POST /v2-experimental/boards/abc/comments",
		},
		{
			name: "list",
			run: func(g *clictx.Globals) error {
				return runList(context.Background(), g, ListFlags{BoardID: "abc", Limit: 10})
			},
			want: "DRY-RUN GET /v2-experimental/boards/abc/comments?limit=10",
		},
		{
			name: "get",
			run: func(g *clictx.Globals) error {
				return runGet(context.Background(), g, "abc", "c1")
			},
			want: "DRY-RUN GET /v2-experimental/boards/abc/comments/c1",
		},
		{
			name: "reply",
			run: func(g *clictx.Globals) error {
				return runReply(context.Background(), g, replyFlags{boardID: "abc", commentID: "c1", content: "x"})
			},
			want: "DRY-RUN POST /v2-experimental/boards/abc/comments/c1/messages",
		},
		{
			name: "resolve",
			run: func(g *clictx.Globals) error {
				return runResolve(context.Background(), g, resolveFlags{boardID: "abc", commentID: "c1"})
			},
			want: "DRY-RUN PATCH /v2-experimental/boards/abc/comments/c1",
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

func TestErrorsCarryExperimentalHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404,"message":"not found"}`))
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runList(context.Background(), g, ListFlags{BoardID: "abc"})
	if err == nil {
		t.Fatal("404 response returned nil error")
	}
	if !strings.Contains(err.Error(), "v2-experimental") {
		t.Errorf("404 error lacks the experimental-availability hint: %v", err)
	}
	// The wrap must not break the exit-code contract.
	var apiErr *miro.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("wrapped error no longer unwraps to *miro.APIError: %v", err)
	}
	if got := miro.ExitCode(err); got != miro.ExitNotFound {
		t.Errorf("ExitCode(404) = %d, want %d (ExitNotFound)", got, miro.ExitNotFound)
	}
}

func TestWrapExperimentalErrPassesOthersThrough(t *testing.T) {
	t.Parallel()
	plain := errors.New("boom")
	if got := wrapExperimentalErr(plain); !errors.Is(got, plain) || strings.Contains(got.Error(), "v2-experimental") {
		t.Errorf("non-API error rewrapped: %v", got)
	}
	server500 := &miro.APIError{Method: "GET", Path: "/x", Status: 500}
	if got := wrapExperimentalErr(server500); strings.Contains(got.Error(), "v2-experimental") {
		t.Errorf("500 error got the experimental hint: %v", got)
	}
}

func TestRunListPropagatesContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the call
	if err := runList(ctx, g, ListFlags{BoardID: "abc"}); err == nil {
		t.Fatal("runList with cancelled context returned nil")
	}
}

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "comments" {
		t.Errorf("Use = %q, want comments", cmd.Use)
	}
	got := map[string]bool{}
	for _, sub := range cmd.Commands() {
		got[sub.Name()] = true
	}
	for _, want := range []string{"create", "list", "get", "reply", "resolve"} {
		if !got[want] {
			t.Errorf("comments parent did not register %q", want)
		}
	}
	if got["delete"] {
		t.Error("comments registered a delete verb; the API answers 405 and none should be offered")
	}
}

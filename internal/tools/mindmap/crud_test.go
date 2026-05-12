package mindmap

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

// ----- list -----------------------------------------------------------------

func TestBuildListPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   listFlags
		want string
	}{
		{
			name: "minimal",
			in:   listFlags{boardID: "abc"},
			want: "/v2-experimental/boards/abc/mindmap_nodes",
		},
		{
			name: "with limit",
			in:   listFlags{boardID: "abc", limit: 25},
			want: "/v2-experimental/boards/abc/mindmap_nodes?limit=25",
		},
		{
			name: "with cursor",
			in:   listFlags{boardID: "abc", cursor: "c-1"},
			want: "/v2-experimental/boards/abc/mindmap_nodes?cursor=c-1",
		},
		{
			name: "with limit + cursor",
			in:   listFlags{boardID: "abc", limit: 25, cursor: "c-1"},
			want: "/v2-experimental/boards/abc/mindmap_nodes?cursor=c-1&limit=25",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := buildListPath(tc.in); got != tc.want {
				t.Errorf("buildListPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunListHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "m1", "type": "mindmap_node"},
				{"id": "m2", "type": "mindmap_node"}
			],
			"total": 2,
			"cursor": "next-page"
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runList(context.Background(), g, listFlags{boardID: "abc", limit: 50}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if gotPath != "/v2-experimental/boards/abc/mindmap_nodes?limit=50" {
		t.Errorf("server saw path %q", gotPath)
	}
	var out listResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 {
		t.Errorf("emitted %d items, want 2", len(out.Data))
	}
	if out.Cursor != "next-page" {
		t.Errorf("cursor = %q, want next-page", out.Cursor)
	}
}

func TestRunListRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runList(context.Background(), g, listFlags{}); err == nil {
		t.Fatal("runList with empty board ID returned nil, want error")
	}
}

func TestRunListDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runList(context.Background(), g, listFlags{boardID: "abc"}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2-experimental/boards/abc/mindmap_nodes") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- create ---------------------------------------------------------------

func TestBuildCreateRequestRootNode(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{content: "hello"})
	if req.Data.NodeView.Data.Type != "text" {
		t.Errorf("nodeView.data.type = %q, want text", req.Data.NodeView.Data.Type)
	}
	if req.Data.NodeView.Data.Content != "hello" {
		t.Errorf("nodeView.data.content = %q, want hello", req.Data.NodeView.Data.Content)
	}
	if req.Parent != nil {
		t.Errorf("parent envelope should be nil for root node, got %+v", req.Parent)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
}

func TestBuildCreateRequestChildNodeIncludesParent(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{content: "child", parentID: "node-7", x: 100, y: 200})
	if req.Parent == nil {
		t.Fatal("parent envelope should be non-nil when --parent-id set")
	}
	if req.Parent.ID != "node-7" {
		t.Errorf("parent.id = %q, want node-7", req.Parent.ID)
	}
	if req.Position == nil || req.Position.X != 100 || req.Position.Y != 200 {
		t.Errorf("position = %+v, want x=100 y=200", req.Position)
	}
}

func TestBuildCreateRequestMarshalsToExpectedJSON(t *testing.T) {
	t.Parallel()
	// Root node: parent key must be absent entirely (not "parent": null
	// or "parent": {}).
	root := buildCreateRequest(createFlags{content: "root"})
	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"parent"`) {
		t.Errorf("root-node JSON should omit parent key entirely, got %s", raw)
	}

	// Child node: parent must be present with the id.
	child := buildCreateRequest(createFlags{content: "child", parentID: "p1"})
	raw, err = json.Marshal(child)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"parent":{"id":"p1"}`) {
		t.Errorf("child-node JSON missing parent envelope, got %s", raw)
	}
}

func TestRunCreateSendsBody(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   createRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"node-1","type":"mindmap_node"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{
		boardID:  "uXjV1",
		content:  "root topic",
		parentID: "frame-1",
	}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2-experimental/boards/uXjV1/mindmap_nodes" {
		t.Errorf("path = %q, want /v2-experimental/boards/uXjV1/mindmap_nodes", gotPath)
	}
	if gotBody.Data.NodeView.Data.Content != "root topic" {
		t.Errorf("body content = %q, want root topic", gotBody.Data.NodeView.Data.Content)
	}
	if gotBody.Parent == nil || gotBody.Parent.ID != "frame-1" {
		t.Errorf("body parent = %+v, want id=frame-1", gotBody.Parent)
	}
	if !strings.Contains(stdout.String(), `"node-1"`) {
		t.Errorf("stdout missing new node id: %q", stdout.String())
	}
}

func TestRunCreateRootNodeOmitsParentOnWire(t *testing.T) {
	t.Parallel()
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"id":"node-1"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{
		boardID: "b",
		content: "root",
	}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if strings.Contains(string(rawBody), `"parent"`) {
		t.Errorf("root-node wire body should omit parent key, got %s", rawBody)
	}
}

func TestRunCreateRejectsEmptyContent(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err == nil {
		t.Fatal("runCreate with empty content returned nil, want error")
	}
}

func TestRunCreateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{content: "x"}); err == nil {
		t.Fatal("runCreate with empty board ID returned nil, want error")
	}
}

func TestRunCreateDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", content: "x"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2-experimental/boards/b/mindmap_nodes") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"m1","type":"mindmap_node","data":{"nodeView":{"data":{"type":"text","content":"hello"}}}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "m1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2-experimental/boards/b1/mindmap_nodes/m1" {
		t.Errorf("path = %q, want /v2-experimental/boards/b1/mindmap_nodes/m1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"hello"`) {
		t.Errorf("stdout missing content: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "m"); err == nil {
		t.Fatal("runGet with empty board ID returned nil, want error")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runGet with empty item ID returned nil, want error")
	}
}

func TestRunGetNotFoundMapsToExitCode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runGet(context.Background(), g, "b", "missing")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if code := miro.ExitCode(err); code != miro.ExitNotFound {
		t.Errorf("404 mapped to exit %d, want %d (not-found)", code, miro.ExitNotFound)
	}
}

// ----- delete ---------------------------------------------------------------

func TestRunDeleteRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDelete(context.Background(), g, "b", "m")
	if err == nil {
		t.Fatal("runDelete without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
	if !strings.Contains(err.Error(), "deleting a parent node deletes its children") {
		t.Errorf("refusal message should warn about subtree deletion, got %q", err.Error())
	}
}

func TestRunDeleteWithYesCallsAPI(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	if err := runDelete(context.Background(), g, "b", "m1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2-experimental/boards/b/mindmap_nodes/m1" {
		t.Errorf("path = %q, want /v2-experimental/boards/b/mindmap_nodes/m1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"deleted": true`) {
		t.Errorf("stdout missing deleted envelope: %q", stdout.String())
	}
}

func TestRunDeleteDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runDelete(context.Background(), g, "b", "m"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2-experimental/boards/b/mindmap_nodes/m") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunDeleteAgentImpliesYes(t *testing.T) {
	t.Parallel()
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: new(bytes.Buffer),
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Agent:  true,
	}
	g.Normalize()
	if err := runDelete(context.Background(), g, "b", "m"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

func TestRunDeleteRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Yes: true}
	if err := runDelete(context.Background(), g, "", "m"); err == nil {
		t.Fatal("runDelete with empty board ID returned nil, want error")
	}
	if err := runDelete(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runDelete with empty item ID returned nil, want error")
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "mindmap" {
		t.Errorf("parent command Use = %q, want mindmap", cmd.Use)
	}
	want := map[string]bool{"list": false, "create": false, "get": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`mindmap` parent missing subcommand %q", verb)
		}
	}
}

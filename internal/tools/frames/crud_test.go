package frames

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

// ----- create ---------------------------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{})
	if req.Data.Title != "" {
		t.Errorf("title = %q, want empty", req.Data.Title)
	}
	if req.Data.Format != "" {
		t.Errorf("format = %q, want empty (passthrough lets API default)", req.Data.Format)
	}
	if req.Data.Type != "" {
		t.Errorf("type = %q, want empty (passthrough lets API default)", req.Data.Type)
	}
	if req.Style != nil {
		t.Errorf("style should be nil when --color unset, got %+v", req.Style)
	}
	if req.Geometry != nil {
		t.Errorf("geometry should be nil when --width/--height unset, got %+v", req.Geometry)
	}
	if req.Parent != nil {
		t.Errorf("parent should be nil when --parent-id unset, got %+v", req.Parent)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
}

func TestBuildCreateRequestFull(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{
		title:       "Section A",
		format:      "custom",
		frameType:   "freeform",
		showContent: true,
		color:       "#ffffff",
		x:           100,
		y:           200,
		width:       800,
		height:      600,
		parentID:    "frame-parent",
	})
	if req.Data.Title != "Section A" {
		t.Errorf("title = %q, want Section A", req.Data.Title)
	}
	if req.Data.Format != "custom" {
		t.Errorf("format = %q, want custom", req.Data.Format)
	}
	if req.Data.Type != "freeform" {
		t.Errorf("type = %q, want freeform", req.Data.Type)
	}
	if !req.Data.ShowContent {
		t.Error("showContent = false, want true")
	}
	if req.Style == nil || req.Style.FillColor != "#ffffff" {
		t.Errorf("style = %+v, want fillColor=#ffffff", req.Style)
	}
	if req.Geometry == nil || req.Geometry.Width != 800 || req.Geometry.Height != 600 {
		t.Errorf("geometry = %+v, want width=800 height=600", req.Geometry)
	}
	if req.Parent == nil || req.Parent.ID != "frame-parent" {
		t.Errorf("parent = %+v, want id=frame-parent", req.Parent)
	}
}

func TestBuildCreateRequestColorPassthrough(t *testing.T) {
	t.Parallel()
	// Frames take hex fillColor strings; no normalization layer.
	req := buildCreateRequest(createFlags{color: "#aabbcc"})
	if req.Style == nil || req.Style.FillColor != "#aabbcc" {
		t.Errorf("color hex should passthrough verbatim: %+v", req.Style)
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
		_, _ = w.Write([]byte(`{"id":"frame-1","data":{"title":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{
		boardID: "uXjV1", title: "hi", color: "#ffffff", width: 800, height: 600, parentID: "frame-parent",
	}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/frames" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/frames", gotPath)
	}
	if gotBody.Data.Title != "hi" {
		t.Errorf("body title = %q, want hi", gotBody.Data.Title)
	}
	if gotBody.Style == nil || gotBody.Style.FillColor != "#ffffff" {
		t.Errorf("body style = %+v, want fillColor=#ffffff", gotBody.Style)
	}
	if gotBody.Geometry == nil || gotBody.Geometry.Width != 800 || gotBody.Geometry.Height != 600 {
		t.Errorf("body geometry = %+v, want width=800 height=600", gotBody.Geometry)
	}
	if gotBody.Parent == nil || gotBody.Parent.ID != "frame-parent" {
		t.Errorf("body parent = %+v, want id=frame-parent", gotBody.Parent)
	}
	if !strings.Contains(stdout.String(), `"frame-1"`) {
		t.Errorf("stdout missing new frame id: %q", stdout.String())
	}
}

func TestRunCreateBareFrameOK(t *testing.T) {
	t.Parallel()
	// A bare frame (only --board-id) is a valid resource. No flag
	// beyond the board ID is required.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"frame-1"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err != nil {
		t.Fatalf("runCreate (bare): %v", err)
	}
	if gotPath != "/v2/boards/b/frames" {
		t.Errorf("path = %q, want /v2/boards/b/frames", gotPath)
	}
}

func TestRunCreateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{title: "hi"}); err == nil {
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
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", title: "hi"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/frames") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"f1","data":{"title":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "f1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/frames/f1" {
		t.Errorf("path = %q, want /v2/boards/b1/frames/f1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"hi"`) {
		t.Errorf("stdout missing title: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "f"); err == nil {
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

// ----- update ---------------------------------------------------------------

func TestBuildUpdateRequestNoFieldsReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := buildUpdateRequest(updateFlags{})
	if ok {
		t.Error("buildUpdateRequest with no fields should report ok=false")
	}
}

func TestBuildUpdateRequestOnlyTitleSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{title: "new", titleSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with title should report ok=true")
	}
	if req.Data == nil || req.Data.Title != "new" {
		t.Errorf("data.title = %+v, want new", req.Data)
	}
	if req.Style != nil || req.Position != nil || req.Geometry != nil || req.Parent != nil {
		t.Errorf("unset sections should stay nil: %+v", req)
	}
}

func TestBuildUpdateRequestShowContentSet(t *testing.T) {
	t.Parallel()
	// Explicit --show-content (even with default-false value) should
	// flow through; the user is asking to toggle the bit.
	req, ok := buildUpdateRequest(updateFlags{showContent: true, showContentSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with showContentSet should report ok=true")
	}
	if req.Data == nil || !req.Data.ShowContent {
		t.Errorf("data.showContent = %+v, want true", req.Data)
	}
}

func TestBuildUpdateRequestXZeroExplicit(t *testing.T) {
	t.Parallel()
	// User explicitly set --x=0; that's a valid coordinate the API
	// should receive. The bool guard distinguishes "explicit zero" from
	// "unset float."
	req, ok := buildUpdateRequest(updateFlags{x: 0, xSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with xSet should report ok=true")
	}
	if req.Position == nil {
		t.Fatal("position should be non-nil when --x set")
	}
	if req.Position.X != 0 {
		t.Errorf("position.x = %v, want 0", req.Position.X)
	}
	if req.Position.Origin != "center" {
		t.Errorf("position.origin = %q, want center", req.Position.Origin)
	}
}

func TestBuildUpdateRequestHeightOnly(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{height: 400, heightSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with heightSet should report ok=true")
	}
	if req.Geometry == nil || req.Geometry.Height != 400 {
		t.Errorf("geometry = %+v, want height=400", req.Geometry)
	}
	if req.Geometry.Width != 0 {
		t.Errorf("geometry.width = %v, want 0 (unset)", req.Geometry.Width)
	}
}

func TestBuildUpdateRequestParentDetach(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{parentID: "", parentIDSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with parentIDSet should report ok=true")
	}
	if req.Parent == nil {
		t.Fatal("parent envelope should be present when --parent-id explicitly empty")
	}
	if req.Parent.ID != "" {
		t.Errorf("parent.id = %q, want empty (detach)", req.Parent.ID)
	}
}

func TestRunUpdatePatchesAndReturnsFrame(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   updateRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"f1","data":{"title":"new"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "f1", title: "new", titleSet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/frames/f1" {
		t.Errorf("path = %q, want /v2/boards/b/frames/f1", gotPath)
	}
	if gotBody.Data == nil || gotBody.Data.Title != "new" {
		t.Errorf("body data = %+v, want title=new", gotBody.Data)
	}
	if !strings.Contains(stdout.String(), `"new"`) {
		t.Errorf("stdout missing updated title: %q", stdout.String())
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "f"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{itemID: "f", title: "x", titleSet: true}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", title: "x", titleSet: true}); err == nil {
		t.Fatal("runUpdate with empty item ID returned nil, want error")
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
	err := runDelete(context.Background(), g, "b", "f")
	if err == nil {
		t.Fatal("runDelete without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
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
	if err := runDelete(context.Background(), g, "b", "f1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/frames/f1" {
		t.Errorf("path = %q, want /v2/boards/b/frames/f1", gotPath)
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
	if err := runDelete(context.Background(), g, "b", "f"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/frames/f") {
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
	if err := runDelete(context.Background(), g, "b", "f"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllCRUDVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{"create": false, "get": false, "update": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`frames` parent missing subcommand %q", verb)
		}
	}
}

package documents

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
	req := buildCreateRequest(createFlags{url: "https://example.com/doc.pdf"})
	if req.Data.URL != "https://example.com/doc.pdf" {
		t.Errorf("url = %q, want https://example.com/doc.pdf", req.Data.URL)
	}
	if req.Data.Title != "" {
		t.Errorf("title should be empty when --title unset, got %q", req.Data.Title)
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

func TestBuildCreateRequestFullPayload(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{
		url:      "https://example.com/spec.pdf",
		title:    "Spec sheet",
		x:        10,
		y:        20,
		width:    640,
		height:   360,
		parentID: "frame-1",
	})
	if req.Data.URL != "https://example.com/spec.pdf" {
		t.Errorf("url = %q", req.Data.URL)
	}
	if req.Data.Title != "Spec sheet" {
		t.Errorf("title = %q, want Spec sheet", req.Data.Title)
	}
	if req.Geometry == nil || req.Geometry.Width != 640 || req.Geometry.Height != 360 {
		t.Errorf("geometry = %+v, want width=640 height=360", req.Geometry)
	}
	if req.Parent == nil || req.Parent.ID != "frame-1" {
		t.Errorf("parent = %+v, want id=frame-1", req.Parent)
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
		_, _ = w.Write([]byte(`{"id":"doc-1","data":{"url":"https://example.com/doc.pdf"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{
		boardID:  "uXjV1",
		url:      "https://example.com/doc.pdf",
		title:    "Doc title",
		width:    640,
		height:   360,
		parentID: "frame-1",
	}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/documents" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/documents", gotPath)
	}
	if gotBody.Data.URL != "https://example.com/doc.pdf" {
		t.Errorf("body url = %q, want https://example.com/doc.pdf", gotBody.Data.URL)
	}
	if gotBody.Data.Title != "Doc title" {
		t.Errorf("body title = %q, want Doc title", gotBody.Data.Title)
	}
	if gotBody.Geometry == nil || gotBody.Geometry.Width != 640 || gotBody.Geometry.Height != 360 {
		t.Errorf("body geometry = %+v, want width=640 height=360", gotBody.Geometry)
	}
	if gotBody.Parent == nil || gotBody.Parent.ID != "frame-1" {
		t.Errorf("body parent = %+v, want id=frame-1", gotBody.Parent)
	}
	if !strings.Contains(stdout.String(), `"doc-1"`) {
		t.Errorf("stdout missing new document id: %q", stdout.String())
	}
}

func TestRunCreateRejectsEmptyURL(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err == nil {
		t.Fatal("runCreate with empty url returned nil, want error")
	}
}

func TestRunCreateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{url: "https://example.com/doc.pdf"}); err == nil {
		t.Fatal("runCreate with empty board ID returned nil, want error")
	}
}

func TestRunCreateAcceptsEmptyTitle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"d1"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", url: "https://x"}); err != nil {
		t.Fatalf("runCreate with empty title: %v", err)
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
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", url: "https://x"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/documents") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"d1","data":{"url":"https://example.com/doc.pdf"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "d1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/documents/d1" {
		t.Errorf("path = %q, want /v2/boards/b1/documents/d1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"https://example.com/doc.pdf"`) {
		t.Errorf("stdout missing url: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "d"); err == nil {
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

func TestBuildUpdateRequestOnlyURLSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{url: "https://new", urlSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with url should report ok=true")
	}
	if req.Data == nil || req.Data.URL != "https://new" {
		t.Errorf("data.url = %+v, want https://new", req.Data)
	}
	if req.Position != nil || req.Geometry != nil || req.Parent != nil {
		t.Errorf("unset sections should stay nil: %+v", req)
	}
}

func TestBuildUpdateRequestOnlyTitleSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{title: "Renamed", titleSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with title should report ok=true")
	}
	if req.Data == nil || req.Data.Title != "Renamed" {
		t.Errorf("data.title = %+v, want Renamed", req.Data)
	}
	if req.Data.URL != "" {
		t.Errorf("data.url should stay empty when only title set, got %q", req.Data.URL)
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

func TestBuildUpdateRequestGeometryWidthOnly(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{width: 800, widthSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with widthSet should report ok=true")
	}
	if req.Geometry == nil || req.Geometry.Width != 800 {
		t.Errorf("geometry = %+v, want width=800", req.Geometry)
	}
	if req.Geometry.Height != 0 {
		t.Errorf("geometry.height = %v, want 0", req.Geometry.Height)
	}
}

func TestRunUpdatePatchesAndReturnsDocument(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"d1","data":{"url":"https://new"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "d1", url: "https://new", urlSet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/documents/d1" {
		t.Errorf("path = %q, want /v2/boards/b/documents/d1", gotPath)
	}
	if gotBody.Data == nil || gotBody.Data.URL != "https://new" {
		t.Errorf("body data = %+v, want url=https://new", gotBody.Data)
	}
	if !strings.Contains(stdout.String(), `"https://new"`) {
		t.Errorf("stdout missing updated url: %q", stdout.String())
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "d"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{itemID: "d", url: "x", urlSet: true}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", url: "x", urlSet: true}); err == nil {
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
	err := runDelete(context.Background(), g, "b", "d")
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
	if err := runDelete(context.Background(), g, "b", "d1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/documents/d1" {
		t.Errorf("path = %q, want /v2/boards/b/documents/d1", gotPath)
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
	if err := runDelete(context.Background(), g, "b", "d"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/documents/d") {
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
	if err := runDelete(context.Background(), g, "b", "d"); err != nil {
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
			t.Errorf("`documents` parent missing subcommand %q", verb)
		}
	}
}

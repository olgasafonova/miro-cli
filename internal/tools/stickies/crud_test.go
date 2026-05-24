package stickies

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// ----- normalize ------------------------------------------------------------

func TestNormalizeStickyColor(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"yellow", "light_yellow"},
		{"YELLOW", "light_yellow"},
		{"green", "light_green"},
		{"blue", "light_blue"},
		{"pink", "light_pink"},
		{"purple", "violet"},
		{"grey", "gray"},
		{"gray", "gray"},
		{"light_yellow", "light_yellow"}, // passthrough — Miro-native names survive
		{"#ff0000", "#ff0000"},           // hex passthrough
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeStickyColor(c.in); got != c.want {
			t.Errorf("normalizeStickyColor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ----- create ---------------------------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{content: "hi"})
	if req.Data.Content != "hi" {
		t.Errorf("content = %q, want hi", req.Data.Content)
	}
	if req.Style != nil {
		t.Errorf("style should be nil when --color unset, got %+v", req.Style)
	}
	if req.Geometry != nil {
		t.Errorf("geometry should be nil when --width unset, got %+v", req.Geometry)
	}
	if req.Parent != nil {
		t.Errorf("parent should be nil when --parent-id unset, got %+v", req.Parent)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
}

func TestBuildCreateRequestNormalizesColor(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{content: "hi", color: "yellow"})
	if req.Style == nil || req.Style.FillColor != "light_yellow" {
		t.Errorf("color yellow should normalize to light_yellow: %+v", req.Style)
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
		_, _ = w.Write([]byte(`{"id":"sticky-1","data":{"content":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "uXjV1", content: "hi", color: "yellow", width: 199, parentID: "frame-1"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/sticky_notes" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/sticky_notes", gotPath)
	}
	if gotBody.Data.Content != "hi" {
		t.Errorf("body content = %q, want hi", gotBody.Data.Content)
	}
	if gotBody.Style == nil || gotBody.Style.FillColor != "light_yellow" {
		t.Errorf("body style = %+v, want fillColor=light_yellow", gotBody.Style)
	}
	if gotBody.Geometry == nil || gotBody.Geometry.Width != 199 {
		t.Errorf("body geometry = %+v, want width=199", gotBody.Geometry)
	}
	if gotBody.Parent == nil || gotBody.Parent.ID != "frame-1" {
		t.Errorf("body parent = %+v, want id=frame-1", gotBody.Parent)
	}
	if !strings.Contains(stdout.String(), `"sticky-1"`) {
		t.Errorf("stdout missing new sticky id: %q", stdout.String())
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
	if err := runCreate(context.Background(), g, createFlags{content: "hi"}); err == nil {
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
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", content: "hi"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/sticky_notes") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"s1","data":{"content":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "s1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/sticky_notes/s1" {
		t.Errorf("path = %q, want /v2/boards/b1/sticky_notes/s1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"hi"`) {
		t.Errorf("stdout missing content: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "s"); err == nil {
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

func TestBuildUpdateRequestOnlyContentSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{content: "new", contentSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with content should report ok=true")
	}
	if req.Data == nil || req.Data.Content != "new" {
		t.Errorf("data.content = %+v, want new", req.Data)
	}
	if req.Style != nil || req.Position != nil || req.Geometry != nil || req.Parent != nil {
		t.Errorf("unset sections should stay nil: %+v", req)
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

func TestRunUpdatePatchesAndReturnsSticky(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"s1","data":{"content":"new"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "s1", content: "new", contentSet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/sticky_notes/s1" {
		t.Errorf("path = %q, want /v2/boards/b/sticky_notes/s1", gotPath)
	}
	if gotBody.Data == nil || gotBody.Data.Content != "new" {
		t.Errorf("body data = %+v, want content=new", gotBody.Data)
	}
	if !strings.Contains(stdout.String(), `"new"`) {
		t.Errorf("stdout missing updated content: %q", stdout.String())
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "s"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{itemID: "s", content: "x", contentSet: true}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", content: "x", contentSet: true}); err == nil {
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
	err := runDelete(context.Background(), g, "b", "s")
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
	if err := runDelete(context.Background(), g, "b", "s1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/sticky_notes/s1" {
		t.Errorf("path = %q, want /v2/boards/b/sticky_notes/s1", gotPath)
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
	if err := runDelete(context.Background(), g, "b", "s"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/sticky_notes/s") {
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
	if err := runDelete(context.Background(), g, "b", "s"); err != nil {
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
			t.Errorf("`stickies` parent missing subcommand %q", verb)
		}
	}
}

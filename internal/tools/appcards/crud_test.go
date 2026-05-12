package appcards

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

// ----- status validation ----------------------------------------------------

func TestValidateStatusAcceptsEmpty(t *testing.T) {
	t.Parallel()
	if err := validateStatus(""); err != nil {
		t.Errorf("validateStatus(\"\") = %v, want nil (empty means don't send)", err)
	}
}

func TestValidateStatusAcceptsKnownValues(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"disconnected", "connected", "disabled"} {
		if err := validateStatus(s); err != nil {
			t.Errorf("validateStatus(%q) = %v, want nil", s, err)
		}
	}
}

func TestValidateStatusRejectsUnknown(t *testing.T) {
	t.Parallel()
	cases := []string{"DISCONNECTED", "active", "foo", "  ", "connected ", "enabled"}
	for _, s := range cases {
		if err := validateStatus(s); err == nil {
			t.Errorf("validateStatus(%q) = nil, want error", s)
		}
	}
}

// ----- create ---------------------------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{title: "hi"})
	if req.Data.Title != "hi" {
		t.Errorf("title = %q, want hi", req.Data.Title)
	}
	if req.Data.Owned != nil {
		t.Errorf("owned should be nil when --owned unset, got %v", *req.Data.Owned)
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

func TestBuildCreateRequestFullPayload(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{
		title:       "Task",
		description: "details",
		status:      "connected",
		owned:       true,
		ownedSet:    true,
		color:       "#ff0000",
		width:       320,
		height:      120,
		parentID:    "frame-1",
	})
	if req.Data.Description != "details" {
		t.Errorf("description = %q, want details", req.Data.Description)
	}
	if req.Data.Status != "connected" {
		t.Errorf("status = %q, want connected", req.Data.Status)
	}
	if req.Data.Owned == nil || !*req.Data.Owned {
		t.Errorf("owned = %v, want true pointer", req.Data.Owned)
	}
	if req.Style == nil || req.Style.FillColor != "#ff0000" {
		t.Errorf("style = %+v, want fillColor=#ff0000", req.Style)
	}
	if req.Geometry == nil || req.Geometry.Width != 320 || req.Geometry.Height != 120 {
		t.Errorf("geometry = %+v, want width=320 height=120", req.Geometry)
	}
	if req.Parent == nil || req.Parent.ID != "frame-1" {
		t.Errorf("parent = %+v, want id=frame-1", req.Parent)
	}
}

func TestBuildCreateRequestOwnedFalseEmitted(t *testing.T) {
	t.Parallel()
	// Explicit --owned=false should still be emitted (pointer non-nil)
	// so the API receives the value.
	req := buildCreateRequest(createFlags{title: "hi", owned: false, ownedSet: true})
	if req.Data.Owned == nil {
		t.Fatal("owned should be non-nil when --owned explicitly set")
	}
	if *req.Data.Owned {
		t.Errorf("owned = true, want false")
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
		_, _ = w.Write([]byte(`{"id":"app-1","data":{"title":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "uXjV1", title: "hi", status: "connected", color: "#00ff00", width: 320, height: 120, parentID: "frame-1"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/app_cards" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/app_cards", gotPath)
	}
	if gotBody.Data.Title != "hi" {
		t.Errorf("body title = %q, want hi", gotBody.Data.Title)
	}
	if gotBody.Data.Status != "connected" {
		t.Errorf("body status = %q, want connected", gotBody.Data.Status)
	}
	if gotBody.Style == nil || gotBody.Style.FillColor != "#00ff00" {
		t.Errorf("body style = %+v, want fillColor=#00ff00", gotBody.Style)
	}
	if gotBody.Geometry == nil || gotBody.Geometry.Width != 320 || gotBody.Geometry.Height != 120 {
		t.Errorf("body geometry = %+v, want width=320 height=120", gotBody.Geometry)
	}
	if gotBody.Parent == nil || gotBody.Parent.ID != "frame-1" {
		t.Errorf("body parent = %+v, want id=frame-1", gotBody.Parent)
	}
	if !strings.Contains(stdout.String(), `"app-1"`) {
		t.Errorf("stdout missing new app card id: %q", stdout.String())
	}
}

func TestRunCreateRejectsEmptyTitle(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err == nil {
		t.Fatal("runCreate with empty title returned nil, want error")
	}
}

func TestRunCreateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{title: "hi"}); err == nil {
		t.Fatal("runCreate with empty board ID returned nil, want error")
	}
}

func TestRunCreateRejectsInvalidStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid --status hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCreate(context.Background(), g, createFlags{boardID: "b", title: "hi", status: "active"})
	if err == nil {
		t.Fatal("runCreate with invalid status returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --status") {
		t.Errorf("error = %q, want substring 'invalid --status'", err.Error())
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
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/app_cards") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"a1","data":{"title":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "a1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/app_cards/a1" {
		t.Errorf("path = %q, want /v2/boards/b1/app_cards/a1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"hi"`) {
		t.Errorf("stdout missing title: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "a"); err == nil {
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

func TestBuildUpdateRequestOwnedFalseSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{owned: false, ownedSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with ownedSet should report ok=true")
	}
	if req.Data == nil || req.Data.Owned == nil {
		t.Fatal("data.owned should be non-nil when --owned explicitly set")
	}
	if *req.Data.Owned {
		t.Errorf("data.owned = true, want false")
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

func TestBuildUpdateRequestHeightOnly(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{height: 200, heightSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with heightSet should report ok=true")
	}
	if req.Geometry == nil {
		t.Fatal("geometry should be non-nil when --height set")
	}
	if req.Geometry.Height != 200 {
		t.Errorf("geometry.height = %v, want 200", req.Geometry.Height)
	}
	if req.Geometry.Width != 0 {
		t.Errorf("geometry.width = %v, want 0 (unset)", req.Geometry.Width)
	}
}

func TestRunUpdatePatchesAndReturnsCard(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"a1","data":{"title":"new"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "a1", title: "new", titleSet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/app_cards/a1" {
		t.Errorf("path = %q, want /v2/boards/b/app_cards/a1", gotPath)
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
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "a"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{itemID: "a", title: "x", titleSet: true}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", title: "x", titleSet: true}); err == nil {
		t.Fatal("runUpdate with empty item ID returned nil, want error")
	}
}

func TestRunUpdateRejectsInvalidStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid --status hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "a", status: "wrong", statusSet: true})
	if err == nil {
		t.Fatal("runUpdate with invalid status returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --status") {
		t.Errorf("error = %q, want substring 'invalid --status'", err.Error())
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
	err := runDelete(context.Background(), g, "b", "a")
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
	if err := runDelete(context.Background(), g, "b", "a1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/app_cards/a1" {
		t.Errorf("path = %q, want /v2/boards/b/app_cards/a1", gotPath)
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
	if err := runDelete(context.Background(), g, "b", "a"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/app_cards/a") {
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
	if err := runDelete(context.Background(), g, "b", "a"); err != nil {
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
			t.Errorf("`app-cards` parent missing subcommand %q", verb)
		}
	}
}

func TestNewCmdUseIsKebabCase(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "app-cards" {
		t.Errorf("parent Use = %q, want app-cards", cmd.Use)
	}
}

package shapes

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

// ----- create --------------------------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{shape: "rectangle"})
	if req.Data.Shape != "rectangle" {
		t.Errorf("shape = %q, want rectangle", req.Data.Shape)
	}
	if req.Style != nil {
		t.Errorf("style should be nil with no color/text-color/align flags: %+v", req.Style)
	}
	if req.Geometry != nil {
		t.Errorf("geometry should be nil with no width/height: %+v", req.Geometry)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
}

func TestBuildCreateRequestFullStyle(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{
		shape:             "circle",
		content:           "Hi",
		color:             "#ff0000",
		textColor:         "#ffffff",
		textAlign:         "center",
		textAlignVertical: "middle",
		width:             300,
		height:            150,
		parentID:          "frame-1",
	})
	if req.Style == nil {
		t.Fatal("style should be set with color flags")
	}
	if req.Style.FillColor != "#ff0000" {
		t.Errorf("style.fillColor = %q", req.Style.FillColor)
	}
	if req.Style.Color != "#ffffff" {
		t.Errorf("style.color = %q", req.Style.Color)
	}
	if req.Style.TextAlign != "center" {
		t.Errorf("style.textAlign = %q", req.Style.TextAlign)
	}
	if req.Style.TextAlignVertical != "middle" {
		t.Errorf("style.textAlignVertical = %q", req.Style.TextAlignVertical)
	}
	if req.Geometry == nil || req.Geometry.Width != 300 || req.Geometry.Height != 150 {
		t.Errorf("geometry = %+v", req.Geometry)
	}
	if req.Parent == nil || req.Parent.ID != "frame-1" {
		t.Errorf("parent = %+v", req.Parent)
	}
}

func TestRunCreateSendsBody(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody createRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"sh-1","data":{"shape":"rectangle"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", shape: "rectangle", content: "Hi"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotPath != "/v2/boards/b/shapes" {
		t.Errorf("path = %q, want /v2/boards/b/shapes", gotPath)
	}
	if gotBody.Data.Shape != "rectangle" || gotBody.Data.Content != "Hi" {
		t.Errorf("body data = %+v", gotBody.Data)
	}
}

func TestRunCreateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{shape: "rectangle"}); err == nil {
		t.Fatal("empty board ID should error")
	}
}

func TestRunCreateRejectsEmptyShape(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err == nil {
		t.Fatal("empty shape should error")
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
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", shape: "rectangle"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/shapes") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get -----------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"sh1","data":{"shape":"circle"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b", "sh1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b/shapes/sh1" {
		t.Errorf("path = %q, want /v2/boards/b/shapes/sh1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"circle"`) {
		t.Errorf("stdout missing shape: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "s"); err == nil {
		t.Fatal("empty board ID should error")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Fatal("empty item ID should error")
	}
}

func TestRunGetNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runGet(context.Background(), g, "b", "missing")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if code := miro.ExitCode(err); code != miro.ExitNotFound {
		t.Errorf("404 mapped to exit %d, want %d", code, miro.ExitNotFound)
	}
}

// ----- update --------------------------------------------------------------

func TestBuildUpdateRequestNoFields(t *testing.T) {
	t.Parallel()
	_, ok := buildUpdateRequest(updateFlags{})
	if ok {
		t.Error("buildUpdateRequest with no fields should report ok=false")
	}
}

func TestBuildUpdateRequestParentDetach(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{parentID: "", parentIDSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with parentIDSet should report ok=true")
	}
	if req.Parent == nil || req.Parent.ID != "" {
		t.Errorf("parent.id should be empty (detach): %+v", req.Parent)
	}
}

func TestBuildUpdateRequestPartialGeometry(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{width: 100, widthSet: true})
	if !ok {
		t.Fatal("widthSet should set geometry")
	}
	if req.Geometry == nil || req.Geometry.Width != 100 {
		t.Errorf("geometry = %+v", req.Geometry)
	}
}

func TestRunUpdatePatches(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	var gotBody updateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"sh1","data":{"content":"updated"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "sh1", content: "updated", contentSet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/shapes/sh1" {
		t.Errorf("path = %q, want /v2/boards/b/shapes/sh1", gotPath)
	}
	if gotBody.Data == nil || gotBody.Data.Content != "updated" {
		t.Errorf("body data = %+v", gotBody.Data)
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "s"}); err == nil {
		t.Fatal("no-field update should error")
	}
}

// ----- delete --------------------------------------------------------------

func TestRunDeleteRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDelete(context.Background(), g, "b", "s")
	if err == nil {
		t.Fatal("expected refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d", code, miro.ExitConfig)
	}
}

func TestRunDeleteWithYes(t *testing.T) {
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
	if err := runDelete(context.Background(), g, "b", "sh1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/shapes/sh1" {
		t.Errorf("path = %q, want /v2/boards/b/shapes/sh1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"deleted": true`) {
		t.Errorf("stdout missing deleted envelope: %q", stdout.String())
	}
}

// ----- registration --------------------------------------------------------

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
			t.Errorf("`shapes` parent missing subcommand %q", verb)
		}
	}
}

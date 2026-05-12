package texts

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

// ----- types ---------------------------------------------------------------

func TestFontSizeString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want string
	}{
		{0, ""},
		{-1, ""},
		{14, "14"},
		{72, "72"},
	}
	for _, c := range cases {
		if got := fontSizeString(c.in); got != c.want {
			t.Errorf("fontSizeString(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ----- create --------------------------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{content: "hi"})
	if req.Data.Content != "hi" {
		t.Errorf("content = %q, want hi", req.Data.Content)
	}
	if req.Style != nil {
		t.Errorf("style should be nil when color/font-size unset, got %+v", req.Style)
	}
	if req.Geometry != nil {
		t.Errorf("geometry should be nil when --width unset, got %+v", req.Geometry)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
}

func TestBuildCreateRequestEmitsFontSizeAsString(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{content: "hi", fontSize: 14})
	if req.Style == nil || req.Style.FontSize != "14" {
		t.Errorf("fontSize should serialize as string \"14\": %+v", req.Style)
	}
	// Round-trip through JSON to verify the wire shape.
	raw, _ := json.Marshal(req)
	if !strings.Contains(string(raw), `"fontSize":"14"`) {
		t.Errorf("wire JSON should contain quoted fontSize: %s", raw)
	}
}

func TestRunCreateSendsBody(t *testing.T) {
	t.Parallel()
	var gotPath string
	var gotBody createRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"t-1","data":{"content":"hi"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", content: "hi", fontSize: 18, width: 200}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotPath != "/v2/boards/b/texts" {
		t.Errorf("path = %q, want /v2/boards/b/texts", gotPath)
	}
	if gotBody.Style == nil || gotBody.Style.FontSize != "18" {
		t.Errorf("body style = %+v, want fontSize=18", gotBody.Style)
	}
	if gotBody.Geometry == nil || gotBody.Geometry.Width != 200 {
		t.Errorf("body geometry = %+v, want width=200", gotBody.Geometry)
	}
}

func TestRunCreateRejectsEmptyContent(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err == nil {
		t.Fatal("runCreate with empty content returned nil, want error")
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
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/texts") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get -----------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"t1","data":{"content":"hello"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b", "t1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b/texts/t1" {
		t.Errorf("path = %q, want /v2/boards/b/texts/t1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"hello"`) {
		t.Errorf("stdout missing content: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "t"); err == nil {
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

func TestRunUpdatePatches(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	var gotBody updateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"t1","data":{"content":"updated"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "t1", content: "updated", contentSet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/texts/t1" {
		t.Errorf("path = %q, want /v2/boards/b/texts/t1", gotPath)
	}
	if gotBody.Data == nil || gotBody.Data.Content != "updated" {
		t.Errorf("body data = %+v, want content=updated", gotBody.Data)
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "t"}); err == nil {
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
	err := runDelete(context.Background(), g, "b", "t")
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
	if err := runDelete(context.Background(), g, "b", "t1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/texts/t1" {
		t.Errorf("path = %q, want /v2/boards/b/texts/t1", gotPath)
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
			t.Errorf("`texts` parent missing subcommand %q", verb)
		}
	}
}

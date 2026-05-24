package tags

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

// ----- validateFillColor ----------------------------------------------------

func TestValidateFillColor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"red", false},
		{"magenta", false},
		{"violet", false},
		{"light_green", false},
		{"green", false},
		{"dark_green", false},
		{"cyan", false},
		{"blue", false},
		{"dark_blue", false},
		{"black", false},
		{"gray", false},
		{"yellow", false},
		{"RED", true},        // case-sensitive — Miro only accepts lowercase
		{"orange", true},     // not in Miro's enum
		{"light_blue", true}, // not in Miro's enum
		{"grey", true},       // British spelling not supported
		{" red", true},       // leading space
		{"red ", true},       // trailing space
	}
	for _, c := range cases {
		err := validateFillColor(c.in)
		if c.wantErr && err == nil {
			t.Errorf("validateFillColor(%q) = nil, want error", c.in)
		}
		if !c.wantErr && err != nil {
			t.Errorf("validateFillColor(%q) = %v, want nil", c.in, err)
		}
	}
}

// ----- list -----------------------------------------------------------------

func TestBuildListPathNoFlags(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1"})
	if got != "/v2/boards/b1/tags" {
		t.Errorf("buildListPath = %q, want /v2/boards/b1/tags", got)
	}
}

func TestBuildListPathWithLimitAndOffset(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1", limit: 25, offset: 50})
	// url.Values.Encode sorts keys alphabetically, so limit comes before offset.
	want := "/v2/boards/b1/tags?limit=25&offset=50"
	if got != want {
		t.Errorf("buildListPath = %q, want %q", got, want)
	}
}

func TestBuildListPathLimitOnly(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1", limit: 10})
	if got != "/v2/boards/b1/tags?limit=10" {
		t.Errorf("buildListPath = %q", got)
	}
}

func TestRunListHappyPath(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotQuery  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"id":"tag-1","title":"todo"}],"total":1}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runList(context.Background(), g, listFlags{boardID: "b1", limit: 25, offset: 50}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/boards/b1/tags" {
		t.Errorf("path = %q, want /v2/boards/b1/tags", gotPath)
	}
	if gotQuery != "limit=25&offset=50" {
		t.Errorf("query = %q, want limit=25&offset=50", gotQuery)
	}
	if !strings.Contains(stdout.String(), `"tag-1"`) {
		t.Errorf("stdout missing tag id: %q", stdout.String())
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
	if err := runList(context.Background(), g, listFlags{boardID: "b", limit: 5}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/tags?limit=5") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- create ---------------------------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{title: "to do"})
	if req.Title != "to do" {
		t.Errorf("title = %q, want to do", req.Title)
	}
	if req.FillColor != "" {
		t.Errorf("fillColor should be empty when --fill-color unset, got %q", req.FillColor)
	}
}

func TestBuildCreateRequestFullPayload(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{title: "blocker", fillColor: "red"})
	if req.Title != "blocker" {
		t.Errorf("title = %q", req.Title)
	}
	if req.FillColor != "red" {
		t.Errorf("fillColor = %q, want red", req.FillColor)
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
		_, _ = w.Write([]byte(`{"id":"tag-1","title":"to do","fillColor":"red"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{
		boardID:   "uXjV1",
		title:     "to do",
		fillColor: "red",
	}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/tags" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/tags", gotPath)
	}
	if gotBody.Title != "to do" {
		t.Errorf("body title = %q, want to do", gotBody.Title)
	}
	if gotBody.FillColor != "red" {
		t.Errorf("body fillColor = %q, want red", gotBody.FillColor)
	}
	if !strings.Contains(stdout.String(), `"tag-1"`) {
		t.Errorf("stdout missing new tag id: %q", stdout.String())
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
	if err := runCreate(context.Background(), g, createFlags{title: "todo"}); err == nil {
		t.Fatal("runCreate with empty board ID returned nil, want error")
	}
}

func TestRunCreateRejectsInvalidFillColor(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreate(context.Background(), g, createFlags{boardID: "b", title: "todo", fillColor: "orange"})
	if err == nil {
		t.Fatal("runCreate with --fill-color=orange returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --fill-color") {
		t.Errorf("error = %q, want invalid --fill-color prefix", err.Error())
	}
}

func TestRunCreateAcceptsEmptyFillColor(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"t1"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", title: "todo"}); err != nil {
		t.Fatalf("runCreate with empty fill-color: %v", err)
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
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", title: "x"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/tags") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"t1","title":"to do","fillColor":"red"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "t1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/tags/t1" {
		t.Errorf("path = %q, want /v2/boards/b1/tags/t1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"to do"`) {
		t.Errorf("stdout missing title: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "t"); err == nil {
		t.Fatal("runGet with empty board ID returned nil, want error")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runGet with empty tag ID returned nil, want error")
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
	req, ok := buildUpdateRequest(updateFlags{title: "new title", titleSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with titleSet should report ok=true")
	}
	if req.Title == nil || *req.Title != "new title" {
		t.Errorf("title = %v, want new title", req.Title)
	}
	if req.FillColor != nil {
		t.Errorf("fillColor should stay nil when --fill-color unset, got %v", req.FillColor)
	}
}

func TestBuildUpdateRequestOnlyFillColorSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{fillColor: "blue", fillColorSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with fillColorSet should report ok=true")
	}
	if req.FillColor == nil || *req.FillColor != "blue" {
		t.Errorf("fillColor = %v, want blue", req.FillColor)
	}
	if req.Title != nil {
		t.Errorf("title should stay nil when --title unset, got %v", req.Title)
	}
}

func TestBuildUpdateRequestBothFieldsSet(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{
		title:        "renamed",
		fillColor:    "green",
		titleSet:     true,
		fillColorSet: true,
	})
	if !ok {
		t.Fatal("buildUpdateRequest with both fields should report ok=true")
	}
	if req.Title == nil || *req.Title != "renamed" {
		t.Errorf("title = %v, want renamed", req.Title)
	}
	if req.FillColor == nil || *req.FillColor != "green" {
		t.Errorf("fillColor = %v, want green", req.FillColor)
	}
}

func TestBuildUpdateRequestEmptyTitleExplicit(t *testing.T) {
	t.Parallel()
	// User explicitly set --title=""; the pointer wire format lets us
	// distinguish that from "field omitted." Whether the server accepts
	// it is the server's concern; the client surface honors the user's
	// explicit choice.
	req, ok := buildUpdateRequest(updateFlags{title: "", titleSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with titleSet should report ok=true")
	}
	if req.Title == nil {
		t.Fatal("title pointer should be non-nil when --title explicitly empty")
	}
	if *req.Title != "" {
		t.Errorf("title = %q, want empty", *req.Title)
	}
}

func TestRunUpdatePatchesAndReturnsTag(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"t1","title":"renamed","fillColor":"red"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{
		boardID:  "b",
		tagID:    "t1",
		title:    "renamed",
		titleSet: true,
	}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/tags/t1" {
		t.Errorf("path = %q, want /v2/boards/b/tags/t1", gotPath)
	}
	if gotBody.Title == nil || *gotBody.Title != "renamed" {
		t.Errorf("body title = %v, want renamed", gotBody.Title)
	}
	if !strings.Contains(stdout.String(), `"renamed"`) {
		t.Errorf("stdout missing updated title: %q", stdout.String())
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", tagID: "t"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{tagID: "t", title: "x", titleSet: true}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", title: "x", titleSet: true}); err == nil {
		t.Fatal("runUpdate with empty tag ID returned nil, want error")
	}
}

func TestRunUpdateRejectsInvalidFillColor(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpdate(context.Background(), g, updateFlags{
		boardID:      "b",
		tagID:        "t",
		fillColor:    "orange",
		fillColorSet: true,
	})
	if err == nil {
		t.Fatal("runUpdate with --fill-color=orange returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --fill-color") {
		t.Errorf("error = %q, want invalid --fill-color prefix", err.Error())
	}
}

func TestRunUpdateDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runUpdate(context.Background(), g, updateFlags{
		boardID:  "b",
		tagID:    "t",
		title:    "x",
		titleSet: true,
	}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PATCH /v2/boards/b/tags/t") {
		t.Errorf("dry-run output: %q", stdout.String())
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
	err := runDelete(context.Background(), g, "b", "t")
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
	if err := runDelete(context.Background(), g, "b", "t1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/tags/t1" {
		t.Errorf("path = %q, want /v2/boards/b/tags/t1", gotPath)
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
	if err := runDelete(context.Background(), g, "b", "t"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/tags/t") {
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
	if err := runDelete(context.Background(), g, "b", "t"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

func TestRunDeleteRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Yes: true}
	if err := runDelete(context.Background(), g, "", "t"); err == nil {
		t.Fatal("runDelete with empty board ID returned nil, want error")
	}
	if err := runDelete(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runDelete with empty tag ID returned nil, want error")
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{
		"list":   false,
		"create": false,
		"get":    false,
		"update": false,
		"delete": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`tags` parent missing subcommand %q", verb)
		}
	}
}

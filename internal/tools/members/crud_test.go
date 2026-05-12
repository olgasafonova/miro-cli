package members

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

// ----- validateRole ---------------------------------------------------------

func TestValidateRoleAcceptsKnown(t *testing.T) {
	t.Parallel()
	cases := []string{"", "viewer", "commenter", "editor", "coowner", "owner", "guest"}
	for _, c := range cases {
		if err := validateRole(c); err != nil {
			t.Errorf("validateRole(%q) = %v, want nil", c, err)
		}
	}
}

func TestValidateRoleRejectsUnknown(t *testing.T) {
	t.Parallel()
	cases := []string{"admin", "VIEWER", "Owner", "guests", "read", " viewer", "viewer "}
	for _, c := range cases {
		err := validateRole(c)
		if err == nil {
			t.Errorf("validateRole(%q) = nil, want error", c)
			continue
		}
		if !strings.Contains(err.Error(), "invalid --role") {
			t.Errorf("validateRole(%q) error = %q, want invalid --role prefix", c, err.Error())
		}
	}
}

// ----- list -----------------------------------------------------------------

func TestBuildListPathNoParams(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1"})
	want := "/v2/boards/b1/members"
	if got != want {
		t.Errorf("buildListPath = %q, want %q", got, want)
	}
}

func TestBuildListPathLimitAndOffset(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1", limit: 10, offset: 20})
	// url.Values.Encode sorts keys alphabetically.
	want := "/v2/boards/b1/members?limit=10&offset=20"
	if got != want {
		t.Errorf("buildListPath = %q, want %q", got, want)
	}
}

func TestBuildListPathOffsetOnly(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1", offset: 5})
	want := "/v2/boards/b1/members?offset=5"
	if got != want {
		t.Errorf("buildListPath = %q, want %q", got, want)
	}
}

func TestBuildListPathZeroOffsetSkipsParam(t *testing.T) {
	t.Parallel()
	// offset=0 is the API default; emitting an explicit ?offset=0 is
	// noise on the wire and confuses log greps. Match items/list.go
	// behaviour: skip zero defaults.
	got := buildListPath(listFlags{boardID: "b1", limit: 10, offset: 0})
	want := "/v2/boards/b1/members?limit=10"
	if got != want {
		t.Errorf("buildListPath = %q, want %q", got, want)
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
		_, _ = w.Write([]byte(`{"data":[{"id":"u1","role":"editor"}],"total":1}`))
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
	if gotPath != "/v2/boards/b1/members" {
		t.Errorf("path = %q, want /v2/boards/b1/members", gotPath)
	}
	if gotQuery != "limit=25&offset=50" {
		t.Errorf("query = %q, want limit=25&offset=50", gotQuery)
	}
	if !strings.Contains(stdout.String(), `"u1"`) {
		t.Errorf("stdout missing member id: %q", stdout.String())
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
	if err := runList(context.Background(), g, listFlags{boardID: "b1", limit: 10}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b1/members?limit=10") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"u1","role":"editor","name":"Jane"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "u1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/members/u1" {
		t.Errorf("path = %q, want /v2/boards/b1/members/u1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"editor"`) {
		t.Errorf("stdout missing role: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "u"); err == nil {
		t.Fatal("runGet with empty board ID returned nil, want error")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runGet with empty member ID returned nil, want error")
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

func TestRunGetDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runGet(context.Background(), g, "b1", "u1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b1/members/u1") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- update ---------------------------------------------------------------

func TestBuildUpdateRequestRoleOnly(t *testing.T) {
	t.Parallel()
	req := buildUpdateRequest(updateFlags{role: "editor"})
	if req.Role != "editor" {
		t.Errorf("role = %q, want editor", req.Role)
	}
}

func TestBuildUpdateRequestEmptyRoleSerializesEmpty(t *testing.T) {
	t.Parallel()
	// runUpdate guards against empty role before calling buildUpdateRequest,
	// but the projection itself stays mechanical: empty role means the
	// omitempty tag drops the field on the wire. This keeps the test
	// asserting on the JSON shape rather than re-asserting runUpdate's
	// guard.
	req := buildUpdateRequest(updateFlags{role: ""})
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != "{}" {
		t.Errorf("empty-role JSON = %q, want {}", string(raw))
	}
}

func TestRunUpdatePatchesAndReturnsMember(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"u1","role":"coowner"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", memberID: "u1", role: "coowner"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/members/u1" {
		t.Errorf("path = %q, want /v2/boards/b/members/u1", gotPath)
	}
	if gotBody.Role != "coowner" {
		t.Errorf("body role = %q, want coowner", gotBody.Role)
	}
	if !strings.Contains(stdout.String(), `"coowner"`) {
		t.Errorf("stdout missing updated role: %q", stdout.String())
	}
}

func TestRunUpdateRejectsInvalidRole(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpdate(context.Background(), g, updateFlags{boardID: "b", memberID: "u", role: "admin"})
	if err == nil {
		t.Fatal("runUpdate with --role=admin returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --role") {
		t.Errorf("error = %q, want invalid --role prefix", err.Error())
	}
}

func TestRunUpdateRequiresRole(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", memberID: "u"}); err == nil {
		t.Fatal("runUpdate with empty role returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{memberID: "u", role: "editor"}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", role: "editor"}); err == nil {
		t.Fatal("runUpdate with empty member ID returned nil, want error")
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
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", memberID: "u", role: "owner"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PATCH /v2/boards/b/members/u") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- remove ---------------------------------------------------------------

func TestRunRemoveRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("remove without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runRemove(context.Background(), g, "b", "u")
	if err == nil {
		t.Fatal("runRemove without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
}

func TestRunRemoveWithYesCallsAPI(t *testing.T) {
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
	if err := runRemove(context.Background(), g, "b", "u1"); err != nil {
		t.Fatalf("runRemove: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/members/u1" {
		t.Errorf("path = %q, want /v2/boards/b/members/u1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"removed": true`) {
		t.Errorf("stdout missing removed envelope: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"u1"`) {
		t.Errorf("stdout missing member id: %q", stdout.String())
	}
}

func TestRunRemoveDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runRemove(context.Background(), g, "b", "u"); err != nil {
		t.Fatalf("runRemove: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/members/u") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunRemoveAgentImpliesYes(t *testing.T) {
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
	if err := runRemove(context.Background(), g, "b", "u"); err != nil {
		t.Fatalf("runRemove: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

func TestRunRemoveRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Yes: true}
	if err := runRemove(context.Background(), g, "", "u"); err == nil {
		t.Fatal("runRemove with empty board ID returned nil, want error")
	}
	if err := runRemove(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runRemove with empty member ID returned nil, want error")
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{"list": false, "get": false, "update": false, "remove": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`members` parent missing subcommand %q", verb)
		}
	}
}

func TestNewCmdParentMetadata(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "members" {
		t.Errorf("parent Use = %q, want members", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("parent Short is empty")
	}
}

package groups

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

// ----- validateItemIDs ------------------------------------------------------

func TestValidateItemIDsRejectsZero(t *testing.T) {
	t.Parallel()
	if err := validateItemIDs(nil); err == nil {
		t.Fatal("validateItemIDs(nil) returned nil, want error")
	}
}

func TestValidateItemIDsRejectsSingle(t *testing.T) {
	t.Parallel()
	err := validateItemIDs([]string{"i1"})
	if err == nil {
		t.Fatal("validateItemIDs with one item returned nil, want error")
	}
	if !strings.Contains(err.Error(), "at least twice") {
		t.Errorf("error = %q, want mention of two-item minimum", err.Error())
	}
}

func TestValidateItemIDsAcceptsTwo(t *testing.T) {
	t.Parallel()
	if err := validateItemIDs([]string{"i1", "i2"}); err != nil {
		t.Errorf("validateItemIDs([i1,i2]) = %v, want nil", err)
	}
}

func TestValidateItemIDsRejectsEmptyString(t *testing.T) {
	t.Parallel()
	err := validateItemIDs([]string{"i1", ""})
	if err == nil {
		t.Fatal("validateItemIDs with empty item returned nil, want error")
	}
}

// ----- list -----------------------------------------------------------------

func TestBuildListPathDefault(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1"})
	if got != "/v2/boards/b1/groups" {
		t.Errorf("path = %q, want /v2/boards/b1/groups", got)
	}
}

func TestBuildListPathWithLimitAndCursor(t *testing.T) {
	t.Parallel()
	got := buildListPath(listFlags{boardID: "b1", limit: 25, cursor: "abc"})
	// Order of query params is map-iteration-stable through url.Values.Encode
	// (alphabetical), so we can compare literal.
	if got != "/v2/boards/b1/groups?cursor=abc&limit=25" {
		t.Errorf("path = %q, want /v2/boards/b1/groups?cursor=abc&limit=25", got)
	}
}

func TestRunListHappyPath(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"id":"g1","type":"group"}],"cursor":""}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runList(context.Background(), g, listFlags{boardID: "b1", limit: 10}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/boards/b1/groups?limit=10" {
		t.Errorf("path = %q, want /v2/boards/b1/groups?limit=10", gotPath)
	}
	if !strings.Contains(stdout.String(), `"g1"`) {
		t.Errorf("stdout missing group id: %q", stdout.String())
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
	if err := runList(context.Background(), g, listFlags{boardID: "b"}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/groups") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- create ---------------------------------------------------------------

func TestBuildCreateRequestShape(t *testing.T) {
	t.Parallel()
	req := buildCreateRequest(createFlags{itemIDs: []string{"i1", "i2", "i3"}})
	if len(req.Data.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(req.Data.Items))
	}
	if req.Data.Items[0] != "i1" || req.Data.Items[2] != "i3" {
		t.Errorf("items = %v, want [i1 i2 i3]", req.Data.Items)
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
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"g1","type":"group","data":{"items":["i1","i2"]}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{boardID: "uXjV1", itemIDs: []string{"i1", "i2"}}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/groups" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/groups", gotPath)
	}
	if len(gotBody.Data.Items) != 2 {
		t.Errorf("body items len = %d, want 2", len(gotBody.Data.Items))
	}
	if gotBody.Data.Items[0] != "i1" || gotBody.Data.Items[1] != "i2" {
		t.Errorf("body items = %v, want [i1 i2]", gotBody.Data.Items)
	}
	if !strings.Contains(stdout.String(), `"g1"`) {
		t.Errorf("stdout missing new group id: %q", stdout.String())
	}
}

func TestRunCreateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{itemIDs: []string{"i1", "i2"}}); err == nil {
		t.Fatal("runCreate with empty board ID returned nil, want error")
	}
}

func TestRunCreateRejectsFewerThanTwoItems(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", itemIDs: []string{"i1"}}); err == nil {
		t.Fatal("runCreate with one item returned nil, want error")
	}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b"}); err == nil {
		t.Fatal("runCreate with no items returned nil, want error")
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
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", itemIDs: []string{"i1", "i2"}}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/groups") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"g1","type":"group","data":{"items":["i1","i2"]}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "g1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/groups/g1" {
		t.Errorf("path = %q, want /v2/boards/b1/groups/g1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"i1"`) {
		t.Errorf("stdout missing items: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "g"); err == nil {
		t.Fatal("runGet with empty board ID returned nil, want error")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Fatal("runGet with empty group ID returned nil, want error")
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

// ----- get-items ------------------------------------------------------------

func TestBuildGetItemsPathDefault(t *testing.T) {
	t.Parallel()
	got := buildGetItemsPath(getItemsFlags{boardID: "b1", groupID: "g1"})
	if got != "/v2/boards/b1/groups/items?group_item_id=g1" {
		t.Errorf("path = %q, want /v2/boards/b1/groups/items?group_item_id=g1", got)
	}
}

func TestBuildGetItemsPathWithLimitAndCursor(t *testing.T) {
	t.Parallel()
	got := buildGetItemsPath(getItemsFlags{boardID: "b1", groupID: "g1", limit: 20, cursor: "xyz"})
	// url.Values.Encode emits keys in alphabetical order: cursor, group_item_id, limit
	if got != "/v2/boards/b1/groups/items?cursor=xyz&group_item_id=g1&limit=20" {
		t.Errorf("path = %q, want /v2/boards/b1/groups/items?cursor=xyz&group_item_id=g1&limit=20", got)
	}
}

func TestRunGetItemsHappyPath(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotQuery  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("group_item_id")
		_, _ = w.Write([]byte(`{"data":{"id":"g1","type":"group","data":[{"id":"i1"}]},"size":1}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGetItems(context.Background(), g, getItemsFlags{boardID: "b1", groupID: "g1"}); err != nil {
		t.Fatalf("runGetItems: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/boards/b1/groups/items" {
		t.Errorf("path = %q, want /v2/boards/b1/groups/items", gotPath)
	}
	if gotQuery != "g1" {
		t.Errorf("group_item_id query = %q, want g1", gotQuery)
	}
	if !strings.Contains(stdout.String(), `"i1"`) {
		t.Errorf("stdout missing item: %q", stdout.String())
	}
}

func TestRunGetItemsRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetItems(context.Background(), g, getItemsFlags{groupID: "g"}); err == nil {
		t.Fatal("runGetItems with empty board ID returned nil, want error")
	}
	if err := runGetItems(context.Background(), g, getItemsFlags{boardID: "b"}); err == nil {
		t.Fatal("runGetItems with empty group ID returned nil, want error")
	}
}

func TestRunGetItemsDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runGetItems(context.Background(), g, getItemsFlags{boardID: "b", groupID: "g"}); err != nil {
		t.Fatalf("runGetItems: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/groups/items?group_item_id=g") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- update ---------------------------------------------------------------

func TestBuildUpdateRequestShape(t *testing.T) {
	t.Parallel()
	req := buildUpdateRequest(updateFlags{itemIDs: []string{"i1", "i2"}})
	if len(req.Data.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(req.Data.Items))
	}
	if req.Data.Items[0] != "i1" || req.Data.Items[1] != "i2" {
		t.Errorf("items = %v, want [i1 i2]", req.Data.Items)
	}
}

func TestBuildUpdateRequestRoundTrip(t *testing.T) {
	t.Parallel()
	// The wire shape is identical to create — verify the JSON encoder
	// emits the same payload create would for the same items.
	uReq := buildUpdateRequest(updateFlags{itemIDs: []string{"i1", "i2", "i3"}})
	cReq := buildCreateRequest(createFlags{itemIDs: []string{"i1", "i2", "i3"}})
	uJSON, _ := json.Marshal(uReq)
	cJSON, _ := json.Marshal(cReq)
	if string(uJSON) != string(cJSON) {
		t.Errorf("update body %s != create body %s", uJSON, cJSON)
	}
}

func TestRunUpdatePutsAndReturnsGroup(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"g2","type":"group","data":{"items":["i1","i3"]}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", groupID: "g1", itemIDs: []string{"i1", "i3"}}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/v2/boards/b/groups/g1" {
		t.Errorf("path = %q, want /v2/boards/b/groups/g1", gotPath)
	}
	if len(gotBody.Data.Items) != 2 || gotBody.Data.Items[0] != "i1" || gotBody.Data.Items[1] != "i3" {
		t.Errorf("body items = %v, want [i1 i3]", gotBody.Data.Items)
	}
	if !strings.Contains(stdout.String(), `"g2"`) {
		t.Errorf("stdout missing new group id (PUT replaces): %q", stdout.String())
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{groupID: "g", itemIDs: []string{"i1", "i2"}}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemIDs: []string{"i1", "i2"}}); err == nil {
		t.Fatal("runUpdate with empty group ID returned nil, want error")
	}
}

func TestRunUpdateRejectsFewerThanTwoItems(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", groupID: "g", itemIDs: []string{"i1"}}); err == nil {
		t.Fatal("runUpdate with one item returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", groupID: "g"}); err == nil {
		t.Fatal("runUpdate with no items returned nil, want error")
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
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", groupID: "g1", itemIDs: []string{"i1", "i2"}}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PUT /v2/boards/b/groups/g1") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- delete ---------------------------------------------------------------

func TestBuildDeletePathDefault(t *testing.T) {
	t.Parallel()
	got := buildDeletePath(deleteFlags{boardID: "b", groupID: "g"})
	if got != "/v2/boards/b/groups/g" {
		t.Errorf("path = %q, want /v2/boards/b/groups/g", got)
	}
}

func TestBuildDeletePathWithDeleteItems(t *testing.T) {
	t.Parallel()
	got := buildDeletePath(deleteFlags{boardID: "b", groupID: "g", deleteItems: true})
	if got != "/v2/boards/b/groups/g?delete_items=true" {
		t.Errorf("path = %q, want /v2/boards/b/groups/g?delete_items=true", got)
	}
}

func TestRunDeleteRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDelete(context.Background(), g, deleteFlags{boardID: "b", groupID: "g"})
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
	if err := runDelete(context.Background(), g, deleteFlags{boardID: "b", groupID: "g1"}); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/groups/g1" {
		t.Errorf("path = %q, want /v2/boards/b/groups/g1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"deleted": true`) {
		t.Errorf("stdout missing deleted envelope: %q", stdout.String())
	}
}

func TestRunDeleteWithDeleteItemsQuery(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("delete_items")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	if err := runDelete(context.Background(), g, deleteFlags{boardID: "b", groupID: "g1", deleteItems: true}); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotQuery != "true" {
		t.Errorf("delete_items query = %q, want true", gotQuery)
	}
	if !strings.Contains(stdout.String(), `"deleteItems": true`) {
		t.Errorf("stdout missing deleteItems flag: %q", stdout.String())
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
	if err := runDelete(context.Background(), g, deleteFlags{boardID: "b", groupID: "g"}); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/groups/g") {
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
	if err := runDelete(context.Background(), g, deleteFlags{boardID: "b", groupID: "g"}); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

func TestRunDeleteRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard, Yes: true}
	if err := runDelete(context.Background(), g, deleteFlags{groupID: "g"}); err == nil {
		t.Fatal("runDelete with empty board ID returned nil, want error")
	}
	if err := runDelete(context.Background(), g, deleteFlags{boardID: "b"}); err == nil {
		t.Fatal("runDelete with empty group ID returned nil, want error")
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{
		"list":      false,
		"create":    false,
		"get":       false,
		"get-items": false,
		"update":    false,
		"delete":    false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`groups` parent missing subcommand %q", verb)
		}
	}
}

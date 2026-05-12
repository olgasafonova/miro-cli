package items

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"i1","type":"shape"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "i1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/items/i1" {
		t.Errorf("path = %q, want /v2/boards/b1/items/i1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"i1"`) {
		t.Errorf("stdout missing id: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "i"); err == nil {
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

func TestRunGetDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runGet(context.Background(), g, "b", "i"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/items/i") {
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
	err := runDelete(context.Background(), g, "b", "i")
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
	if err := runDelete(context.Background(), g, "b", "i1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/items/i1" {
		t.Errorf("path = %q, want /v2/boards/b/items/i1", gotPath)
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
	if err := runDelete(context.Background(), g, "b", "i"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/items/i") {
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
	if err := runDelete(context.Background(), g, "b", "i"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
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

func TestBuildUpdateRequestXZeroExplicit(t *testing.T) {
	t.Parallel()
	req, ok := buildUpdateRequest(updateFlags{x: 0, xSet: true})
	if !ok {
		t.Fatal("buildUpdateRequest with xSet should report ok=true")
	}
	if req.Position == nil {
		t.Fatal("position should be non-nil when --x set")
	}
	if req.Position.X != 0 || req.Position.Origin != "center" {
		t.Errorf("position = %+v, want X=0 origin=center", req.Position)
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
}

func TestRunUpdatePatchesAndReturnsItem(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"i1","position":{"x":10,"y":20}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "i1", x: 10, xSet: true, y: 20, ySet: true}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/items/i1" {
		t.Errorf("path = %q, want /v2/boards/b/items/i1", gotPath)
	}
	if gotBody.Position == nil || gotBody.Position.X != 10 || gotBody.Position.Y != 20 {
		t.Errorf("body position = %+v, want X=10 Y=20", gotBody.Position)
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", itemID: "i"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{itemID: "i", xSet: true}); err == nil {
		t.Fatal("runUpdate with empty board ID returned nil, want error")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", xSet: true}); err == nil {
		t.Fatal("runUpdate with empty item ID returned nil, want error")
	}
}

// ----- get-by-tag -----------------------------------------------------------

func TestBuildGetByTagPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   getByTagFlags
		want string
	}{
		{
			name: "minimal",
			in:   getByTagFlags{boardID: "b", tagID: "tag-1"},
			want: "/v2/boards/b/items?tag_id=tag-1",
		},
		{
			name: "with limit + offset",
			in:   getByTagFlags{boardID: "b", tagID: "tag-1", limit: 25, offset: 50},
			want: "/v2/boards/b/items?limit=25&offset=50&tag_id=tag-1",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := buildGetByTagPath(c.in); got != c.want {
				t.Errorf("path = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRunGetByTagHappyPath(t *testing.T) {
	t.Parallel()
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"data":[{"id":"i1"}],"total":1}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGetByTag(context.Background(), g, getByTagFlags{boardID: "b", tagID: "tag-1", limit: 10}); err != nil {
		t.Fatalf("runGetByTag: %v", err)
	}
	if gotURI != "/v2/boards/b/items?limit=10&tag_id=tag-1" {
		t.Errorf("URI = %q", gotURI)
	}
}

func TestRunGetByTagRequiresFlags(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetByTag(context.Background(), g, getByTagFlags{tagID: "t"}); err == nil {
		t.Error("missing --board-id should error")
	}
	if err := runGetByTag(context.Background(), g, getByTagFlags{boardID: "b"}); err == nil {
		t.Error("missing --tag-id should error")
	}
}

// ----- get-within-frame -----------------------------------------------------

func TestBuildGetWithinFramePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   getWithinFrameFlags
		want string
	}{
		{
			name: "minimal",
			in:   getWithinFrameFlags{boardID: "b", frameID: "f1"},
			want: "/v2/boards/b/items?parent_item_id=f1",
		},
		{
			name: "with type + cursor",
			in:   getWithinFrameFlags{boardID: "b", frameID: "f1", itemType: "sticky_note", cursor: "c1", limit: 20},
			want: "/v2/boards/b/items?cursor=c1&limit=20&parent_item_id=f1&type=sticky_note",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := buildGetWithinFramePath(c.in); got != c.want {
				t.Errorf("path = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRunGetWithinFrameHappyPath(t *testing.T) {
	t.Parallel()
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGetWithinFrame(context.Background(), g, getWithinFrameFlags{boardID: "b", frameID: "f1"}); err != nil {
		t.Fatalf("runGetWithinFrame: %v", err)
	}
	if gotURI != "/v2/boards/b/items?parent_item_id=f1" {
		t.Errorf("URI = %q", gotURI)
	}
}

func TestRunGetWithinFrameRequiresFlags(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetWithinFrame(context.Background(), g, getWithinFrameFlags{frameID: "f"}); err == nil {
		t.Error("missing --board-id should error")
	}
	if err := runGetWithinFrame(context.Background(), g, getWithinFrameFlags{boardID: "b"}); err == nil {
		t.Error("missing --frame-id should error")
	}
}

// ----- bulk-create ----------------------------------------------------------

func TestLoadBulkItemsFromJSON(t *testing.T) {
	t.Parallel()
	items, err := loadBulkItems(bulkCreateFlags{itemsJSON: `[{"type":"sticky_note"},{"type":"shape"}]`})
	if err != nil {
		t.Fatalf("loadBulkItems: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}
}

func TestLoadBulkItemsFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "items.json")
	if err := os.WriteFile(p, []byte(`[{"type":"text"}]`), 0o600); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}
	items, err := loadBulkItems(bulkCreateFlags{itemsFile: p})
	if err != nil {
		t.Fatalf("loadBulkItems: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("len = %d, want 1", len(items))
	}
}

func TestLoadBulkItemsRejectsBothFlags(t *testing.T) {
	t.Parallel()
	_, err := loadBulkItems(bulkCreateFlags{itemsFile: "x", itemsJSON: `[]`})
	if err == nil {
		t.Error("both flags set should error")
	}
}

func TestLoadBulkItemsRejectsNeitherFlag(t *testing.T) {
	t.Parallel()
	if _, err := loadBulkItems(bulkCreateFlags{}); err == nil {
		t.Error("no flags set should error")
	}
}

func TestLoadBulkItemsRejectsEmptyArray(t *testing.T) {
	t.Parallel()
	if _, err := loadBulkItems(bulkCreateFlags{itemsJSON: `[]`}); err == nil {
		t.Error("empty array should error")
	}
}

func TestLoadBulkItemsRejectsNonArray(t *testing.T) {
	t.Parallel()
	if _, err := loadBulkItems(bulkCreateFlags{itemsJSON: `{"type":"x"}`}); err == nil {
		t.Error("non-array JSON should error")
	}
}

func TestRunBulkCreateSendsArray(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"data":[{"id":"a"},{"id":"b"}]}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runBulkCreate(context.Background(), g, bulkCreateFlags{
		boardID:   "b",
		itemsJSON: `[{"type":"sticky_note"},{"type":"shape"}]`,
	})
	if err != nil {
		t.Fatalf("runBulkCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/b/items/bulk" {
		t.Errorf("path = %q, want /v2/boards/b/items/bulk", gotPath)
	}
	if len(gotBody) != 2 || gotBody[0]["type"] != "sticky_note" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestRunBulkCreateDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runBulkCreate(context.Background(), g, bulkCreateFlags{boardID: "b", itemsJSON: `[{"type":"x"}]`}); err != nil {
		t.Fatalf("runBulkCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/items/bulk") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- attach-tag -----------------------------------------------------------

func TestRunAttachTagHappyPath(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotURI    string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURI = r.URL.RequestURI()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runAttachTag(context.Background(), g, "b", "i1", "tag-1"); err != nil {
		t.Fatalf("runAttachTag: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotURI != "/v2/boards/b/items/i1?tag_id=tag-1" {
		t.Errorf("URI = %q", gotURI)
	}
	if !strings.Contains(stdout.String(), `"attached": true`) {
		t.Errorf("stdout missing attached envelope: %q", stdout.String())
	}
}

func TestRunAttachTagDoesNotRequireYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Note: --yes is NOT set; attach-tag is non-destructive.
	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runAttachTag(context.Background(), g, "b", "i", "t"); err != nil {
		t.Fatalf("runAttachTag without --yes: %v", err)
	}
}

func TestRunAttachTagRejectsEmpty(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runAttachTag(context.Background(), g, "", "i", "t"); err == nil {
		t.Error("empty board ID should error")
	}
	if err := runAttachTag(context.Background(), g, "b", "", "t"); err == nil {
		t.Error("empty item ID should error")
	}
	if err := runAttachTag(context.Background(), g, "b", "i", ""); err == nil {
		t.Error("empty tag ID should error")
	}
}

func TestRunAttachTagDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runAttachTag(context.Background(), g, "b", "i", "t"); err != nil {
		t.Fatalf("runAttachTag: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/items/i?tag_id=t") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- detach-tag -----------------------------------------------------------

func TestRunDetachTagRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("detach-tag without --yes hit the API")
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDetachTag(context.Background(), g, "b", "i", "t")
	if err == nil {
		t.Fatal("detach-tag without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
}

func TestRunDetachTagWithYesCallsAPI(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotURI    string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURI = r.URL.RequestURI()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	if err := runDetachTag(context.Background(), g, "b", "i1", "tag-1"); err != nil {
		t.Fatalf("runDetachTag: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotURI != "/v2/boards/b/items/i1?tag_id=tag-1" {
		t.Errorf("URI = %q", gotURI)
	}
	if !strings.Contains(stdout.String(), `"detached": true`) {
		t.Errorf("stdout missing detached envelope: %q", stdout.String())
	}
}

func TestRunDetachTagDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runDetachTag(context.Background(), g, "b", "i", "t"); err != nil {
		t.Fatalf("runDetachTag: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/items/i?tag_id=t") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get-tags -------------------------------------------------------------

func TestRunGetTagsHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"tags":[{"id":"t1","title":"red"}]}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGetTags(context.Background(), g, "b", "i1"); err != nil {
		t.Fatalf("runGetTags: %v", err)
	}
	if gotPath != "/v2/boards/b/items/i1/tags" {
		t.Errorf("path = %q, want /v2/boards/b/items/i1/tags", gotPath)
	}
	if !strings.Contains(stdout.String(), `"red"`) {
		t.Errorf("stdout missing tag title: %q", stdout.String())
	}
}

func TestRunGetTagsRejectsEmpty(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetTags(context.Background(), g, "", "i"); err == nil {
		t.Error("empty board ID should error")
	}
	if err := runGetTags(context.Background(), g, "b", ""); err == nil {
		t.Error("empty item ID should error")
	}
}

// ----- list-all -------------------------------------------------------------

func TestRunListAllPaginatesAndEmitsEnvelope(t *testing.T) {
	t.Parallel()
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"1"},{"id":"2"}],"cursor":"next"}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":[{"id":"3"}],"cursor":""}`))
		default:
			t.Errorf("extra request: page %d", page)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runListAll(context.Background(), g, listAllFlags{boardID: "b"}); err != nil {
		t.Fatalf("runListAll: %v", err)
	}
	var got listAllResponse
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if got.Total != 3 || len(got.Items) != 3 {
		t.Errorf("total=%d items=%d, want 3 / 3", got.Total, len(got.Items))
	}
	if got.Truncated {
		t.Error("truncated should be false under cap")
	}
}

func TestRunListAllRequiresBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runListAll(context.Background(), g, listAllFlags{}); err == nil {
		t.Error("missing --board-id should error")
	}
}

func TestRunListAllDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runListAll(context.Background(), g, listAllFlags{boardID: "b", itemType: "shape"}); err != nil {
		t.Fatalf("runListAll: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/items?type=shape (paginated)") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllPhase3CVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{
		"list":             false,
		"get":              false,
		"delete":           false,
		"update":           false,
		"get-by-tag":       false,
		"get-within-frame": false,
		"bulk-create":      false,
		"attach-tag":       false,
		"detach-tag":       false,
		"get-tags":         false,
		"list-all":         false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`items` parent missing subcommand %q", verb)
		}
	}
}

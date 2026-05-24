package items

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func TestLoadPatchesExclusivity(t *testing.T) {
	t.Parallel()
	if _, err := loadPatches(bulkUpdateFlags{}); err == nil {
		t.Error("loadPatches with no flags returned nil")
	}
	if _, err := loadPatches(bulkUpdateFlags{patchesFile: "/x", patchesJSON: "[]"}); err == nil {
		t.Error("loadPatches with both flags returned nil")
	}
}

func TestLoadPatchesParsesArray(t *testing.T) {
	t.Parallel()
	in := `[{"id":"a","x":10,"y":20},{"id":"b","parent_id":""}]`
	got, err := loadPatches(bulkUpdateFlags{patchesJSON: in})
	if err != nil {
		t.Fatalf("loadPatches: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[0].X == nil || *got[0].X != 10 {
		t.Errorf("patches[0] = %+v", got[0])
	}
	if got[1].ParentID == nil || *got[1].ParentID != "" {
		t.Errorf("patches[1].ParentID should be non-nil empty string for detach, got %+v", got[1].ParentID)
	}
}

func TestLoadPatchesRejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := loadPatches(bulkUpdateFlags{patchesJSON: `[]`}); err == nil {
		t.Error("loadPatches with empty array returned nil")
	}
}

func TestBuildBulkUpdateBodyNoFields(t *testing.T) {
	t.Parallel()
	_, ok := buildBulkUpdateBody(bulkUpdateItem{ID: "a"})
	if ok {
		t.Error("buildBulkUpdateBody with no fields returned ok=true")
	}
}

func TestBuildBulkUpdateBodyPositionOnly(t *testing.T) {
	t.Parallel()
	x := 1.5
	req, ok := buildBulkUpdateBody(bulkUpdateItem{ID: "a", X: &x})
	if !ok {
		t.Fatal("expected ok=true with X set")
	}
	if req.Position == nil || req.Position.X != 1.5 || req.Position.Origin != "center" {
		t.Errorf("position = %+v", req.Position)
	}
	if req.Geometry != nil || req.Parent != nil {
		t.Errorf("geometry/parent should be nil: g=%+v p=%+v", req.Geometry, req.Parent)
	}
}

func TestBuildBulkUpdateBodyDetachParent(t *testing.T) {
	t.Parallel()
	empty := ""
	req, ok := buildBulkUpdateBody(bulkUpdateItem{ID: "a", ParentID: &empty})
	if !ok {
		t.Fatal("expected ok=true with empty parent_id (detach)")
	}
	if req.Parent == nil || req.Parent.ID != "" {
		t.Errorf("parent = %+v, want envelope with empty id", req.Parent)
	}
}

func TestRunBulkUpdateDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API")
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		DryRun: true,
	}
	err := runBulkUpdate(context.Background(), g, bulkUpdateFlags{
		boardID:     "b1",
		patchesJSON: `[{"id":"a","x":1},{"id":"b","y":2}]`,
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PATCH /v2/boards/b1/items/{item_id} x 2") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunBulkUpdateHappyPath(t *testing.T) {
	type req struct {
		path string
		body json.RawMessage
	}
	var (
		mu   sync.Mutex
		seen []req
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("server saw %s, want PATCH", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, req{path: r.URL.Path, body: raw})
		mu.Unlock()
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	err := runBulkUpdate(context.Background(), g, bulkUpdateFlags{
		boardID:     "b1",
		patchesJSON: `[{"id":"a","x":100,"y":50},{"id":"b","parent_id":""}]`,
	})
	if err != nil {
		t.Fatalf("runBulkUpdate: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("server saw %d requests, want 2", len(seen))
	}
	if seen[0].path != "/v2/boards/b1/items/a" {
		t.Errorf("seen[0].path = %q", seen[0].path)
	}
	if !strings.Contains(string(seen[0].body), `"x":100`) || !strings.Contains(string(seen[0].body), `"origin":"center"`) {
		t.Errorf("seen[0].body = %s", seen[0].body)
	}
	// b's body should have parent envelope with empty id (detach)
	if !strings.Contains(string(seen[1].body), `"parent":{"id":""}`) {
		t.Errorf("seen[1].body = %s, want parent.id=\"\"", seen[1].body)
	}

	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Requested != 2 || out.Succeeded != 2 || out.Failed != 0 {
		t.Errorf("summary = %+v", out)
	}
}

func TestRunBulkUpdateMissingIDIsPerItemError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	err := runBulkUpdate(context.Background(), g, bulkUpdateFlags{
		boardID:     "b1",
		patchesJSON: `[{"id":"","x":1},{"id":"b","x":2}]`,
	})
	if err != nil {
		t.Fatalf("runBulkUpdate: %v", err)
	}
	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Failed != 1 || out.Succeeded != 1 {
		t.Errorf("summary = %+v, want 1 failed + 1 succeeded", out)
	}
	if out.Results[0].Status != "error" || !strings.Contains(out.Results[0].Error, "missing") {
		t.Errorf("results[0] = %+v", out.Results[0])
	}
}

func TestRunBulkUpdateEmptyPatchIsPerItemError(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	err := runBulkUpdate(context.Background(), g, bulkUpdateFlags{
		boardID:     "b1",
		patchesJSON: `[{"id":"a"},{"id":"b","x":10}]`,
	})
	if err != nil {
		t.Fatalf("runBulkUpdate: %v", err)
	}
	if requests != 1 {
		t.Errorf("server got %d requests, want 1 (empty patch should be pre-flight-rejected)", requests)
	}
	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Succeeded != 1 || out.Failed != 1 {
		t.Errorf("summary = %+v", out)
	}
}

func TestRunBulkUpdatePartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/bad") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"bad request"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	err := runBulkUpdate(context.Background(), g, bulkUpdateFlags{
		boardID:     "b1",
		patchesJSON: `[{"id":"ok","x":1},{"id":"bad","x":2},{"id":"ok2","x":3}]`,
	})
	if err != nil {
		t.Fatalf("runBulkUpdate: %v", err)
	}
	var out bulkOpResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Succeeded != 2 || out.Failed != 1 {
		t.Errorf("summary = %+v", out)
	}
	if out.Results[1].Status != "error" || out.Results[1].ID != "bad" {
		t.Errorf("results[1] = %+v", out.Results[1])
	}
}

func TestRunBulkUpdateRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runBulkUpdate(context.Background(), g, bulkUpdateFlags{patchesJSON: `[{"id":"a","x":1}]`}); err == nil {
		t.Fatal("runBulkUpdate with empty board ID returned nil")
	}
}

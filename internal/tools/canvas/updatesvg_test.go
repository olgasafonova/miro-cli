package canvas

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

// recordedCall captures one API request the diff issued.
type recordedCall struct {
	Method string
	Path   string
	Body   map[string]any
}

// serveRecorder returns a server that records every call and answers
// each with a fresh id.
func serveRecorder(calls *[]recordedCall) *httptest.Server {
	var n int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		*calls = append(*calls, recordedCall{Method: r.Method, Path: r.URL.Path, Body: body})
		n++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "new-" + string(rune('0'+n))})
	}))
}

// decodeUpdateSVG unmarshals the emitted update-from-svg envelope.
func decodeUpdateSVG(t *testing.T, stdout *bytes.Buffer) updateSVGResult {
	t.Helper()
	var out updateSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	return out
}

// assertRoutingEnvelope checks the emitted diff envelope: one update,
// one delete, one additive create, no failures.
func assertRoutingEnvelope(t *testing.T, out updateSVGResult) {
	t.Helper()
	if len(out.Updated) != 1 {
		t.Fatalf("updated = %+v, want exactly u1", out.Updated)
	}
	if out.Updated[0].ID != "u1" {
		t.Errorf("updated id = %q, want u1", out.Updated[0].ID)
	}
	if len(out.Deleted) != 1 {
		t.Fatalf("deleted = %+v, want exactly d1", out.Deleted)
	}
	if len(out.Created) != 1 {
		t.Fatalf("created = %+v, want one shape", out.Created)
	}
	if len(out.Failed) != 0 {
		t.Errorf("failed = %+v, want none", out.Failed)
	}
}

// assertPatchBody checks the PATCH restated geometry as a unit with the
// center recomputed from the top-left rect, plus the fill mapping.
func assertPatchBody(t *testing.T, patch recordedCall) {
	t.Helper()
	if patch.Path != "/v2/boards/abc/items/u1" {
		t.Errorf("PATCH path = %q, want /v2/boards/abc/items/u1", patch.Path)
	}
	position, _ := patch.Body["position"].(map[string]any)
	if position["x"] != 10.0 {
		t.Errorf("PATCH position = %v, want center x=10", position)
	}
	if position["y"] != 5.0 {
		t.Errorf("PATCH position = %v, want center y=5", position)
	}
	geometry, _ := patch.Body["geometry"].(map[string]any)
	if geometry["width"] != 20.0 {
		t.Errorf("PATCH geometry = %v, want width 20", geometry)
	}
	if geometry["height"] != 10.0 {
		t.Errorf("PATCH geometry = %v, want height 10", geometry)
	}
	style, _ := patch.Body["style"].(map[string]any)
	if style["fillColor"] != "#ff0000" {
		t.Errorf("PATCH style = %v, want fillColor #ff0000", style)
	}
}

func TestRunUpdateFromSVG_RoutesUpdateDeleteCreate(t *testing.T) {
	var calls []recordedCall
	srv := serveRecorder(&calls)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{
		boardID: "abc",
		svg: `<svg>
			<rect data-miro-id="u1" x="0" y="0" width="20" height="10" fill="#ff0000"/>
			<rect data-miro-id="d1" data-deleted="true" x="0" y="0" width="5" height="5"/>
			<rect x="50" y="50" width="10" height="10"/>
		</svg>`,
	})
	if err != nil {
		t.Fatalf("runUpdateFromSVG: %v", err)
	}
	assertRoutingEnvelope(t, decodeUpdateSVG(t, stdout))

	byMethod := map[string]recordedCall{}
	for _, c := range calls {
		byMethod[c.Method] = c
	}
	assertPatchBody(t, byMethod[http.MethodPatch])
	if byMethod[http.MethodDelete].Path != "/v2/boards/abc/items/d1" {
		t.Errorf("DELETE path = %q, want /v2/boards/abc/items/d1", byMethod[http.MethodDelete].Path)
	}
	if !strings.HasSuffix(byMethod[http.MethodPost].Path, "/shapes") {
		t.Errorf("POST path = %q, want a /shapes create", byMethod[http.MethodPost].Path)
	}
}

func TestRunUpdateFromSVG_SemanticErrorsLandInFailed(t *testing.T) {
	var calls []recordedCall
	srv := serveRecorder(&calls)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{
		boardID: "abc",
		svg: `<svg>
			<line data-miro-id="c1" data-start="a" data-end="b"/>
			<rect data-miro-id="u1" x="0" y="0" width="20" height="10"/>
		</svg>`,
	})
	if err != nil {
		t.Fatalf("runUpdateFromSVG: %v", err)
	}
	out := decodeUpdateSVG(t, stdout)

	// The connector update fails semantically; the rect still applies.
	if len(out.Failed) != 1 {
		t.Fatalf("failed = %+v, want exactly the c1 connector rejection", out.Failed)
	}
	if out.Failed[0].ID != "c1" {
		t.Errorf("failed id = %q, want c1", out.Failed[0].ID)
	}
	if !strings.Contains(out.Failed[0].Reason, "connector") {
		t.Errorf("failure reason = %q, want it to name connectors", out.Failed[0].Reason)
	}
	if len(out.Updated) != 1 {
		t.Fatalf("updated = %+v, want u1 despite the failed line", out.Updated)
	}
	if out.Updated[0].ID != "u1" {
		t.Errorf("updated id = %q, want u1", out.Updated[0].ID)
	}
}

func TestRunUpdateFromSVG_TextRestatesAnchorOnly(t *testing.T) {
	var calls []recordedCall
	srv := serveRecorder(&calls)
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{
		boardID: "abc",
		svg:     `<svg><text data-miro-id="t1" x="30" y="40">new words</text></svg>`,
	})
	if err != nil {
		t.Fatalf("runUpdateFromSVG: %v", err)
	}
	if len(calls) != 1 || calls[0].Method != http.MethodPatch {
		t.Fatalf("calls = %+v, want one PATCH", calls)
	}
	body := calls[0].Body
	if _, hasGeometry := body["geometry"]; hasGeometry {
		t.Error("text update restated geometry; Miro sizes text by content")
	}
	data, _ := body["data"].(map[string]any)
	if data["content"] != "new words" {
		t.Errorf("data = %v, want content 'new words'", data)
	}
}

func TestRunUpdateFromSVG_MalformedXMLFailsWhole(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("malformed SVG hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{
		boardID: "abc",
		svg:     `<svg><rect data-miro-id="u1"`,
	})
	if err == nil {
		t.Fatal("malformed SVG accepted")
	}
}

func TestRunUpdateFromSVG_DryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{
		boardID: "abc",
		svg:     `<svg><rect data-miro-id="u1" x="0" y="0" width="5" height="5"/></svg>`,
	})
	if err != nil {
		t.Fatalf("runUpdateFromSVG: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunUpdateFromSVG_EmptyBoardIDIsUsageError(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{svg: "<svg/>"})
	if err == nil || !strings.Contains(err.Error(), "board_id") {
		t.Fatalf("empty --board-id: err = %v, want board_id usage error", err)
	}
}

// TestReadSVGOutputIsResubmittable is the round-trip contract the diff
// verb exists for: feed read-svg's rendered document straight into the
// update path and every identified element must route to an update, not
// a create.
func TestReadSVGOutputIsResubmittable(t *testing.T) {
	srv := serveBoard(svgItemsPayload(), emptyListing)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	if err := runReadSVG(context.Background(), g, readSVGFlags{boardID: "abc"}); err != nil {
		t.Fatalf("render: %v", err)
	}
	rendered := decodeReadSVG(t, stdout)

	elements, _, err := parseSVGElements(rendered.SVG)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	for _, el := range elements {
		if el.name != "text" && el.miroID == "" {
			t.Errorf("rendered %s carries no data-miro-id; re-submitting would create a duplicate", el.name)
		}
	}
}

// TestRunUpdateFromSVG_MinimalDeletionMarkers pins the two fixes the
// live smoke test forced (18-08-2026): a deletion marker needs no
// geometry or content, and a line deletion routes to /connectors
// because the generic items DELETE answers 404 for connectors.
func TestRunUpdateFromSVG_MinimalDeletionMarkers(t *testing.T) {
	var calls []recordedCall
	srv := serveRecorder(&calls)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runUpdateFromSVG(context.Background(), g, updateSVGFlags{
		boardID: "abc",
		svg: `<svg>
			<rect data-miro-id="r1" data-deleted="true"/>
			<text data-miro-id="t1" data-deleted="true"></text>
			<line data-miro-id="c1" data-deleted="true"/>
		</svg>`,
	})
	if err != nil {
		t.Fatalf("runUpdateFromSVG: %v", err)
	}
	out := decodeUpdateSVG(t, stdout)
	if len(out.Deleted) != 3 {
		t.Fatalf("deleted = %+v, want r1 t1 c1: %s", out.Deleted, stdout.String())
	}
	paths := map[string]bool{}
	for _, c := range calls {
		if c.Method == http.MethodDelete {
			paths[c.Path] = true
		}
	}
	if !paths["/v2/boards/abc/items/r1"] || !paths["/v2/boards/abc/items/t1"] {
		t.Errorf("DELETE paths = %v, want items routes for r1 and t1", paths)
	}
	if !paths["/v2/boards/abc/connectors/c1"] {
		t.Errorf("DELETE paths = %v, want the connectors route for c1", paths)
	}
}

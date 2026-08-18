package canvas

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// newTestGlobals wires a Globals at the given httptest server with a
// stdout buffer for output assertions.
func newTestGlobals(srvURL string) (*clictx.Globals, *bytes.Buffer) {
	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srvURL})}
	return g, &stdout
}

// svgItemsPayload is the items-listing fixture for the read direction:
// a frame, a sticky, and an embed with no drawable geometry.
func svgItemsPayload() map[string]any {
	return map[string]any{
		"data": []map[string]any{
			{
				"id": "f1", "type": "frame",
				"position": map[string]any{"x": 100.0, "y": 100.0},
				"geometry": map[string]any{"width": 400.0, "height": 300.0},
				"data":     map[string]any{"title": "Sprint"},
			},
			{
				"id": "s1", "type": "sticky_note",
				"position": map[string]any{"x": 50.0, "y": 60.0},
				"geometry": map[string]any{"width": 100.0, "height": 100.0},
				"data":     map[string]any{"content": "<p>Do the thing</p>"},
				"style":    map[string]any{"fillColor": "light_yellow"},
			},
			{
				"id": "e1", "type": "embed",
				"position": map[string]any{"x": 0.0, "y": 0.0},
			},
		},
		"cursor": "",
	}
}

// serveBoard returns an httptest server answering the items and
// connectors listings with the given payloads.
func serveBoard(items map[string]any, connectors map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/connectors") {
			_ = json.NewEncoder(w).Encode(connectors)
			return
		}
		_ = json.NewEncoder(w).Encode(items)
	}))
}

var emptyListing = map[string]any{"data": []any{}, "cursor": ""}

// decodeReadSVG unmarshals the emitted read-svg envelope.
func decodeReadSVG(t *testing.T, stdout *bytes.Buffer) readSVGResult {
	t.Helper()
	var out readSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	return out
}

func TestRunReadSVG_RendersAndCounts(t *testing.T) {
	srv := serveBoard(svgItemsPayload(), emptyListing)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	if err := runReadSVG(context.Background(), g, readSVGFlags{boardID: "abc"}); err != nil {
		t.Fatalf("runReadSVG: %v", err)
	}
	out := decodeReadSVG(t, stdout)
	if out.ItemCount != 2 {
		t.Errorf("item_count = %d, want 2 (frame + sticky)", out.ItemCount)
	}
	if out.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the embed)", out.Skipped)
	}
	if !strings.Contains(out.SVG, `data-miro-id="s1"`) {
		t.Error("sticky's data-miro-id missing from SVG")
	}
	if !strings.Contains(out.SVG, "Do the thing") {
		t.Error("sticky label missing from SVG")
	}
	if strings.Contains(out.SVG, "<p>") {
		t.Error("HTML fragments leaked into SVG text")
	}
	// The document must be well-formed XML.
	if err := xml.Unmarshal([]byte(out.SVG), new(any)); err != nil {
		t.Errorf("rendered SVG is not well-formed XML: %v", err)
	}
	// Frame must be drawn before the sticky so it sits underneath.
	if strings.Index(out.SVG, `data-miro-id="f1"`) > strings.Index(out.SVG, `data-miro-id="s1"`) {
		t.Error("frame rendered after items; z-order wrong")
	}
}

func TestRunReadSVG_EscapesContent(t *testing.T) {
	srv := serveBoard(map[string]any{
		"data": []map[string]any{{
			"id": "t1", "type": "text",
			"position": map[string]any{"x": 0.0, "y": 0.0},
			"data":     map[string]any{"content": `a < b & "c"`},
		}},
		"cursor": "",
	}, emptyListing)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	if err := runReadSVG(context.Background(), g, readSVGFlags{boardID: "abc"}); err != nil {
		t.Fatalf("runReadSVG: %v", err)
	}
	out := decodeReadSVG(t, stdout)
	if err := xml.Unmarshal([]byte(out.SVG), new(any)); err != nil {
		t.Errorf("special characters broke the XML: %v\n%s", err, out.SVG)
	}
}

func TestRunReadSVG_DrawsConnectors(t *testing.T) {
	srv := serveBoard(map[string]any{
		"data": []map[string]any{
			{
				"id": "a", "type": "shape",
				"position": map[string]any{"x": 0.0, "y": 0.0},
				"geometry": map[string]any{"width": 10.0, "height": 10.0},
			},
			{
				"id": "b", "type": "shape",
				"position": map[string]any{"x": 100.0, "y": 100.0},
				"geometry": map[string]any{"width": 10.0, "height": 10.0},
			},
		},
		"cursor": "",
	}, map[string]any{
		"data": []map[string]any{{
			"id":        "conn1",
			"startItem": map[string]any{"id": "a"},
			"endItem":   map[string]any{"id": "b"},
			"captions":  []any{map[string]any{"content": "flows to"}},
		}},
		"cursor": "",
	})
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	if err := runReadSVG(context.Background(), g, readSVGFlags{boardID: "abc"}); err != nil {
		t.Fatalf("runReadSVG: %v", err)
	}
	out := decodeReadSVG(t, stdout)
	if !strings.Contains(out.SVG, `data-miro-id="conn1"`) {
		t.Error("connector line missing from SVG")
	}
	if !strings.Contains(out.SVG, "flows to") {
		t.Error("connector caption missing from SVG")
	}
}

func TestRunReadSVG_DryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runReadSVG(context.Background(), g, readSVGFlags{boardID: "abc"}); err != nil {
		t.Fatalf("runReadSVG: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/abc/items") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunReadSVG_EmptyBoardIDIsUsageError(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runReadSVG(context.Background(), g, readSVGFlags{})
	if err == nil || !strings.Contains(err.Error(), "board_id") {
		t.Fatalf("empty --board-id: err = %v, want board_id usage error", err)
	}
}

func TestRunCreateFromSVG_CreatesShapesAndText(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "item-1", "data": map[string]any{"shape": "rectangle"},
		})
	}))
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{
		boardID: "abc",
		svg:     `<svg><rect x="0" y="0" width="20" height="20"/><text x="5" y="5">hi</text></svg>`,
	})
	if err != nil {
		t.Fatalf("runCreateFromSVG: %v", err)
	}
	var out createSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Count != 2 {
		t.Errorf("count = %d, want 2", out.Count)
	}
	if out.Created[0].Element != "rect" || out.Created[0].Type != "shape" {
		t.Errorf("first created = %+v, want rect->shape", out.Created[0])
	}
	if out.Created[1].Element != "text" || out.Created[1].Type != "text" {
		t.Errorf("second created = %+v, want text->text", out.Created[1])
	}
	joined := strings.Join(paths, " ")
	if !strings.Contains(joined, "/shapes") || !strings.Contains(joined, "/texts") {
		t.Errorf("API paths hit = %v, want shapes and texts endpoints", paths)
	}
}

func TestRunCreateFromSVG_ElementCapRejects(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString("<svg>")
	for range maxSVGCreateElements + 1 {
		sb.WriteString(`<rect x="0" y="0" width="5" height="5"/>`)
	}
	sb.WriteString("</svg>")

	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{boardID: "abc", svg: sb.String()})
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Errorf("oversized SVG accepted or wrong error: %v", err)
	}
}

func TestRunCreateFromSVG_EmptyAndNoDrawables(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateFromSVG(context.Background(), g, createSVGFlags{boardID: "abc", svg: "  "}); err == nil {
		t.Error("blank SVG accepted")
	}

	var stdout bytes.Buffer
	g2 := &clictx.Globals{Stdout: &stdout}
	err := runCreateFromSVG(context.Background(), g2, createSVGFlags{boardID: "abc", svg: `<svg><path d="M0 0"/></svg>`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out createSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Count != 0 || out.Message == "" || len(out.Skipped) != 1 {
		t.Errorf("no-drawables result = %+v, want count 0, a message, and the path in skipped", out)
	}
}

func TestRunCreateFromSVG_PartialFailureNamesCreated(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "boom"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "item-1"})
	}))
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{
		boardID: "abc",
		svg:     `<svg><rect x="0" y="0" width="5" height="5"/><rect x="10" y="10" width="5" height="5"/></svg>`,
	})
	if err == nil {
		t.Fatal("expected error from second create")
	}
	var out createSVGResult
	if jsonErr := json.Unmarshal(stdout.Bytes(), &out); jsonErr != nil {
		t.Fatalf("partial envelope not emitted: %v\n%s", jsonErr, stdout.String())
	}
	if len(out.Created) != 1 || out.Created[0].ID != "item-1" {
		t.Errorf("partial result loses created IDs: %+v", out.Created)
	}
}

func TestRunCreateFromSVG_DryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{
		boardID: "abc",
		svg:     `<svg><rect x="0" y="0" width="5" height="5"/></svg>`,
	})
	if err != nil {
		t.Fatalf("runCreateFromSVG: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/abc/{shapes,texts,sticky_notes,frames,images,connectors} × 1 elements") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunCreateFromSVG_SVGFileFromDisk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "item-1"})
	}))
	defer srv.Close()

	path := t.TempDir() + "/doc.svg"
	if err := os.WriteFile(path, []byte(`<svg><rect x="0" y="0" width="5" height="5"/></svg>`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	g, stdout := newTestGlobals(srv.URL)
	if err := runCreateFromSVG(context.Background(), g, createSVGFlags{boardID: "abc", svgFile: path}); err != nil {
		t.Fatalf("runCreateFromSVG: %v", err)
	}
	var out createSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Count != 1 {
		t.Errorf("count = %d, want 1", out.Count)
	}
}

func TestRunCreateFromSVG_OffsetsApplied(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "item-1"})
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{
		boardID: "abc",
		svg:     `<svg><rect x="0" y="0" width="20" height="20"/></svg>`,
		offsetX: 100, offsetY: 50,
	})
	if err != nil {
		t.Fatalf("runCreateFromSVG: %v", err)
	}
	position, _ := gotBody["position"].(map[string]any)
	if position["x"] != 110.0 || position["y"] != 60.0 {
		t.Errorf("position = %v, want x=110 y=60 (center 10,10 + offset 100,50)", position)
	}
}

// TestSVGRoundTrip feeds runReadSVG's output into parseSVGElements:
// every rendered shape must come back out, proving read and create
// speak the same dialect. The second shape carries its kind in
// data.shape — the wire location — not style.
func TestSVGRoundTrip(t *testing.T) {
	srv := serveBoard(map[string]any{
		"data": []map[string]any{
			{
				"id": "sh1", "type": "shape",
				"position": map[string]any{"x": 100.0, "y": 200.0},
				"geometry": map[string]any{"width": 80.0, "height": 40.0},
				"style":    map[string]any{"fillColor": "#00ff00"},
			},
			{
				"id": "sh2", "type": "shape",
				"position": map[string]any{"x": 300.0, "y": 300.0},
				"geometry": map[string]any{"width": 60.0, "height": 60.0},
				"data":     map[string]any{"shape": "circle"},
			},
		},
		"cursor": "",
	}, emptyListing)
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
	var rect, ellipse *svgElement
	for i := range elements {
		switch elements[i].name {
		case "rect":
			rect = &elements[i]
		case "ellipse":
			ellipse = &elements[i]
		}
	}
	if rect == nil || ellipse == nil {
		t.Fatalf("round trip lost shapes; got %+v", elements)
	}
	if rect.x != 100 || rect.y != 200 || rect.w != 80 || rect.h != 40 {
		t.Errorf("rect round trip = %+v, want center (100,200) 80x40", rect)
	}
	if ellipse.x != 300 || ellipse.y != 300 || ellipse.w != 60 {
		t.Errorf("ellipse round trip = %+v, want center (300,300) d=60", ellipse)
	}
}

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "canvas" {
		t.Errorf("Use = %q, want canvas", cmd.Use)
	}
	got := map[string]bool{}
	for _, sub := range cmd.Commands() {
		got[sub.Name()] = true
	}
	for _, want := range []string{"read-svg", "create-from-svg", "update-from-svg"} {
		if !got[want] {
			t.Errorf("canvas parent did not register %q", want)
		}
	}
}

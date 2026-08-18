package canvas

// Tests for the extended create dialect (data-type hints, polygon,
// image, line connectors) and the frame-scoped read — the surface
// ported from miro-mcp-server's SVG round trip.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSVGElements_DataTypeHints(t *testing.T) {
	t.Parallel()
	elements, skipped, err := parseSVGElements(`<svg>
		<rect data-type="sticky" data-content="do it" x="0" y="0" width="100" height="100" fill="light_yellow"/>
		<rect data-type="frame" data-title="Sprint" x="0" y="0" width="400" height="300"/>
		<rect data-type="banana" x="0" y="0" width="10" height="10"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(elements) != 2 {
		t.Fatalf("elements = %d, want 2 (sticky + frame; banana skipped)", len(elements))
	}
	if elements[0].dataType != "sticky" || elements[0].text != "do it" {
		t.Errorf("sticky = %+v, want dataType sticky with data-content", elements[0])
	}
	if elements[1].dataType != "frame" || elements[1].title != "Sprint" {
		t.Errorf("frame = %+v, want dataType frame with data-title", elements[1])
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0].Reason, "banana") {
		t.Errorf("skipped = %+v, want the banana data-type named", skipped)
	}
}

func TestParseSVGElements_TrianglePolygonAndImage(t *testing.T) {
	t.Parallel()
	elements, skipped, err := parseSVGElements(`<svg>
		<polygon points="0,10 10,10 5,0" fill="#ff0000"/>
		<image href="https://example.com/pic.png" x="20" y="20" width="40" height="30"/>
		<image x="0" y="0" width="40" height="30"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(elements) != 2 {
		t.Fatalf("elements = %d, want 2 (triangle + image; hrefless image skipped)", len(elements))
	}
	tri := elements[0]
	if tri.name != "polygon" || tri.x != 5 || tri.y != 5 || tri.w != 10 || tri.h != 10 {
		t.Errorf("triangle = %+v, want bounding box center (5,5) 10x10", tri)
	}
	img := elements[1]
	if img.name != "image" || img.href != "https://example.com/pic.png" || img.x != 40 {
		t.Errorf("image = %+v, want href and center x=40", img)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0].Reason, "href") {
		t.Errorf("skipped = %+v, want the hrefless image named", skipped)
	}
}

func TestParseSVGElements_IdentityStamping(t *testing.T) {
	t.Parallel()
	elements, _, err := parseSVGElements(`<svg>
		<rect id="a" data-miro-id="m1" data-deleted="true" x="0" y="0" width="5" height="5"/>
		<line data-start="a" data-end="b" data-caption="flows"/>
	</svg>`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(elements) != 2 {
		t.Fatalf("elements = %d, want 2", len(elements))
	}
	rect := elements[0]
	if rect.authoredID != "a" || rect.miroID != "m1" || !rect.deleted {
		t.Errorf("rect identity = %+v, want id=a miroID=m1 deleted", rect)
	}
	line := elements[1]
	if line.start != "a" || line.end != "b" || line.text != "flows" {
		t.Errorf("line = %+v, want start=a end=b caption", line)
	}
}

func TestRunCreateFromSVG_TypedEndpointsAndConnector(t *testing.T) {
	var calls []recordedCall
	srv := serveRecorder(&calls)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	// The line appears BEFORE its targets in document order; the
	// two-pass create must still resolve it.
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{
		boardID: "abc",
		svg: `<svg>
			<line data-start="s" data-end="f" data-caption="belongs to"/>
			<rect id="s" data-type="sticky" data-content="task" x="0" y="0" width="100" height="100"/>
			<rect id="f" data-type="frame" data-title="Sprint" x="200" y="0" width="400" height="300"/>
			<image href="https://example.com/pic.png" x="20" y="20" width="40" height="30"/>
		</svg>`,
	})
	if err != nil {
		t.Fatalf("runCreateFromSVG: %v", err)
	}
	var out createSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Count != 4 {
		t.Fatalf("count = %d, want 4 (sticky, frame, image, connector): %+v", out.Count, out)
	}

	var paths []string
	for _, c := range calls {
		paths = append(paths, c.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range []string{"/sticky_notes", "/frames", "/images", "/connectors"} {
		if !strings.Contains(joined, want) {
			t.Errorf("API paths %v missing %s", paths, want)
		}
	}
	// Connector last (second pass), referencing ids created this call.
	last := calls[len(calls)-1]
	if !strings.HasSuffix(last.Path, "/connectors") {
		t.Fatalf("last call = %q, want the connector (second pass)", last.Path)
	}
	startItem, _ := last.Body["startItem"].(map[string]any)
	if startItem["id"] == "" || startItem["id"] == "s" {
		t.Errorf("connector startItem = %v, want a created board id, not the authored id", startItem)
	}
}

func TestRunCreateFromSVG_UnresolvedConnectorIsSkip(t *testing.T) {
	var calls []recordedCall
	srv := serveRecorder(&calls)
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runCreateFromSVG(context.Background(), g, createSVGFlags{
		boardID: "abc",
		svg: `<svg>
			<rect id="s" x="0" y="0" width="10" height="10"/>
			<line data-start="s" data-end="ghost"/>
		</svg>`,
	})
	if err != nil {
		t.Fatalf("runCreateFromSVG: %v", err)
	}
	var out createSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Count != 1 {
		t.Errorf("count = %d, want 1 (the rect; connector skipped)", out.Count)
	}
	found := false
	for _, s := range out.Skipped {
		if s.Element == "line" && strings.Contains(s.Reason, "ghost") {
			found = true
		}
	}
	if !found {
		t.Errorf("skipped = %+v, want the unresolved line naming its references", out.Skipped)
	}
}

// serveFrame answers the frame get and its children listing.
func serveFrame(children []map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/frames/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "f1",
				"data":     map[string]any{"title": "Sprint"},
				"geometry": map[string]any{"width": 400.0, "height": 300.0},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": children, "cursor": ""})
	}))
}

func TestRunReadSVG_FrameScoped(t *testing.T) {
	srv := serveFrame([]map[string]any{{
		"id": "s1", "type": "sticky_note",
		"position": map[string]any{"x": 50.0, "y": 60.0},
		"geometry": map[string]any{"width": 100.0, "height": 100.0},
		"data":     map[string]any{"content": "child"},
	}})
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	if err := runReadSVG(context.Background(), g, readSVGFlags{boardID: "abc", frameID: "f1"}); err != nil {
		t.Fatalf("runReadSVG: %v", err)
	}
	out := decodeReadSVG(t, stdout)

	if out.ItemCount != 1 {
		t.Errorf("item_count = %d, want 1 child", out.ItemCount)
	}
	// Frame outline at origin with the frame's own geometry.
	if !strings.Contains(out.SVG, `<rect x="0" y="0" width="400.0" height="300.0"`) {
		t.Errorf("frame outline not at origin:\n%s", out.SVG)
	}
	if !strings.Contains(out.SVG, `data-miro-id="f1"`) || !strings.Contains(out.SVG, `data-miro-id="s1"`) {
		t.Error("frame or child data-miro-id missing")
	}
	if !strings.Contains(out.Message, "relative to the frame's top-left") {
		t.Errorf("message = %q, want the frame-relative caveat", out.Message)
	}
}

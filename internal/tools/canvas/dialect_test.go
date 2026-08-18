package canvas

// Tests for the extended create dialect (data-type hints, polygon,
// image, line connectors) and the frame-scoped read — the surface
// ported from miro-mcp-server's SVG round trip.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// parseElements parses the document, failing the test on a parse error
// or an unexpected drawable count.
func parseElements(t *testing.T, svg string, wantCount int) ([]svgElement, []skippedElement) {
	t.Helper()
	elements, skipped, err := parseSVGElements(svg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(elements) != wantCount {
		t.Fatalf("elements = %d, want %d (skipped: %+v)", len(elements), wantCount, skipped)
	}
	return elements, skipped
}

// assertElement compares one parsed element against its expected value
// in full; svgElement is comparable, so this pins every field at once.
func assertElement(t *testing.T, label string, got, want svgElement) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %+v, want %+v", label, got, want)
	}
}

// assertSingleSkip checks that exactly one element was skipped and its
// reason names the expected cause.
func assertSingleSkip(t *testing.T, skipped []skippedElement, wantInReason string) {
	t.Helper()
	if len(skipped) != 1 {
		t.Fatalf("skipped = %+v, want exactly one entry", skipped)
	}
	if !strings.Contains(skipped[0].Reason, wantInReason) {
		t.Errorf("skip reason = %q, want it to name %q", skipped[0].Reason, wantInReason)
	}
}

func TestParseSVGElements_DataTypeHints(t *testing.T) {
	t.Parallel()
	elements, skipped := parseElements(t, `<svg>
		<rect data-type="sticky" data-content="do it" x="0" y="0" width="100" height="100" fill="light_yellow"/>
		<rect data-type="frame" data-title="Sprint" x="0" y="0" width="400" height="300"/>
		<rect data-type="banana" x="0" y="0" width="10" height="10"/>
	</svg>`, 2)
	assertElement(t, "sticky", elements[0], svgElement{
		name: "rect", x: 50, y: 50, w: 100, h: 100,
		fill: "light_yellow", dataType: "sticky", text: "do it",
	})
	assertElement(t, "frame", elements[1], svgElement{
		name: "rect", x: 200, y: 150, w: 400, h: 300,
		dataType: "frame", title: "Sprint",
	})
	assertSingleSkip(t, skipped, "banana")
}

func TestParseSVGElements_TrianglePolygonAndImage(t *testing.T) {
	t.Parallel()
	elements, skipped := parseElements(t, `<svg>
		<polygon points="0,10 10,10 5,0" fill="#ff0000"/>
		<image href="https://example.com/pic.png" x="20" y="20" width="40" height="30"/>
		<image x="0" y="0" width="40" height="30"/>
	</svg>`, 2)
	assertElement(t, "triangle", elements[0], svgElement{
		name: "polygon", x: 5, y: 5, w: 10, h: 10, fill: "#ff0000",
	})
	assertElement(t, "image", elements[1], svgElement{
		name: "image", x: 40, y: 35, w: 40, h: 30,
		href: "https://example.com/pic.png",
	})
	assertSingleSkip(t, skipped, "href")
}

func TestParseSVGElements_IdentityStamping(t *testing.T) {
	t.Parallel()
	elements, _ := parseElements(t, `<svg>
		<rect id="a" data-miro-id="m1" data-deleted="true" x="0" y="0" width="5" height="5"/>
		<line data-start="a" data-end="b" data-caption="flows"/>
	</svg>`, 2)
	assertElement(t, "rect identity", elements[0], svgElement{
		name: "rect", x: 2.5, y: 2.5, w: 5, h: 5,
		authoredID: "a", miroID: "m1", deleted: true,
	})
	assertElement(t, "line", elements[1], svgElement{
		name: "line", start: "a", end: "b", text: "flows",
	})
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
	decodeCreateResult(t, stdout, 4)
	assertPathsHit(t, calls, "/sticky_notes", "/frames", "/images", "/connectors")
	assertConnectorSecondPass(t, calls)
}

// decodeCreateResult unmarshals the create envelope and checks the count.
func decodeCreateResult(t *testing.T, stdout *bytes.Buffer, wantCount int) createSVGResult {
	t.Helper()
	var out createSVGResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if out.Count != wantCount {
		t.Fatalf("count = %d, want %d: %+v", out.Count, wantCount, out)
	}
	return out
}

// assertPathsHit checks that every wanted endpoint appears among the
// recorded API calls.
func assertPathsHit(t *testing.T, calls []recordedCall, wants ...string) {
	t.Helper()
	var paths []string
	for _, c := range calls {
		paths = append(paths, c.Path)
	}
	joined := strings.Join(paths, " ")
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("API paths %v missing %s", paths, want)
		}
	}
}

// assertConnectorSecondPass checks the connector was created last and
// references a created board id rather than the authored SVG id.
func assertConnectorSecondPass(t *testing.T, calls []recordedCall) {
	t.Helper()
	last := calls[len(calls)-1]
	if !strings.HasSuffix(last.Path, "/connectors") {
		t.Fatalf("last call = %q, want the connector (second pass)", last.Path)
	}
	startItem, _ := last.Body["startItem"].(map[string]any)
	id, _ := startItem["id"].(string)
	if id == "" {
		t.Errorf("connector startItem = %v, want a created board id", startItem)
	}
	if id == "s" {
		t.Errorf("connector startItem still carries the authored id %q", id)
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
	out := decodeCreateResult(t, stdout, 1)
	assertSingleSkip(t, out.Skipped, "ghost")
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

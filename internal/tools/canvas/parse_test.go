package canvas

import (
	"testing"
)

func TestParseSVGElements_SupportedSubset(t *testing.T) {
	t.Parallel()
	svg := `<svg xmlns="http://www.w3.org/2000/svg">
		<rect x="10" y="20" width="100" height="50" fill="#ff0000"/>
		<rect x="0" y="0" width="40" height="40" rx="8"/>
		<circle cx="200" cy="100" r="30" fill="blue"/>
		<ellipse cx="300" cy="150" rx="40" ry="20"/>
		<text x="50" y="60">Hello board</text>
	</svg>`

	elements, skipped, err := parseSVGElements(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %+v, want none", skipped)
	}
	if len(elements) != 5 {
		t.Fatalf("got %d elements, want 5", len(elements))
	}

	// rect x/y are top-left; Miro coordinates are centers.
	r := elements[0]
	if r.x != 60 || r.y != 45 {
		t.Errorf("rect center = (%v,%v), want (60,45)", r.x, r.y)
	}
	if r.fill != "#ff0000" {
		t.Errorf("rect fill = %q, want '#ff0000'", r.fill)
	}
	if elements[1].rounded != true {
		t.Error("rx>0 rect not marked rounded")
	}
	c := elements[2]
	if c.x != 200 || c.y != 100 || c.w != 60 || c.h != 60 {
		t.Errorf("circle = %+v, want center (200,100) size 60x60", c)
	}
	txt := elements[4]
	if txt.text != "Hello board" {
		t.Errorf("text content = %q, want 'Hello board'", txt.text)
	}
}

func TestParseSVGElements_NestedTranslate(t *testing.T) {
	t.Parallel()
	svg := `<svg><g transform="translate(100, 50)"><g transform="translate(10,5)">
		<rect x="0" y="0" width="20" height="20"/>
	</g></g></svg>`

	elements, _, err := parseSVGElements(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("got %d elements, want 1", len(elements))
	}
	// center = 0+10 (half size) + 100+10 offsets, 0+10 + 50+5
	if elements[0].x != 120 || elements[0].y != 65 {
		t.Errorf("nested translate center = (%v,%v), want (120,65)", elements[0].x, elements[0].y)
	}
}

func TestParseSVGElements_UnsupportedAreReported(t *testing.T) {
	t.Parallel()
	svg := `<svg>
		<path d="M0 0 L10 10"/>
		<polygon points="0,0 10,0 5,10"/>
		<line x1="0" y1="0" x2="10" y2="10"/>
		<rect x="0" y="0" width="10" height="10"/>
	</svg>`

	elements, skipped, err := parseSVGElements(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 1 {
		t.Errorf("got %d elements, want 1 (the rect)", len(elements))
	}
	if len(skipped) != 3 {
		t.Fatalf("skipped = %d, want 3 (path, polygon, line)", len(skipped))
	}
	names := map[string]bool{}
	for _, s := range skipped {
		if s.Reason == "" {
			t.Errorf("skipped %q has empty reason", s.Element)
		}
		names[s.Element] = true
	}
	for _, want := range []string{"path", "polygon", "line"} {
		if !names[want] {
			t.Errorf("skip report missing %q", want)
		}
	}
}

func TestParseSVGElements_DegenerateGeometry(t *testing.T) {
	t.Parallel()
	svg := `<svg>
		<rect x="0" y="0" width="0" height="10"/>
		<circle cx="1" cy="1" r="0"/>
		<text x="0" y="0">   </text>
	</svg>`

	elements, skipped, err := parseSVGElements(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 0 {
		t.Errorf("degenerate elements created: %+v", elements)
	}
	if len(skipped) != 3 {
		t.Errorf("skipped = %d, want 3", len(skipped))
	}
}

func TestParseSVGElements_UnsupportedTransformReported(t *testing.T) {
	t.Parallel()
	svg := `<svg><g transform="rotate(45)"><rect x="0" y="0" width="10" height="10"/></g></svg>`

	elements, skipped, err := parseSVGElements(svg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("got %d elements, want 1 (child placed untransformed)", len(elements))
	}
	if len(skipped) != 1 || skipped[0].Element != "g" {
		t.Errorf("skipped = %+v, want the g with the rotate transform", skipped)
	}
}

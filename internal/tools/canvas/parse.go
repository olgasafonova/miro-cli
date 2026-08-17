package canvas

// SVG -> board (local parsing; item creation lives in createsvg.go).
//
// Parses a constrained SVG subset into drawable elements:
//   rect            -> shape "rectangle"   (rx>0 -> "round_rectangle")
//   circle/ellipse  -> shape "circle"
//   text            -> text item
//   g transform=translate(x,y) -> offset applied to children (nesting ok)
// Everything else (path, polygon, line, image, ...) is reported per
// element in the skip list rather than silently dropped. SVG y-down
// matches Miro y-down; SVG rect x/y are top-left while Miro positions
// are centers, so the converter recenters.

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// maxSVGDocumentBytes bounds the accepted SVG source.
const maxSVGDocumentBytes = 1 << 20 // 1 MiB

// maxSVGCreateElements bounds how many items one call may create.
const maxSVGCreateElements = 200

// translateRe extracts translate(x[,y]) from a transform attribute.
var translateRe = regexp.MustCompile(`translate\(\s*(-?[\d.]+)\s*[,\s]\s*(-?[\d.]+)\s*\)|translate\(\s*(-?[\d.]+)\s*\)`)

// svgElement is one drawable element pulled out of the document.
type svgElement struct {
	name    string  // rect | circle | ellipse | text
	x, y    float64 // Miro center coordinates (after transform + recentering)
	w, h    float64
	rounded bool
	fill    string
	text    string
}

// skippedElement records one element the parser could not map, with the
// reason. Exported field names because it lands in the JSON envelope.
type skippedElement struct {
	Element string `json:"element"`
	Reason  string `json:"reason"`
}

// parseTranslate reads a transform attribute; only translate is
// honored, anything else returns ok=false so the caller can report the
// element.
func parseTranslate(transform string) (svgOffset, bool) {
	if strings.TrimSpace(transform) == "" {
		return svgOffset{}, true
	}
	m := translateRe.FindStringSubmatch(transform)
	if m == nil {
		return svgOffset{}, false
	}
	if m[3] != "" { // single-argument form
		dx, _ := strconv.ParseFloat(m[3], 64)
		return svgOffset{dx: dx}, true
	}
	dx, _ := strconv.ParseFloat(m[1], 64)
	dy, _ := strconv.ParseFloat(m[2], 64)
	return svgOffset{dx: dx, dy: dy}, true
}

// svgAttrs is one element's attribute set, keyed by local name.
type svgAttrs map[string]string

// attrMap flattens the element attributes for lookup.
func attrMap(attrs []xml.Attr) svgAttrs {
	m := make(svgAttrs, len(attrs))
	for _, a := range attrs {
		m[a.Name.Local] = a.Value
	}
	return m
}

// float parses a numeric attribute, tolerating a trailing "px".
func (a svgAttrs) float(key string) float64 {
	v := strings.TrimSuffix(strings.TrimSpace(a[key]), "px")
	f, _ := strconv.ParseFloat(v, 64)
	return f
}

// svgOffset is a cumulative translation applied to nested elements.
type svgOffset struct{ dx, dy float64 }

// plus returns the sum of two offsets.
func (o svgOffset) plus(other svgOffset) svgOffset {
	return svgOffset{o.dx + other.dx, o.dy + other.dy}
}

// svgParseState carries the offset stack and accumulators through the
// token walk.
type svgParseState struct {
	offsets  []svgOffset
	elements []svgElement
	skipped  []skippedElement
	textBuf  *svgElement // non-nil while inside <text>
}

func (st *svgParseState) offset() svgOffset {
	var total svgOffset
	for _, o := range st.offsets {
		total = total.plus(o)
	}
	return total
}

func (st *svgParseState) skip(name, reason string) {
	st.skipped = append(st.skipped, skippedElement{Element: name, Reason: reason})
}

// pushGroup handles an opening <g>, honoring translate transforms only.
func (st *svgParseState) pushGroup(attrs svgAttrs) {
	off, ok := parseTranslate(attrs["transform"])
	if !ok {
		st.skip("g", "unsupported transform (only translate is honored); children are placed untransformed")
		off = svgOffset{}
	}
	st.offsets = append(st.offsets, off)
}

// addDrawable appends a parsed element, or records a skip when its
// geometry is degenerate. The shared exit point for every shape
// converter.
func (st *svgParseState) addDrawable(el svgElement, valid bool, reason string) {
	if !valid {
		st.skip(el.name, reason)
		return
	}
	st.elements = append(st.elements, el)
}

// addRect converts a <rect> to a centered element.
func (st *svgParseState) addRect(attrs svgAttrs, off svgOffset) {
	w, h := attrs.float("width"), attrs.float("height")
	st.addDrawable(svgElement{
		name: "rect",
		x:    attrs.float("x") + w/2 + off.dx,
		y:    attrs.float("y") + h/2 + off.dy,
		w:    w, h: h,
		rounded: attrs.float("rx") > 0,
		fill:    attrs["fill"],
	}, w > 0 && h > 0, "zero or missing width/height")
}

// svgRadii reads the radius pair for a circle (r) or ellipse (rx/ry).
func svgRadii(name string, attrs svgAttrs) (rx, ry float64) {
	if name == "circle" {
		r := attrs.float("r")
		return r, r
	}
	return attrs.float("rx"), attrs.float("ry")
}

// addEllipseKind converts a <circle> or <ellipse>; the two differ only
// in how their radii are spelled.
func (st *svgParseState) addEllipseKind(name string, attrs svgAttrs, off svgOffset) {
	rx, ry := svgRadii(name, attrs)
	st.addDrawable(svgElement{
		name: name,
		x:    attrs.float("cx") + off.dx,
		y:    attrs.float("cy") + off.dy,
		w:    2 * rx, h: 2 * ry,
		fill: attrs["fill"],
	}, rx > 0 && ry > 0, "zero or missing radius")
}

// startElement dispatches one opening tag to its element handler.
func (st *svgParseState) startElement(tok xml.StartElement) {
	attrs := attrMap(tok.Attr)
	off := st.offset()

	switch tok.Name.Local {
	case "svg", "title", "desc", "defs":
		// structural; nothing to draw
	case "g":
		st.pushGroup(attrs)
	case "rect":
		st.addRect(attrs, off)
	case "circle", "ellipse":
		st.addEllipseKind(tok.Name.Local, attrs, off)
	case "text":
		st.textBuf = &svgElement{
			name: "text",
			x:    attrs.float("x") + off.dx,
			y:    attrs.float("y") + off.dy,
		}
	default:
		st.skip(tok.Name.Local, "unsupported element")
	}
}

// closeGroup pops the offset stack for a closing </g>.
func (st *svgParseState) closeGroup() {
	if len(st.offsets) > 0 {
		st.offsets = st.offsets[:len(st.offsets)-1]
	}
}

// closeText finalizes a </text>, keeping non-blank content only.
func (st *svgParseState) closeText() {
	if st.textBuf == nil {
		return
	}
	if s := strings.TrimSpace(st.textBuf.text); s != "" {
		st.textBuf.text = s
		st.elements = append(st.elements, *st.textBuf)
	} else {
		st.skip("text", "empty content")
	}
	st.textBuf = nil
}

// endElement dispatches one closing tag.
func (st *svgParseState) endElement(tok xml.EndElement) {
	switch tok.Name.Local {
	case "g":
		st.closeGroup()
	case "text":
		st.closeText()
	}
}

// handleToken routes one XML token to the matching state handler.
func (st *svgParseState) handleToken(tok xml.Token) {
	switch t := tok.(type) {
	case xml.StartElement:
		st.startElement(t)
	case xml.CharData:
		if st.textBuf != nil {
			st.textBuf.text += string(t)
		}
	case xml.EndElement:
		st.endElement(t)
	}
}

// parseSVGElements walks the document and returns drawable elements
// plus the skip report.
func parseSVGElements(svgSource string) ([]svgElement, []skippedElement, error) {
	dec := xml.NewDecoder(strings.NewReader(svgSource))
	// SVG documents commonly carry entity-free straightforward XML;
	// anything exotic fails the parse and surfaces as an error rather
	// than a guess.
	dec.Strict = false

	st := &svgParseState{}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return st.elements, st.skipped, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse SVG: %w", err)
		}
		st.handleToken(tok)
	}
}

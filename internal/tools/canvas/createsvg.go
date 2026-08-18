package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// createSVGFlags captures the per-invocation knobs for `miro canvas
// create-from-svg`. Exactly one of svg / svgFile supplies the document.
type createSVGFlags struct {
	boardID string
	svg     string
	svgFile string
	offsetX float64
	offsetY float64
}

// createdItem names one board item the verb created and which SVG
// element produced it.
type createdItem struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Element string `json:"element"`
}

// createSVGResult is the JSON envelope `canvas create-from-svg` emits.
// On a mid-batch failure the partial envelope is still emitted so the
// caller has the IDs that already landed.
type createSVGResult struct {
	Created []createdItem    `json:"created"`
	Skipped []skippedElement `json:"skipped,omitempty"`
	Count   int              `json:"count"`
	Message string           `json:"message"`
}

func newCreateFromSVGCmd(g *clictx.Globals) *cobra.Command {
	var f createSVGFlags
	cmd := &cobra.Command{
		Use:   "create-from-svg",
		Short: "Create board items from an SVG document",
		Long: "Parses a constrained SVG subset and creates matching items via\n" +
			"the typed create endpoints:\n\n" +
			"  rect            -> shape rectangle (rx>0 -> round_rectangle)\n" +
			"  rect data-type=\"sticky\" -> sticky note (data-content -> text)\n" +
			"  rect data-type=\"frame\"  -> frame (data-title -> title)\n" +
			"  circle/ellipse  -> shape circle\n" +
			"  text            -> text item\n" +
			"  polygon (3 pts) -> shape triangle (bounding box)\n" +
			"  image href=URL  -> image item\n" +
			"  line data-start/data-end -> connector between element ids\n" +
			"  g translate(x,y)-> offset applied to children (nesting ok)\n\n" +
			"Lines resolve their references against the id attributes of\n" +
			"elements created in the same call (items first, connectors\n" +
			"second). Unsupported elements (path, multi-point polygon, ...)\n" +
			"are reported in `skipped`, never silently dropped. Supply the\n" +
			"document with --svg-file (use \"-\" for stdin) or inline via\n" +
			fmt.Sprintf("--svg. The cap is %d drawable elements per call.", maxSVGCreateElements),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateFromSVG(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.svg, "svg", "", "SVG document as an inline string")
	cmd.Flags().StringVar(&f.svgFile, "svg-file", "", "Path to an SVG file, or \"-\" for stdin")
	cmd.Flags().Float64Var(&f.offsetX, "offset-x", 0, "X offset added to every created item")
	cmd.Flags().Float64Var(&f.offsetY, "offset-y", 0, "Y offset added to every created item")
	_ = cmd.MarkFlagRequired("board-id")
	cmd.MarkFlagsMutuallyExclusive("svg", "svg-file")
	cmd.MarkFlagsOneRequired("svg", "svg-file")
	return cmd
}

// loadSVGSource resolves the document from --svg or --svg-file.
func loadSVGSource(f createSVGFlags) (string, error) {
	if f.svg != "" {
		return f.svg, nil
	}
	raw, err := clictx.ReadFileOrStdin(f.svgFile)
	if err != nil {
		return "", fmt.Errorf("read --svg-file: %w", err)
	}
	return string(raw), nil
}

// validateSVGSource checks the request bounds before any parsing.
func validateSVGSource(svg string) error {
	if strings.TrimSpace(svg) == "" {
		return errors.New("the SVG document is empty")
	}
	if len(svg) > maxSVGDocumentBytes {
		return fmt.Errorf("svg exceeds %d bytes (got %d)", maxSVGDocumentBytes, len(svg))
	}
	return nil
}

// svgPlan carries a parsed, bounds-checked document into the create
// loop: the offset that applies plus the drawable and skipped elements.
type svgPlan struct {
	boardID  string
	off      svgOffset
	elements []svgElement
	skipped  []skippedElement
}

// loadAndParseSVG resolves the document from the flags, bounds-checks
// it, and parses it into a plan.
func loadAndParseSVG(f createSVGFlags) (svgPlan, error) {
	svg, err := loadSVGSource(f)
	if err != nil {
		return svgPlan{}, err
	}
	if err := validateSVGSource(svg); err != nil {
		return svgPlan{}, err
	}
	elements, skipped, err := parseSVGElements(svg)
	if err != nil {
		return svgPlan{}, err
	}
	if len(elements) > maxSVGCreateElements {
		return svgPlan{}, fmt.Errorf("svg contains %d drawable elements; the cap is %d per call", len(elements), maxSVGCreateElements)
	}
	return svgPlan{
		boardID:  f.boardID,
		off:      svgOffset{dx: f.offsetX, dy: f.offsetY},
		elements: elements,
		skipped:  skipped,
	}, nil
}

func runCreateFromSVG(ctx context.Context, g *clictx.Globals, f createSVGFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	plan, err := loadAndParseSVG(f)
	if err != nil {
		return err
	}
	if g.DryRun {
		return g.EmitDryRun("POST",
			fmt.Sprintf("/v2/boards/%s/{shapes,texts,sticky_notes,frames,images,connectors} × %d elements", f.boardID, len(plan.elements)))
	}
	if len(plan.elements) == 0 {
		return g.EmitJSON(createSVGResult{
			Created: []createdItem{},
			Skipped: plan.skipped,
			Message: "No supported elements found in the SVG (supported: rect, circle, ellipse, text, polygon, image, line)",
		})
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	run, err := createSVGElements(ctx, client, plan)
	if err != nil {
		_ = g.EmitJSON(createSVGResult{
			Created: run.created,
			Skipped: append(plan.skipped, run.skipped...),
			Count:   len(run.created),
			Message: fmt.Sprintf("Partial failure after %d item(s); see error", len(run.created)),
		})
		return fmt.Errorf("create from svg failed after %d item(s): %w", len(run.created), err)
	}

	allSkipped := append(plan.skipped, run.skipped...)
	return g.EmitJSON(createSVGResult{
		Created: run.created,
		Skipped: allSkipped,
		Count:   len(run.created),
		Message: fmt.Sprintf("Created %d item(s) from SVG (%d element(s) skipped)", len(run.created), len(allSkipped)),
	})
}

// svgItemType names the Miro item type a parsed non-line element maps to.
func svgItemType(el svgElement) string {
	switch {
	case el.name == "text":
		return "text"
	case el.dataType == "sticky":
		return "sticky_note"
	case el.dataType == "frame":
		return "frame"
	case el.name == "image":
		return "image"
	}
	return "shape"
}

// elementShape picks the Miro shape kind for a parsed element.
func elementShape(el svgElement) string {
	switch {
	case el.name == "circle" || el.name == "ellipse":
		return "circle"
	case el.name == "polygon":
		return "triangle"
	case el.rounded:
		return "round_rectangle"
	default:
		return "rectangle"
	}
}

// svgStickyColor passes only named sticky colors through; the sticky
// API rejects hex fills.
func svgStickyColor(fill string) string {
	unnamed := fill == "" || fill == "none"
	if unnamed || strings.HasPrefix(fill, "#") {
		return ""
	}
	return fill
}

// svgPosition builds the position payload for one element. Positions
// are centers with origin=center, matching how the parser recentered
// the SVG coordinates.
func svgPosition(el svgElement, off svgOffset) map[string]any {
	return map[string]any{"x": el.x + off.dx, "y": el.y + off.dy, "origin": "center"}
}

// buildElementRequest maps a parsed non-line element to its typed
// endpoint and wire body.
func buildElementRequest(boardID string, el svgElement, off svgOffset) (path string, body map[string]any) {
	position := svgPosition(el, off)
	base := "/v2/boards/" + boardID

	switch svgItemType(el) {
	case "text":
		return base + "/texts", map[string]any{
			"data":     map[string]any{"content": el.text},
			"position": position,
		}
	case "sticky_note":
		body = map[string]any{
			"data":     map[string]any{"content": el.text},
			"position": position,
			"geometry": map[string]any{"width": el.w},
		}
		if color := svgStickyColor(el.fill); color != "" {
			body["style"] = map[string]any{"fillColor": color}
		}
		return base + "/sticky_notes", body
	case "frame":
		return base + "/frames", map[string]any{
			"data":     map[string]any{"title": el.title},
			"position": position,
			"geometry": map[string]any{"width": el.w, "height": el.h},
		}
	case "image":
		body = map[string]any{
			"data":     map[string]any{"url": el.href},
			"position": position,
			"geometry": map[string]any{"width": el.w},
		}
		if el.title != "" {
			body["data"].(map[string]any)["title"] = el.title
		}
		return base + "/images", body
	}

	body = map[string]any{
		"data":     map[string]any{"shape": elementShape(el)},
		"position": position,
		"geometry": map[string]any{"width": el.w, "height": el.h},
	}
	if el.fill != "" && el.fill != "none" {
		body["style"] = map[string]any{"fillColor": el.fill}
	}
	return base + "/shapes", body
}

// svgCreateRun carries the state of one create pass: what landed, what
// was skipped, and the authored-id map connectors resolve against.
type svgCreateRun struct {
	created []createdItem
	skipped []skippedElement
	ids     map[string]string
}

// postCreate posts one create body and returns the new item's id.
func postCreate(ctx context.Context, client *miro.Client, path string, body map[string]any) (string, error) {
	var resp map[string]any
	if err := client.Post(ctx, path, body, &resp); err != nil {
		return "", err
	}
	id, _ := resp["id"].(string)
	return id, nil
}

// createItem creates one non-line element and records its authored id.
func (r *svgCreateRun) createItem(ctx context.Context, client *miro.Client, plan svgPlan, el svgElement) error {
	path, body := buildElementRequest(plan.boardID, el, plan.off)
	id, err := postCreate(ctx, client, path, body)
	if err != nil {
		return err
	}
	r.created = append(r.created, createdItem{ID: id, Type: svgItemType(el), Element: el.name})
	if el.authoredID != "" {
		r.ids[el.authoredID] = id
	}
	return nil
}

// createConnector creates one line connector, or records a skip when
// its data-start/data-end references don't resolve against the ids
// created this call. An unresolvable reference is a skip, not an
// error: the referenced element may itself have been skipped.
func (r *svgCreateRun) createConnector(ctx context.Context, client *miro.Client, plan svgPlan, el svgElement) error {
	startID, okS := r.ids[el.start]
	endID, okE := r.ids[el.end]
	if !okS || !okE {
		r.skipped = append(r.skipped, skippedElement{Element: "line", Reason: fmt.Sprintf("unresolved reference (data-start=%q, data-end=%q must match created element ids)", el.start, el.end)})
		return nil
	}
	body := map[string]any{
		"startItem": map[string]any{"id": startID},
		"endItem":   map[string]any{"id": endID},
	}
	if el.text != "" {
		body["captions"] = []map[string]any{{"content": el.text}}
	}
	id, err := postCreate(ctx, client, "/v2/boards/"+plan.boardID+"/connectors", body)
	if err != nil {
		return err
	}
	r.created = append(r.created, createdItem{ID: id, Type: "connector", Element: "line"})
	return nil
}

// pass creates every element matching the line filter, in document order.
func (r *svgCreateRun) pass(ctx context.Context, client *miro.Client, plan svgPlan, lines bool) error {
	for _, el := range plan.elements {
		if (el.name == "line") != lines {
			continue
		}
		var err error
		if lines {
			err = r.createConnector(ctx, client, plan, el)
		} else {
			err = r.createItem(ctx, client, plan, el)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// createSVGElements runs the two creation passes: items first (building
// the authored-id map connectors resolve against), then line
// connectors. On a mid-batch failure the run so far is returned along
// with the error so the caller can emit the partial envelope.
func createSVGElements(ctx context.Context, client *miro.Client, plan svgPlan) (*svgCreateRun, error) {
	run := &svgCreateRun{
		created: make([]createdItem, 0, len(plan.elements)),
		ids:     make(map[string]string),
	}
	for _, lines := range []bool{false, true} {
		if err := run.pass(ctx, client, plan, lines); err != nil {
			return run, err
		}
	}
	return run, nil
}

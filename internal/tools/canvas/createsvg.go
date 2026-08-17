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
			"POST /v2/boards/{board_id}/shapes and /texts:\n\n" +
			"  rect            -> shape rectangle (rx>0 -> round_rectangle)\n" +
			"  circle/ellipse  -> shape circle\n" +
			"  text            -> text item\n" +
			"  g translate(x,y)-> offset applied to children (nesting ok)\n\n" +
			"Unsupported elements (path, polygon, line, ...) are reported in\n" +
			"`skipped`, never silently dropped. Supply the document with\n" +
			"--svg-file (use \"-\" for stdin) or inline via --svg. The cap is\n" +
			fmt.Sprintf("%d drawable elements per call.", maxSVGCreateElements),
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
// loop: the flags that produced it plus the drawable and skipped
// elements.
type svgPlan struct {
	flags    createSVGFlags
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
	return svgPlan{flags: f, elements: elements, skipped: skipped}, nil
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
			fmt.Sprintf("/v2/boards/%s/{shapes,texts} × %d elements", f.boardID, len(plan.elements)))
	}
	if len(plan.elements) == 0 {
		return g.EmitJSON(createSVGResult{
			Created: []createdItem{},
			Skipped: plan.skipped,
			Message: "No supported elements found in the SVG (supported: rect, circle, ellipse, text)",
		})
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	return createSVGElements(ctx, g, client, plan)
}

// createSVGElements posts one create per parsed element, emitting the
// full envelope on success or the partial envelope (created IDs kept)
// plus an error on a mid-batch failure.
func createSVGElements(ctx context.Context, g *clictx.Globals, client *miro.Client, plan svgPlan) error {
	created := make([]createdItem, 0, len(plan.elements))
	for _, el := range plan.elements {
		item, err := createSVGElement(ctx, client, plan.flags, el)
		if err != nil {
			_ = g.EmitJSON(createSVGResult{
				Created: created,
				Skipped: plan.skipped,
				Count:   len(created),
				Message: fmt.Sprintf("Partial failure after %d item(s); see error", len(created)),
			})
			return fmt.Errorf("create from svg failed after %d item(s): %w", len(created), err)
		}
		created = append(created, item)
	}

	return g.EmitJSON(createSVGResult{
		Created: created,
		Skipped: plan.skipped,
		Count:   len(created),
		Message: fmt.Sprintf("Created %d item(s) from SVG (%d element(s) skipped)", len(created), len(plan.skipped)),
	})
}

// createSVGElement creates one board item for a parsed element.
func createSVGElement(ctx context.Context, client *miro.Client, f createSVGFlags, el svgElement) (createdItem, error) {
	path, body := buildElementRequest(f, el)
	var resp map[string]any
	if err := client.Post(ctx, path, body, &resp); err != nil {
		return createdItem{}, err
	}
	id, _ := resp["id"].(string)
	itemType := "shape"
	if el.name == "text" {
		itemType = "text"
	}
	return createdItem{ID: id, Type: itemType, Element: el.name}, nil
}

// buildElementRequest maps a parsed element to its endpoint and wire
// body. Positions are centers with origin=center, matching how the
// parser recentered the SVG coordinates.
func buildElementRequest(f createSVGFlags, el svgElement) (path string, body map[string]any) {
	position := map[string]any{"x": el.x + f.offsetX, "y": el.y + f.offsetY, "origin": "center"}

	if el.name == "text" {
		return "/v2/boards/" + f.boardID + "/texts", map[string]any{
			"data":     map[string]any{"content": el.text},
			"position": position,
		}
	}

	body = map[string]any{
		"data":     map[string]any{"shape": elementShape(el)},
		"position": position,
		"geometry": map[string]any{"width": el.w, "height": el.h},
	}
	if el.fill != "" && el.fill != "none" {
		body["style"] = map[string]any{"fillColor": el.fill}
	}
	return "/v2/boards/" + f.boardID + "/shapes", body
}

// elementShape picks the Miro shape kind for a parsed element.
func elementShape(el svgElement) string {
	switch {
	case el.name == "circle" || el.name == "ellipse":
		return "circle"
	case el.rounded:
		return "round_rectangle"
	default:
		return "rectangle"
	}
}

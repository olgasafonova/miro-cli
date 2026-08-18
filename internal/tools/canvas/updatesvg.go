package canvas

// SVG -> board diff (update / delete / additive create, keyed on
// data-miro-id). Ported from miro-mcp-server miro/svg_update.go (same
// author; same deliberate duplication as the read/create bridge).
//
// Elements carrying data-miro-id are updated in place with PATCH
// semantics: geometry is restated as a unit (x, y, width, height
// together), fill maps to style.fillColor, text content maps to
// data.content. Elements with data-miro-id and data-deleted="true" are
// deleted. Elements without data-miro-id are created, reusing the
// create dialect and its two-pass connector resolution.
//
// Two failure classes, deliberately: a malformed document fails the
// whole request at the parse layer (nothing applied); a per-item
// semantic error lands in `failed` while the rest of the batch applies.
// The output of `canvas read-svg` is re-submittable here — its elements
// carry data-miro-id and top-left rect geometry, exactly what this
// parser expects.

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// updateSVGFlags captures the per-invocation knobs for `miro canvas
// update-from-svg`. Exactly one of svg / svgFile supplies the document.
type updateSVGFlags struct {
	boardID string
	svg     string
	svgFile string
}

// updatedItem names one board item the diff updated in place.
type updatedItem struct {
	ID      string `json:"id"`
	Element string `json:"element"`
}

// failedItem records one element whose update or delete failed, with
// the reason. The rest of the batch still applies.
type failedItem struct {
	ID      string `json:"id"`
	Element string `json:"element"`
	Reason  string `json:"reason"`
}

// updateSVGResult is the JSON envelope `canvas update-from-svg` emits.
type updateSVGResult struct {
	Updated []updatedItem    `json:"updated"`
	Deleted []string         `json:"deleted"`
	Created []createdItem    `json:"created"`
	Failed  []failedItem     `json:"failed,omitempty"`
	Skipped []skippedElement `json:"skipped,omitempty"`
	Message string           `json:"message"`
}

func newUpdateFromSVGCmd(g *clictx.Globals) *cobra.Command {
	var f updateSVGFlags
	cmd := &cobra.Command{
		Use:   "update-from-svg",
		Short: "Apply an SVG document to a board as a diff",
		Long: "Applies an SVG document as a diff keyed on data-miro-id:\n\n" +
			"  data-miro-id=X                    -> PATCH the item in place\n" +
			"  data-miro-id=X data-deleted=true  -> DELETE the item\n" +
			"  no data-miro-id                   -> create (same dialect as\n" +
			"                                       create-from-svg)\n\n" +
			"Geometry is restated as a unit (x, y, width, height together);\n" +
			"fill maps to the fill color, text content to the item content.\n" +
			"A malformed document fails the whole request with nothing\n" +
			"applied; per-item semantic errors land in `failed` while the\n" +
			"rest of the batch applies. The output of `canvas read-svg` is\n" +
			"re-submittable here as-is. Supply the document with --svg-file\n" +
			"(use \"-\" for stdin) or inline via --svg.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateFromSVG(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.svg, "svg", "", "SVG document as an inline string")
	cmd.Flags().StringVar(&f.svgFile, "svg-file", "", "Path to an SVG file, or \"-\" for stdin")
	_ = cmd.MarkFlagRequired("board-id")
	cmd.MarkFlagsMutuallyExclusive("svg", "svg-file")
	cmd.MarkFlagsOneRequired("svg", "svg-file")
	return cmd
}

// svgUpdateOutcome accumulates the per-element results of one update call.
type svgUpdateOutcome struct {
	updated []updatedItem
	deleted []string
	failed  []failedItem
	creates []svgElement
}

func (o *svgUpdateOutcome) fail(el svgElement, reason string) {
	o.failed = append(o.failed, failedItem{ID: el.miroID, Element: el.name, Reason: reason})
}

// svgUpdateBody builds the PATCH payload for one identified element.
// Geometry is a unit: rects and ellipses restate all four values, text
// restates its anchor only (Miro sizes text by content).
func svgUpdateBody(el svgElement) (map[string]any, string) {
	body := map[string]any{
		"position": map[string]any{"x": el.x, "y": el.y, "origin": "center"},
	}
	if el.name == "text" {
		body["data"] = map[string]any{"content": el.text}
		return body, ""
	}
	if el.w <= 0 || el.h <= 0 {
		return nil, "update requires full geometry (x, y, width, height restated as a unit)"
	}
	body["geometry"] = map[string]any{"width": el.w, "height": el.h}
	if el.fill != "" && el.fill != "none" {
		body["style"] = map[string]any{"fillColor": el.fill}
	}
	if el.text != "" {
		body["data"] = map[string]any{"content": el.text}
	}
	return body, ""
}

// applySVGUpdate updates one identified element in place via the
// generic items PATCH.
func applySVGUpdate(ctx context.Context, client *miro.Client, boardID string, el svgElement, out *svgUpdateOutcome) {
	body, reason := svgUpdateBody(el)
	if reason != "" {
		out.fail(el, reason)
		return
	}
	var resp map[string]any
	if err := client.Patch(ctx, "/v2/boards/"+boardID+"/items/"+el.miroID, body, &resp); err != nil {
		out.fail(el, err.Error())
		return
	}
	out.updated = append(out.updated, updatedItem{ID: el.miroID, Element: el.name})
}

// applySVGDelete deletes one identified element. Connectors are not
// reachable through the generic items endpoint (DELETE there is a 404,
// verified live 18-08-2026), so lines route to /connectors.
func applySVGDelete(ctx context.Context, client *miro.Client, boardID string, el svgElement, out *svgUpdateOutcome) {
	family := "/items/"
	if el.name == "line" {
		family = "/connectors/"
	}
	if err := client.Delete(ctx, "/v2/boards/"+boardID+family+el.miroID); err != nil {
		out.fail(el, err.Error())
		return
	}
	out.deleted = append(out.deleted, el.miroID)
}

// routeSVGElement sends one parsed element to its diff action: delete,
// update, or (for elements with no data-miro-id) deferred creation.
func routeSVGElement(ctx context.Context, client *miro.Client, boardID string, el svgElement, out *svgUpdateOutcome) {
	switch {
	case el.miroID == "":
		out.creates = append(out.creates, el)
	case el.deleted:
		applySVGDelete(ctx, client, boardID, el, out)
	case el.name == "line":
		out.fail(el, "connectors cannot be updated through SVG; use `miro connectors update`")
	default:
		applySVGUpdate(ctx, client, boardID, el, out)
	}
}

// updateFromSVGMessage summarizes one applied diff.
func updateFromSVGMessage(out *svgUpdateOutcome, created, skipped int) string {
	return fmt.Sprintf("Updated %d, deleted %d, created %d item(s) (%d failed, %d skipped)",
		len(out.updated), len(out.deleted), created, len(out.failed), skipped)
}

// loadUpdateSVGPlan resolves and parses the document for the diff.
func loadUpdateSVGPlan(f updateSVGFlags) (svgPlan, error) {
	return loadAndParseSVG(createSVGFlags{
		boardID: f.boardID,
		svg:     f.svg,
		svgFile: f.svgFile,
	})
}

func runUpdateFromSVG(ctx context.Context, g *clictx.Globals, f updateSVGFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	plan, err := loadUpdateSVGPlan(f)
	if err != nil {
		return err
	}
	if g.DryRun {
		return g.EmitDryRun("PATCH/DELETE/POST",
			fmt.Sprintf("/v2/boards/%s/items/* (diff keyed on data-miro-id, %d elements)", f.boardID, len(plan.elements)))
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	out := &svgUpdateOutcome{updated: []updatedItem{}, deleted: []string{}}
	for _, el := range plan.elements {
		routeSVGElement(ctx, client, f.boardID, el, out)
	}

	createPlan := svgPlan{boardID: f.boardID, elements: out.creates}
	run, createErr := createSVGElements(ctx, client, createPlan)

	skipped := append(plan.skipped, run.skipped...)
	result := updateSVGResult{
		Updated: out.updated,
		Deleted: out.deleted,
		Created: run.created,
		Failed:  out.failed,
		Skipped: skipped,
		Message: updateFromSVGMessage(out, len(run.created), len(skipped)),
	}
	if createErr != nil {
		_ = g.EmitJSON(result)
		return fmt.Errorf("update from svg: create pass failed after %d item(s): %w", len(run.created), createErr)
	}
	return g.EmitJSON(result)
}

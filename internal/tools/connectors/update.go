package connectors

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set. Cobra zeroes unset string vars, so we'd otherwise
// confuse "user passed --shape=”" with "user didn't pass --shape".
// The *Set fields track presence via cmd.Flags().Changed().
type updateFlags struct {
	boardID     string
	connectorID string

	startItemID string
	endItemID   string
	startSnapTo string
	endSnapTo   string
	startPos    string
	endPos      string

	shape string

	strokeColor     string
	strokeWidth     string
	strokeStyle     string
	startStrokeCap  string
	endStrokeCap    string
	fontSize        string
	captionColor    string
	textOrientation string

	captions      []string
	clearCaptions bool

	startItemIDSet     bool
	endItemIDSet       bool
	startSnapToSet     bool
	endSnapToSet       bool
	startPosSet        bool
	endPosSet          bool
	shapeSet           bool
	strokeColorSet     bool
	strokeWidthSet     bool
	strokeStyleSet     bool
	startStrokeCapSet  bool
	endStrokeCapSet    bool
	fontSizeSet        bool
	captionColorSet    bool
	textOrientationSet bool
	captionsSet        bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a connector (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/connectors/{connector_id} with\n" +
			"only the fields you set. Caption updates replace the full caption\n" +
			"list — pass --caption once per caption you want kept. Use\n" +
			"--clear-captions to remove all captions without setting new ones.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.startItemIDSet = fl.Changed("start-item-id")
			f.endItemIDSet = fl.Changed("end-item-id")
			f.startSnapToSet = fl.Changed("start-snap-to")
			f.endSnapToSet = fl.Changed("end-snap-to")
			f.startPosSet = fl.Changed("start-position")
			f.endPosSet = fl.Changed("end-position")
			f.shapeSet = fl.Changed("shape")
			f.strokeColorSet = fl.Changed("stroke-color")
			f.strokeWidthSet = fl.Changed("stroke-width")
			f.strokeStyleSet = fl.Changed("stroke-style")
			f.startStrokeCapSet = fl.Changed("start-stroke-cap")
			f.endStrokeCapSet = fl.Changed("end-stroke-cap")
			f.fontSizeSet = fl.Changed("font-size")
			f.captionColorSet = fl.Changed("caption-color")
			f.textOrientationSet = fl.Changed("text-orientation")
			f.captionsSet = fl.Changed("caption") || f.clearCaptions
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.connectorID, "connector-id", "", "Connector ID (required)")
	cmd.Flags().StringVar(&f.startItemID, "start-item-id", "", "New start item ID")
	cmd.Flags().StringVar(&f.endItemID, "end-item-id", "", "New end item ID")
	cmd.Flags().StringVar(&f.startSnapTo, "start-snap-to", "", "New start snap side (auto|top|right|bottom|left)")
	cmd.Flags().StringVar(&f.endSnapTo, "end-snap-to", "", "New end snap side (auto|top|right|bottom|left)")
	cmd.Flags().StringVar(&f.startPos, "start-position", "", "New start relative position, e.g. 50%,0%")
	cmd.Flags().StringVar(&f.endPos, "end-position", "", "New end relative position, e.g. 50%,100%")
	cmd.Flags().StringVar(&f.shape, "shape", "", "New path shape (curved|straight|elbowed)")
	cmd.Flags().StringVar(&f.strokeColor, "stroke-color", "", "New line color (hex)")
	cmd.Flags().StringVar(&f.strokeWidth, "stroke-width", "", "New line thickness (1-24, as string)")
	cmd.Flags().StringVar(&f.strokeStyle, "stroke-style", "", "New line pattern (normal|dotted|dashed)")
	cmd.Flags().StringVar(&f.startStrokeCap, "start-stroke-cap", "", "New start decoration cap")
	cmd.Flags().StringVar(&f.endStrokeCap, "end-stroke-cap", "", "New end decoration cap")
	cmd.Flags().StringVar(&f.fontSize, "font-size", "", "New caption font size (10-288, as string)")
	cmd.Flags().StringVar(&f.captionColor, "caption-color", "", "New caption text color (hex)")
	cmd.Flags().StringVar(&f.textOrientation, "text-orientation", "", "New caption orientation (horizontal|aligned)")
	cmd.Flags().StringArrayVar(&f.captions, "caption", nil, "Replacement caption (repeatable). Replaces all existing captions.")
	cmd.Flags().BoolVar(&f.clearCaptions, "clear-captions", false, "Remove all captions from the connector")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("connector-id")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if err := validateUpdateFlags(f); err != nil {
		return err
	}
	req, ok, err := buildUpdateRequest(f)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/connectors/" + f.connectorID
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// validateUpdateFlags checks the required IDs and runs the validator for
// each style/endpoint flag the user actually set. Each entry pairs a
// "was this flag set" guard with the validation to run when it was, so
// adding a validated flag means adding one row, not another if-block.
func validateUpdateFlags(f updateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("connector_id", f.connectorID); err != nil {
		return err
	}
	checks := []struct {
		set      bool
		validate func() error
	}{
		{f.shapeSet, func() error { return validateShape(f.shape) }},
		{f.startSnapToSet, func() error { return validateSnapTo(f.startSnapTo, "start-snap-to") }},
		{f.endSnapToSet, func() error { return validateSnapTo(f.endSnapTo, "end-snap-to") }},
		{f.strokeStyleSet, func() error { return validateStrokeStyle(f.strokeStyle) }},
		{f.startStrokeCapSet, func() error { return validateStrokeCap(f.startStrokeCap, "start-stroke-cap") }},
		{f.endStrokeCapSet, func() error { return validateStrokeCap(f.endStrokeCap, "end-stroke-cap") }},
		{f.textOrientationSet, func() error { return validateTextOrientation(f.textOrientation) }},
	}
	for _, c := range checks {
		if !c.set {
			continue
		}
		if err := c.validate(); err != nil {
			return err
		}
	}
	return nil
}

// buildUpdateRequest projects updateFlags into the PATCH body and
// reports whether any field was set. ok=false means the caller should
// reject the update — Miro 400s an empty PATCH body anyway, and a
// pre-flight check produces a clearer error.
func buildUpdateRequest(f updateFlags) (updateRequest, bool, error) {
	var req updateRequest
	any := false

	start, ok, err := buildEndpointFromFlags(endpointFlags{
		itemID: f.startItemID, itemIDSet: f.startItemIDSet,
		snapTo: f.startSnapTo, snapToSet: f.startSnapToSet,
		position: f.startPos, positionSet: f.startPosSet,
	})
	if err != nil {
		return updateRequest{}, false, err
	}
	if ok {
		req.StartItem = start
		any = true
	}

	end, ok, err := buildEndpointFromFlags(endpointFlags{
		itemID: f.endItemID, itemIDSet: f.endItemIDSet,
		snapTo: f.endSnapTo, snapToSet: f.endSnapToSet,
		position: f.endPos, positionSet: f.endPosSet,
	})
	if err != nil {
		return updateRequest{}, false, err
	}
	if ok {
		req.EndItem = end
		any = true
	}

	if f.shapeSet {
		req.Shape = f.shape
		any = true
	}
	if f.captionsSet {
		captions, err := buildCaptions(f.captions)
		if err != nil {
			return updateRequest{}, false, err
		}
		req.Captions = captions
		req.captionsSet = true
		any = true
	}
	if style := buildUpdateStyle(f); style != nil {
		req.Style = style
		any = true
	}
	return req, any, nil
}

// endpointFlags groups one endpoint's (item/snap/position) flag values
// and their presence bits so the builder takes one argument, not six.
type endpointFlags struct {
	itemID      string
	itemIDSet   bool
	snapTo      string
	snapToSet   bool
	position    string
	positionSet bool
}

// buildEndpointFromFlags reports whether any of an endpoint's fields were
// set and, if so, returns the constructed itemEndpoint. ok=false means
// the caller should leave that endpoint untouched in the PATCH body.
func buildEndpointFromFlags(ef endpointFlags) (*itemEndpoint, bool, error) {
	if !ef.itemIDSet && !ef.snapToSet && !ef.positionSet {
		return nil, false, nil
	}
	ep := &itemEndpoint{}
	if ef.itemIDSet {
		ep.ID = ef.itemID
	}
	if ef.snapToSet {
		ep.SnapTo = ef.snapTo
	}
	if ef.positionSet {
		off, err := parsePosition(ef.position)
		if err != nil {
			return nil, false, err
		}
		ep.Position = off
	}
	return ep, true, nil
}

// buildUpdateStyle assembles the style envelope only when at least one
// style field was set. The check uses the *Set bools so explicit empty
// strings can clear individual fields without clobbering the rest.
func buildUpdateStyle(f updateFlags) *connectorStyle {
	if !anyStyleFieldSet(f) {
		return nil
	}
	s := &connectorStyle{}
	for _, field := range styleFieldSetters(f) {
		if field.set {
			field.apply(s)
		}
	}
	return s
}

// anyStyleFieldSet reports whether at least one style flag was provided,
// which is the condition for emitting a style envelope at all.
func anyStyleFieldSet(f updateFlags) bool {
	for _, field := range styleFieldSetters(f) {
		if field.set {
			return true
		}
	}
	return false
}

// styleFieldSetters maps each style flag's "was it set" guard to the
// mutation that copies its value onto the style envelope. Centralising
// the list keeps anyStyleFieldSet and buildUpdateStyle in lockstep.
func styleFieldSetters(f updateFlags) []struct {
	set   bool
	apply func(*connectorStyle)
} {
	return []struct {
		set   bool
		apply func(*connectorStyle)
	}{
		{f.strokeColorSet, func(s *connectorStyle) { s.StrokeColor = f.strokeColor }},
		{f.strokeWidthSet, func(s *connectorStyle) { s.StrokeWidth = f.strokeWidth }},
		{f.strokeStyleSet, func(s *connectorStyle) { s.StrokeStyle = f.strokeStyle }},
		{f.startStrokeCapSet, func(s *connectorStyle) { s.StartStrokeCap = f.startStrokeCap }},
		{f.endStrokeCapSet, func(s *connectorStyle) { s.EndStrokeCap = f.endStrokeCap }},
		{f.fontSizeSet, func(s *connectorStyle) { s.FontSize = f.fontSize }},
		{f.captionColorSet, func(s *connectorStyle) { s.Color = f.captionColor }},
		{f.textOrientationSet, func(s *connectorStyle) { s.TextOrientation = f.textOrientation }},
	}
}

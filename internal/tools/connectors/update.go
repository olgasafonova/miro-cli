package connectors

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
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
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("connector_id", f.connectorID); err != nil {
		return err
	}
	if f.shapeSet {
		if err := validateShape(f.shape); err != nil {
			return err
		}
	}
	if f.startSnapToSet {
		if err := validateSnapTo(f.startSnapTo, "start-snap-to"); err != nil {
			return err
		}
	}
	if f.endSnapToSet {
		if err := validateSnapTo(f.endSnapTo, "end-snap-to"); err != nil {
			return err
		}
	}
	if f.strokeStyleSet {
		if err := validateStrokeStyle(f.strokeStyle); err != nil {
			return err
		}
	}
	if f.startStrokeCapSet {
		if err := validateStrokeCap(f.startStrokeCap, "start-stroke-cap"); err != nil {
			return err
		}
	}
	if f.endStrokeCapSet {
		if err := validateStrokeCap(f.endStrokeCap, "end-stroke-cap"); err != nil {
			return err
		}
	}
	if f.textOrientationSet {
		if err := validateTextOrientation(f.textOrientation); err != nil {
			return err
		}
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

// buildUpdateRequest projects updateFlags into the PATCH body and
// reports whether any field was set. ok=false means the caller should
// reject the update — Miro 400s an empty PATCH body anyway, and a
// pre-flight check produces a clearer error.
func buildUpdateRequest(f updateFlags) (updateRequest, bool, error) {
	var req updateRequest
	any := false

	if f.startItemIDSet || f.startSnapToSet || f.startPosSet {
		ep, err := buildUpdateEndpoint(f.startItemID, f.startItemIDSet, f.startSnapTo, f.startSnapToSet, f.startPos, f.startPosSet)
		if err != nil {
			return updateRequest{}, false, err
		}
		req.StartItem = ep
		any = true
	}
	if f.endItemIDSet || f.endSnapToSet || f.endPosSet {
		ep, err := buildUpdateEndpoint(f.endItemID, f.endItemIDSet, f.endSnapTo, f.endSnapToSet, f.endPos, f.endPosSet)
		if err != nil {
			return updateRequest{}, false, err
		}
		req.EndItem = ep
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

// buildUpdateEndpoint constructs an itemEndpoint for a PATCH. Only set
// fields are populated; the resulting envelope can carry an id alone,
// just a new snap side, or only a new relative position.
func buildUpdateEndpoint(itemID string, itemIDSet bool, snapTo string, snapToSet bool, position string, positionSet bool) (*itemEndpoint, error) {
	ep := &itemEndpoint{}
	if itemIDSet {
		ep.ID = itemID
	}
	if snapToSet {
		ep.SnapTo = snapTo
	}
	if positionSet {
		off, err := parsePosition(position)
		if err != nil {
			return nil, err
		}
		ep.Position = off
	}
	return ep, nil
}

// buildUpdateStyle assembles the style envelope only when at least one
// style field was set. The check uses the *Set bools so explicit empty
// strings can clear individual fields without clobbering the rest.
func buildUpdateStyle(f updateFlags) *connectorStyle {
	if !f.strokeColorSet && !f.strokeWidthSet && !f.strokeStyleSet &&
		!f.startStrokeCapSet && !f.endStrokeCapSet && !f.fontSizeSet &&
		!f.captionColorSet && !f.textOrientationSet {
		return nil
	}
	s := &connectorStyle{}
	if f.strokeColorSet {
		s.StrokeColor = f.strokeColor
	}
	if f.strokeWidthSet {
		s.StrokeWidth = f.strokeWidth
	}
	if f.strokeStyleSet {
		s.StrokeStyle = f.strokeStyle
	}
	if f.startStrokeCapSet {
		s.StartStrokeCap = f.startStrokeCap
	}
	if f.endStrokeCapSet {
		s.EndStrokeCap = f.endStrokeCap
	}
	if f.fontSizeSet {
		s.FontSize = f.fontSize
	}
	if f.captionColorSet {
		s.Color = f.captionColor
	}
	if f.textOrientationSet {
		s.TextOrientation = f.textOrientation
	}
	return s
}

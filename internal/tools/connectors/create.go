package connectors

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro connectors
// create`. The flag surface is wide because connectors carry both
// endpoint geometry (start/end items + snap-or-position) and style.
type createFlags struct {
	boardID string

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

	// captions are repeatable: "--caption text" or "--caption text@50%".
	captions []string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a connector between two items on a board",
		Long: "Calls POST /v2/boards/{board_id}/connectors with --start-item-id\n" +
			"and --end-item-id (both required). Optional flags control the\n" +
			"shape (curved|straight|elbowed), snap-to side (auto|top|right|\n" +
			"bottom|left), exact relative position (X%,Y%), stroke style, end\n" +
			"caps, captions, and caption typography.\n\n" +
			"Captions are repeatable: --caption \"Approve\" or --caption\n" +
			"\"Approve@25%\" to place the text at 25% along the connector.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.startItemID, "start-item-id", "", "ID of the item where the connector starts (required)")
	cmd.Flags().StringVar(&f.endItemID, "end-item-id", "", "ID of the item where the connector ends (required)")
	cmd.Flags().StringVar(&f.startSnapTo, "start-snap-to", "", "Side of the start item to anchor to (auto|top|right|bottom|left)")
	cmd.Flags().StringVar(&f.endSnapTo, "end-snap-to", "", "Side of the end item to anchor to (auto|top|right|bottom|left)")
	cmd.Flags().StringVar(&f.startPos, "start-position", "", "Relative position on the start item, e.g. 50%,0% (mutually exclusive with --start-snap-to)")
	cmd.Flags().StringVar(&f.endPos, "end-position", "", "Relative position on the end item, e.g. 50%,100% (mutually exclusive with --end-snap-to)")
	cmd.Flags().StringVar(&f.shape, "shape", "", "Path shape (curved|straight|elbowed). Default: curved")
	cmd.Flags().StringVar(&f.strokeColor, "stroke-color", "", "Hex color for the connector line, e.g. #2d9bf0")
	cmd.Flags().StringVar(&f.strokeWidth, "stroke-width", "", "Line thickness in dp (1-24), as a string")
	cmd.Flags().StringVar(&f.strokeStyle, "stroke-style", "", "Line pattern (normal|dotted|dashed)")
	cmd.Flags().StringVar(&f.startStrokeCap, "start-stroke-cap", "", "Decoration at the start (none|stealth|arrow|...)")
	cmd.Flags().StringVar(&f.endStrokeCap, "end-stroke-cap", "", "Decoration at the end (none|stealth|arrow|...)")
	cmd.Flags().StringVar(&f.fontSize, "font-size", "", "Caption font size in dp (10-288), as a string")
	cmd.Flags().StringVar(&f.captionColor, "caption-color", "", "Hex color for caption text, e.g. #1a1a1a")
	cmd.Flags().StringVar(&f.textOrientation, "text-orientation", "", "Caption orientation (horizontal|aligned)")
	cmd.Flags().StringArrayVar(&f.captions, "caption", nil, "Caption text, repeatable. Format: \"text\" or \"text@50%\"")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("start-item-id")
	_ = cmd.MarkFlagRequired("end-item-id")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("start_item_id", f.startItemID); err != nil {
		return err
	}
	if err := miro.ValidateID("end_item_id", f.endItemID); err != nil {
		return err
	}
	if f.startItemID == f.endItemID {
		return errors.New("--start-item-id and --end-item-id must differ")
	}
	if err := validateShape(f.shape); err != nil {
		return err
	}
	if err := validateSnapTo(f.startSnapTo, "start-snap-to"); err != nil {
		return err
	}
	if err := validateSnapTo(f.endSnapTo, "end-snap-to"); err != nil {
		return err
	}
	if err := validateStrokeStyle(f.strokeStyle); err != nil {
		return err
	}
	if err := validateStrokeCap(f.startStrokeCap, "start-stroke-cap"); err != nil {
		return err
	}
	if err := validateStrokeCap(f.endStrokeCap, "end-stroke-cap"); err != nil {
		return err
	}
	if err := validateTextOrientation(f.textOrientation); err != nil {
		return err
	}
	req, err := buildCreateRequest(f)
	if err != nil {
		return err
	}
	path := "/v2/boards/" + f.boardID + "/connectors"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Post(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildCreateRequest projects createFlags into the wire body. Split out
// so tests can assert on the shape without an httptest server. Returns
// an error only when caption parsing fails — most validation happens in
// runCreate before this call.
func buildCreateRequest(f createFlags) (createRequest, error) {
	start, err := buildEndpoint(f.startItemID, f.startSnapTo, f.startPos)
	if err != nil {
		return createRequest{}, err
	}
	end, err := buildEndpoint(f.endItemID, f.endSnapTo, f.endPos)
	if err != nil {
		return createRequest{}, err
	}
	captions, err := buildCaptions(f.captions)
	if err != nil {
		return createRequest{}, err
	}
	req := createRequest{
		StartItem: start,
		EndItem:   end,
		Shape:     f.shape,
		Captions:  captions,
	}
	if style := buildStyle(f.strokeColor, f.strokeWidth, f.strokeStyle, f.startStrokeCap, f.endStrokeCap, f.fontSize, f.captionColor, f.textOrientation); style != nil {
		req.Style = style
	}
	return req, nil
}

// buildEndpoint constructs an itemEndpoint from the three per-side
// inputs. itemID is always required at the call site; snapTo and
// position are both optional and the API rejects setting both.
func buildEndpoint(itemID, snapTo, position string) (*itemEndpoint, error) {
	ep := &itemEndpoint{ID: itemID}
	if snapTo != "" {
		ep.SnapTo = snapTo
	}
	if position != "" {
		off, err := parsePosition(position)
		if err != nil {
			return nil, err
		}
		ep.Position = off
	}
	return ep, nil
}

// buildCaptions parses each repeated --caption value into a captionData.
// Returns nil (not empty slice) when no captions were passed, so the
// JSON encoder omits the field.
func buildCaptions(raw []string) ([]captionData, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]captionData, 0, len(raw))
	for _, s := range raw {
		c, err := parseCaption(s)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// buildStyle returns a *connectorStyle only when at least one style
// field was set. Empty fields are omitted from the wire body via the
// omitempty tags on connectorStyle.
func buildStyle(strokeColor, strokeWidth, strokeStyle, startCap, endCap, fontSize, captionColor, textOrientation string) *connectorStyle {
	if strokeColor == "" && strokeWidth == "" && strokeStyle == "" &&
		startCap == "" && endCap == "" && fontSize == "" &&
		captionColor == "" && textOrientation == "" {
		return nil
	}
	return &connectorStyle{
		Color:           captionColor,
		StrokeColor:     strokeColor,
		StrokeWidth:     strokeWidth,
		StrokeStyle:     strokeStyle,
		StartStrokeCap:  startCap,
		EndStrokeCap:    endCap,
		FontSize:        fontSize,
		TextOrientation: textOrientation,
	}
}

package appcards

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro app-cards create`.
// Named so the cobra wiring stays flat and the helper signature stays
// friendly to table-driven tests.
type createFlags struct {
	boardID     string
	title       string
	description string
	status      string
	owned       bool
	ownedSet    bool
	color       string
	x           float64
	y           float64
	width       float64
	height      float64
	parentID    string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an app card on a board",
		Long: "Calls POST /v2/boards/{board_id}/app_cards with --title\n" +
			"(required) and optional --description / --status / --owned /\n" +
			"--color / --x / --y / --width / --height / --parent-id. Returns\n" +
			"the new app card's id, data, style, and viewLink.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"frame's top-left and the card's center is placed at (x, y).\n" +
			"On the canvas (no parent), they are absolute.\n\n" +
			"Custom fields (data.fields) are deferred to Phase 4; callers\n" +
			"needing them today should use the generic items command.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f.ownedSet = cmd.Flags().Changed("owned")
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "App card title (required)")
	cmd.Flags().StringVar(&f.description, "description", "", "App card description")
	cmd.Flags().StringVar(&f.status, "status", "", "App card status (disconnected|connected|disabled)")
	cmd.Flags().BoolVar(&f.owned, "owned", false, "Mark the card as owned by the calling app")
	cmd.Flags().StringVar(&f.color, "color", "", "Fill color (hex code, e.g. #ff0000)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the card inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.title == "" {
		return errors.New("--title is required")
	}
	if err := validateStatus(f.status); err != nil {
		return err
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/app_cards"
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

// buildCreateRequest is split out so tests can assert on the wire shape
// without spinning an httptest server.
func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{
		Data: dataField{Title: f.title, Description: f.description, Status: f.status},
	}
	if f.ownedSet {
		owned := f.owned
		req.Data.Owned = &owned
	}
	if f.color != "" {
		req.Style = &styleField{FillColor: f.color}
	}
	// Position is always emitted when create is called — Miro defaults
	// to (0, 0) which is fine for canvas placement, and explicit origin
	// keeps the math predictable for frame-relative coords.
	req.Position = &positionData{X: f.x, Y: f.y, Origin: "center"}
	if f.width > 0 || f.height > 0 {
		req.Geometry = &geometryData{Width: f.width, Height: f.height}
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

package cards

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro cards create`.
// Named so the cobra wiring stays flat and the helper signature stays
// friendly to table-driven tests.
type createFlags struct {
	boardID     string
	title       string
	description string
	assigneeID  string
	dueDate     string
	cardTheme   string
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
		Short: "Create a card on a board",
		Long: "Calls POST /v2/boards/{board_id}/cards with --title\n" +
			"(required) and optional --description / --assignee-id /\n" +
			"--due-date / --card-theme / --x / --y / --width / --height /\n" +
			"--parent-id. Returns the new card's id, data, style, and\n" +
			"viewLink.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"frame's top-left and the card's center is placed at (x, y).\n" +
			"On the canvas (no parent), they are absolute.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Card title (required)")
	cmd.Flags().StringVar(&f.description, "description", "", "Card description")
	cmd.Flags().StringVar(&f.assigneeID, "assignee-id", "", "Assignee user ID")
	cmd.Flags().StringVar(&f.dueDate, "due-date", "", "Due date (ISO8601)")
	cmd.Flags().StringVar(&f.cardTheme, "card-theme", "", "Card theme color (hex, e.g. #2d9bf0)")
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
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/cards"
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
		Data: dataField{
			Title:       f.title,
			Description: f.description,
			AssigneeID:  f.assigneeID,
			DueDate:     f.dueDate,
		},
	}
	if f.cardTheme != "" {
		req.Style = &styleField{CardTheme: f.cardTheme}
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

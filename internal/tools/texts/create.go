package texts

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

type createFlags struct {
	boardID  string
	content  string
	color    string
	fontSize int
	width    float64
	x        float64
	y        float64
	parentID string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a free-floating text item on a board",
		Long: "Calls POST /v2/boards/{board_id}/texts with --content (required)\n" +
			"and optional --color / --font-size / --width / --x / --y /\n" +
			"--parent-id. Texts have no background — for notes with colored\n" +
			"fills use `miro stickies create` instead.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "Text content (required)")
	cmd.Flags().StringVar(&f.color, "color", "", "Text color (hex like #006400 or named: red, green, blue, ...)")
	cmd.Flags().IntVar(&f.fontSize, "font-size", 0, "Font size in points (Miro accepts integers; default ~14)")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels (height auto-scales)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the text inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.content == "" {
		return errors.New("--content is required")
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/texts"
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

func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{Data: dataField{Content: f.content}}
	if f.color != "" || f.fontSize > 0 {
		req.Style = &styleField{
			Color:    f.color,
			FontSize: fontSizeString(f.fontSize),
		}
	}
	req.Position = &positionData{X: f.x, Y: f.y, Origin: "center"}
	if f.width > 0 {
		req.Geometry = &geometryData{Width: f.width}
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

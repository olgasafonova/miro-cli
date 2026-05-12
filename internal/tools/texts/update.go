package texts

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

type updateFlags struct {
	boardID  string
	itemID   string
	content  string
	color    string
	fontSize int
	width    float64
	x        float64
	y        float64
	parentID string

	contentSet  bool
	colorSet    bool
	fontSizeSet bool
	widthSet    bool
	xSet        bool
	ySet        bool
	parentIDSet bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a text item (partial)",
		Long: "Calls PATCH /v2/boards/{board_id}/texts/{item_id} with only\n" +
			"the fields you set. Pass --parent-id='' to detach from frame.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fl := cmd.Flags()
			f.contentSet = fl.Changed("content")
			f.colorSet = fl.Changed("color")
			f.fontSizeSet = fl.Changed("font-size")
			f.widthSet = fl.Changed("width")
			f.xSet = fl.Changed("x")
			f.ySet = fl.Changed("y")
			f.parentIDSet = fl.Changed("parent-id")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Text ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "New text content")
	cmd.Flags().StringVar(&f.color, "color", "", "New color (hex or named)")
	cmd.Flags().IntVar(&f.fontSize, "font-size", 0, "New font size in points")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Move to frame (empty string detaches)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	if f.itemID == "" {
		return errors.New("--item-id is required")
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("at least one field flag must be set")
	}
	path := "/v2/boards/" + f.boardID + "/texts/" + f.itemID
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

func buildUpdateRequest(f updateFlags) (updateRequest, bool) {
	var req updateRequest
	any := false

	if f.contentSet {
		req.Data = &dataField{Content: f.content}
		any = true
	}
	if f.colorSet || f.fontSizeSet {
		req.Style = &styleField{}
		if f.colorSet {
			req.Style.Color = f.color
		}
		if f.fontSizeSet {
			req.Style.FontSize = fontSizeString(f.fontSize)
		}
		any = true
	}
	if f.xSet || f.ySet {
		req.Position = &positionData{Origin: "center"}
		if f.xSet {
			req.Position.X = f.x
		}
		if f.ySet {
			req.Position.Y = f.y
		}
		any = true
	}
	if f.widthSet {
		req.Geometry = &geometryData{Width: f.width}
		any = true
	}
	if f.parentIDSet {
		req.Parent = &parentRef{ID: f.parentID}
		any = true
	}
	return req, any
}

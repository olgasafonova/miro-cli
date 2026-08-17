package codewidgets

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// moveFlags captures the per-invocation knobs for `miro codewidgets
// move`.
type moveFlags struct {
	boardID string
	itemID  string
	x       float64
	y       float64
}

func newMoveCmd(g *clictx.Globals) *cobra.Command {
	var f moveFlags
	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move a code widget to a new position",
		Long: "Calls PATCH /v2-experimental/boards/{board_id}/code_widgets/{item_id}/position\n" +
			"with the new center coordinates. Content and geometry changes go\n" +
			"through `miro codewidgets update` — position has its own endpoint.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMove(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Code widget item ID (required)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate (required)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	_ = cmd.MarkFlagRequired("x")
	_ = cmd.MarkFlagRequired("y")
	return cmd
}

func runMove(ctx context.Context, g *clictx.Globals, f moveFlags) error {
	if err := validateWidgetRef(f.boardID, f.itemID); err != nil {
		return err
	}
	path := widgetPath(f.boardID, f.itemID) + "/position"
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, moveRequest{X: f.x, Y: f.y, Origin: "center"}, &resp); err != nil {
		return wrapExperimentalErr(err)
	}
	return g.EmitJSON(resp)
}

package codewidgets

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// updateFlags tracks both the values and which fields the user
// explicitly set: Cobra zeroes unset vars, so presence is tracked via
// Changed. Position moves go through `codewidgets move`, which has its
// own dedicated endpoint.
type updateFlags struct {
	boardID     string
	itemID      string
	code        string
	language    string
	title       string
	lineNumbers bool
	width       float64
	height      float64
	parentID    string

	lineNumbersSet bool
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a code widget (partial)",
		Long: "Calls PATCH /v2-experimental/boards/{board_id}/code_widgets/{item_id}\n" +
			"with only the fields you set. At least one content, geometry, or\n" +
			"parent flag is required. To change position, use\n" +
			"`miro codewidgets move` — position has its own endpoint.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f.lineNumbersSet = cmd.Flags().Changed("line-numbers")
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Code widget item ID (required)")
	cmd.Flags().StringVar(&f.code, "code", "", "New source code (max 6000 chars)")
	cmd.Flags().StringVar(&f.language, "language", "", "New syntax-highlight language")
	cmd.Flags().StringVar(&f.title, "title", "", "New title (max 100 chars)")
	cmd.Flags().BoolVar(&f.lineNumbers, "line-numbers", false, "Show line numbers")
	cmd.Flags().Float64Var(&f.width, "width", 0, "New width in pixels")
	cmd.Flags().Float64Var(&f.height, "height", 0, "New height in pixels")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to move the widget into")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if err := validateUpdateFlags(f); err != nil {
		return err
	}
	req, ok := buildUpdateRequest(f)
	if !ok {
		return errors.New("no fields to update; supply at least one of --code, --language, --title, --line-numbers, --width, --height, or --parent-id (use `codewidgets move` to change position)")
	}
	path := widgetPath(f.boardID, f.itemID)
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, req, &resp); err != nil {
		return wrapExperimentalErr(err)
	}
	return g.EmitJSON(resp)
}

// validateUpdateFlags checks the identifier flags and the length caps
// before any request is built.
func validateUpdateFlags(f updateFlags) error {
	if err := validateWidgetRef(f.boardID, f.itemID); err != nil {
		return err
	}
	if err := validateFieldCaps(f.code, f.title); err != nil {
		return err
	}
	if f.parentID == "" {
		return nil
	}
	return miro.ValidateID("parent_id", f.parentID)
}

// buildUpdateRequest projects the updateFlags into the wire body and
// reports whether any field was set. ok=false means the caller should
// reject the update: an empty PATCH body would 400 server-side anyway,
// and a pre-flight check produces a clearer error.
func buildUpdateRequest(f updateFlags) (writeRequest, bool) {
	var lineNumbers *bool
	if f.lineNumbersSet {
		lineNumbers = &f.lineNumbers
	}
	req := writeRequest{
		Data:     buildDataBlock(f.code, f.language, f.title, lineNumbers),
		Geometry: buildGeometry(f.width, f.height),
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	ok := req.Data != nil || req.Geometry != nil || req.Parent != nil
	return req, ok
}

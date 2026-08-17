package codewidgets

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro codewidgets
// create`. lineNumbersSet tracks flag presence so an explicit
// --line-numbers=false reaches the wire while an absent flag stays out
// of the payload.
type createFlags struct {
	boardID     string
	code        string
	language    string
	title       string
	lineNumbers bool
	x           float64
	y           float64
	width       float64
	height      float64
	parentID    string

	lineNumbersSet bool
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a code widget on a board",
		Long: "Calls POST /v2-experimental/boards/{board_id}/code_widgets with\n" +
			"--code (required, max 6000 chars) and optional --language /\n" +
			"--title (max 100 chars) / --line-numbers / geometry flags.\n" +
			"Position is the widget's center.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f.lineNumbersSet = cmd.Flags().Changed("line-numbers")
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.code, "code", "", "Source code for the widget (required)")
	cmd.Flags().StringVar(&f.language, "language", "", "Syntax-highlight language (e.g. go, python, javascript)")
	cmd.Flags().StringVar(&f.title, "title", "", "Widget title")
	cmd.Flags().BoolVar(&f.lineNumbers, "line-numbers", false, "Show line numbers")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate")
	cmd.Flags().Float64Var(&f.width, "width", 0, "Width in pixels")
	cmd.Flags().Float64Var(&f.height, "height", 0, "Height in pixels")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the widget inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("code")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := validateCreateFlags(f); err != nil {
		return err
	}
	path := basePath(f.boardID)
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Post(ctx, path, buildCreateRequest(f), &resp); err != nil {
		return wrapExperimentalErr(err)
	}
	return g.EmitJSON(resp)
}

// validateCreateFlags checks required fields, ID formats, and the
// API-documented length caps before any request is built.
func validateCreateFlags(f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.code == "" {
		return errors.New("--code is required")
	}
	if err := validateFieldCaps(f.code, f.title); err != nil {
		return err
	}
	if f.parentID == "" {
		return nil
	}
	return miro.ValidateID("parent_id", f.parentID)
}

// buildDataBlock assembles the data section shared by create and
// update; nil when no data field is set.
func buildDataBlock(code, language, title string, lineNumbers *bool) map[string]any {
	data := map[string]any{}
	if code != "" {
		data["code"] = code
	}
	if language != "" {
		data["language"] = language
	}
	if title != "" {
		data["title"] = title
	}
	if lineNumbers != nil {
		data["lineNumbersVisible"] = *lineNumbers
	}
	if len(data) == 0 {
		return nil
	}
	return data
}

// buildGeometry assembles the geometry section; nil when neither
// dimension is set.
func buildGeometry(width, height float64) *geometryData {
	if width <= 0 && height <= 0 {
		return nil
	}
	return &geometryData{Width: width, Height: height}
}

// buildCreateRequest assembles the wire body. Position is always sent
// (0,0 center is a valid placement); optional sections are omitted.
func buildCreateRequest(f createFlags) writeRequest {
	var lineNumbers *bool
	if f.lineNumbersSet {
		lineNumbers = &f.lineNumbers
	}
	req := writeRequest{
		Data:     buildDataBlock(f.code, f.language, f.title, lineNumbers),
		Position: &positionData{X: f.x, Y: f.y, Origin: "center"},
		Geometry: buildGeometry(f.width, f.height),
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

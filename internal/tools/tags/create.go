package tags

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro tags create`.
type createFlags struct {
	boardID   string
	title     string
	fillColor string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tag on a board",
		Long: "Calls POST /v2/boards/{board_id}/tags with --title (required)\n" +
			"and optional --fill-color. Returns the new tag's id and metadata.\n\n" +
			"Allowed --fill-color values: red, magenta, violet, light_green,\n" +
			"green, dark_green, cyan, blue, dark_blue, black, gray, yellow.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Tag title (required, must be unique on the board)")
	cmd.Flags().StringVar(&f.fillColor, "fill-color", "", "Fill color (defaults to red server-side if omitted)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if f.boardID == "" {
		return errors.New("--board-id is required")
	}
	if f.title == "" {
		return errors.New("--title is required")
	}
	if err := validateFillColor(f.fillColor); err != nil {
		return err
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/tags"
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
	return createRequest{Title: f.title, FillColor: f.fillColor}
}

package tags

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		tagID   string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single tag",
		Long: "Calls GET /v2/boards/{board_id}/tags/{tag_id} and prints the\n" +
			"response. Returns id, title, and fillColor.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, boardID, tagID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&tagID, "tag-id", "", "Tag ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("tag-id")
	return cmd
}

func runGet(ctx context.Context, g *clictx.Globals, boardID, tagID string) error {
	if boardID == "" {
		return errors.New("--board-id is required")
	}
	if tagID == "" {
		return errors.New("--tag-id is required")
	}
	path := "/v2/boards/" + boardID + "/tags/" + tagID
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

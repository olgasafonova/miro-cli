package comments

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newGetCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID   string
		commentID string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single comment thread",
		Long: "Calls GET /v2-experimental/boards/{board_id}/comments/{comment_id}\n" +
			"and prints the thread, including its messages[] array.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), g, boardID, commentID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&commentID, "comment-id", "", "Comment thread ID (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("comment-id")
	return cmd
}

func runGet(ctx context.Context, g *clictx.Globals, boardID, commentID string) error {
	if err := validateThreadRef(boardID, commentID); err != nil {
		return err
	}
	path := threadPath(boardID, commentID)
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Get(ctx, path, &resp); err != nil {
		return wrapExperimentalErr(err)
	}
	return g.EmitJSON(resp)
}

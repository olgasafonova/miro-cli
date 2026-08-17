package comments

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// replyFlags captures the per-invocation knobs for `miro comments reply`.
type replyFlags struct {
	boardID   string
	commentID string
	content   string
}

func newReplyCmd(g *clictx.Globals) *cobra.Command {
	var f replyFlags
	cmd := &cobra.Command{
		Use:   "reply",
		Short: "Reply to an existing comment thread",
		Long: "Calls POST /v2-experimental/boards/{board_id}/comments/{comment_id}/messages\n" +
			"to append a message to the thread. The response is the updated\n" +
			"thread with the full messages[] array.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReply(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.commentID, "comment-id", "", "Comment thread ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "Reply message (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("comment-id")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func runReply(ctx context.Context, g *clictx.Globals, f replyFlags) error {
	if err := validateThreadRef(f.boardID, f.commentID); err != nil {
		return err
	}
	if f.content == "" {
		return errors.New("--content is required")
	}
	path := threadPath(f.boardID, f.commentID) + "/messages"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Post(ctx, path, replyRequest{Content: f.content}, &resp); err != nil {
		return wrapExperimentalErr(err)
	}
	return g.EmitJSON(resp)
}

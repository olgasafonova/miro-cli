package comments

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// resolveFlags captures the per-invocation knobs for `miro comments
// resolve`. reopen flips the PATCH body to {"resolved": false}.
type resolveFlags struct {
	boardID   string
	commentID string
	reopen    bool
}

func newResolveCmd(g *clictx.Globals) *cobra.Command {
	var f resolveFlags
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve (or reopen) a comment thread",
		Long: "Calls PATCH /v2-experimental/boards/{board_id}/comments/{comment_id}\n" +
			"with {\"resolved\": true}, or {\"resolved\": false} when --reopen is\n" +
			"set. The operation is reversible in both directions.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResolve(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.commentID, "comment-id", "", "Comment thread ID (required)")
	cmd.Flags().BoolVar(&f.reopen, "reopen", false, "Reopen the thread instead of resolving it")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("comment-id")
	return cmd
}

func runResolve(ctx context.Context, g *clictx.Globals, f resolveFlags) error {
	if err := validateThreadRef(f.boardID, f.commentID); err != nil {
		return err
	}
	path := threadPath(f.boardID, f.commentID)
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, resolveRequest{Resolved: !f.reopen}, &resp); err != nil {
		return wrapExperimentalErr(err)
	}
	return g.EmitJSON(resp)
}

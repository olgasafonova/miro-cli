package comments

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro comments create`.
type createFlags struct {
	boardID string
	content string
	itemID  string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open a comment thread on a board",
		Long: "Calls POST /v2-experimental/boards/{board_id}/comments with\n" +
			"--content (required). Pass --item-id to anchor the thread to an\n" +
			"item (position.type becomes \"attached\"); without it the thread\n" +
			"is board-level. The API ignores x/y placement for comments, so\n" +
			"no coordinate flags are offered.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "First message of the thread (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Item ID to attach the thread to")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("content")
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

// validateCreateFlags checks required fields and ID formats before any
// request is built.
func validateCreateFlags(f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.content == "" {
		return errors.New("--content is required")
	}
	if f.itemID == "" {
		return nil
	}
	return miro.ValidateID("item_id", f.itemID)
}

// buildCreateRequest assembles the wire body; itemId is only sent when
// the thread attaches to an item.
func buildCreateRequest(f createFlags) createRequest {
	return createRequest{Content: f.content, ItemID: f.itemID}
}

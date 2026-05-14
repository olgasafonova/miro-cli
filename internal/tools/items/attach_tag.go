package items

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newAttachTagCmd(g *clictx.Globals) *cobra.Command {
	var (
		boardID string
		itemID  string
		tagID   string
	)
	cmd := &cobra.Command{
		Use:   "attach-tag",
		Short: "Attach a tag to an item",
		Long: "Calls POST /v2/boards/{board_id}/items/{item_id}?tag_id=X.\n" +
			"Card and sticky-note items can carry up to 8 tags; other types\n" +
			"may reject the call.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAttachTag(cmd.Context(), g, boardID, itemID, tagID)
		},
	}
	cmd.Flags().StringVar(&boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&itemID, "item-id", "", "Item ID (required)")
	cmd.Flags().StringVar(&tagID, "tag-id", "", "Tag ID to attach (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	_ = cmd.MarkFlagRequired("tag-id")
	return cmd
}

func runAttachTag(ctx context.Context, g *clictx.Globals, boardID, itemID, tagID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", itemID); err != nil {
		return err
	}
	if err := miro.ValidateID("tag_id", tagID); err != nil {
		return err
	}
	q := url.Values{}
	q.Set("tag_id", tagID)
	path := "/v2/boards/" + boardID + "/items/" + itemID + "?" + q.Encode()
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	// Empty body — Miro signals the operation entirely via tag_id query.
	var resp map[string]any
	if err := client.Post(ctx, path, nil, &resp); err != nil {
		return err
	}
	if resp == nil {
		// 204-style response: synthesize an envelope so agents have a
		// deterministic JSON shape to branch on.
		resp = map[string]any{"attached": true, "item_id": itemID, "tag_id": tagID}
	}
	return g.EmitJSON(resp)
}

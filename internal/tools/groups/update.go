package groups

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// updateFlags captures the per-invocation knobs for `miro groups update`.
// Unlike most update verbs in this CLI, this is a full PUT replace, not a
// partial PATCH — the API requires the complete list of items every call.
type updateFlags struct {
	boardID string
	groupID string
	itemIDs []string
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Replace a group's members (full PUT, not partial)",
		Long: "Calls PUT /v2/boards/{board_id}/groups/{group_id} with the\n" +
			"complete list of --item-id values. The original group is\n" +
			"replaced entirely and a new group ID is assigned, so the\n" +
			"response carries an id that may differ from --group-id.\n\n" +
			"At least two --item-id flags are required, mirroring the\n" +
			"API's two-item minimum for groups.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.groupID, "group-id", "", "Group ID to replace (required)")
	cmd.Flags().StringArrayVar(&f.itemIDs, "item-id", nil, "Item ID for the new group membership (repeatable, at least 2 required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("group-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("group_id", f.groupID); err != nil {
		return err
	}
	if err := validateItemIDs(f.itemIDs); err != nil {
		return err
	}
	req := buildUpdateRequest(f)
	path := "/v2/boards/" + f.boardID + "/groups/" + f.groupID
	if g.DryRun {
		return g.EmitDryRun("PUT", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Put(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildUpdateRequest projects updateFlags into the wire body. The body
// schema is identical to createRequest — Miro reuses BoardItemGroupCreateBody
// for both endpoints.
func buildUpdateRequest(f updateFlags) updateRequest {
	return updateRequest{
		Data: createData{Items: f.itemIDs},
	}
}

package groups

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro groups create`.
// itemIDs is repeatable (`--item-id <id> --item-id <id>`). The API rejects
// fewer than 2 items; we mirror that with a pre-flight check so users see
// a clear error before the round-trip.
type createFlags struct {
	boardID string
	itemIDs []string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a group from existing items on a board",
		Long: "Calls POST /v2/boards/{board_id}/groups with at least two\n" +
			"--item-id values (repeatable). Returns the new group's id and\n" +
			"member item list.\n\n" +
			"The items remain unchanged on the canvas; only the grouping\n" +
			"relationship is created. Use `miro groups delete --group-id X`\n" +
			"to undo the grouping without removing the items.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringArrayVar(&f.itemIDs, "item-id", nil, "Item ID to include in the group (repeatable, at least 2 required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := validateItemIDs(f.itemIDs); err != nil {
		return err
	}
	req := buildCreateRequest(f)
	path := "/v2/boards/" + f.boardID + "/groups"
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

// buildCreateRequest projects createFlags into the wire body. Split out
// so tests can assert on the wire shape without an httptest server.
func buildCreateRequest(f createFlags) createRequest {
	return createRequest{
		Data: createData{Items: f.itemIDs},
	}
}

// validateItemIDs enforces the API rule that a group must contain at
// least two items. Returns nil for valid input; a descriptive error
// otherwise so the user sees a useful message before any HTTP call.
func validateItemIDs(ids []string) error {
	if len(ids) < 2 {
		return fmt.Errorf("--item-id must be provided at least twice (got %d); a group needs two or more items", len(ids))
	}
	for i, id := range ids {
		if id == "" {
			return fmt.Errorf("--item-id at position %d is empty", i)
		}
	}
	return nil
}

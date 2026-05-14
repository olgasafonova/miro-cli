package members

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// updateFlags tracks the partial-update knobs. Only role is mutable
// today, so the *Set bool dance from embeds/update.go is overkill —
// runUpdate just checks that --role is non-empty before calling.
type updateFlags struct {
	boardID  string
	memberID string
	role     string
}

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a board member's role",
		Long: "Calls PATCH /v2/boards/{board_id}/members/{board_member_id}\n" +
			"with {role: <value>}. Valid roles: viewer, commenter, editor,\n" +
			"coowner, owner, guest.\n\n" +
			"--member-id maps to the API's board_member_id path parameter.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.memberID, "member-id", "", "Board member ID (required)")
	cmd.Flags().StringVar(&f.role, "role", "", "New role (viewer|commenter|editor|coowner|owner|guest) (required)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("member-id")
	_ = cmd.MarkFlagRequired("role")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, f updateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("member_id", f.memberID); err != nil {
		return err
	}
	if f.role == "" {
		return errors.New("--role is required")
	}
	if err := validateRole(f.role); err != nil {
		return err
	}
	req := buildUpdateRequest(f)
	path := "/v2/boards/" + f.boardID + "/members/" + f.memberID
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, req, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

// buildUpdateRequest projects the updateFlags into the wire body. Split
// out so tests can assert on the JSON shape without spinning a server.
func buildUpdateRequest(f updateFlags) updateRequest {
	return updateRequest{Role: f.role}
}

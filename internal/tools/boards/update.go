package boards

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func newUpdateCmd(g *clictx.Globals) *cobra.Command {
	var req updateRequest
	cmd := &cobra.Command{
		Use:   "update <board_id>",
		Short: "Update a board's name or description",
		Long: "Calls PATCH /v2/boards/{board_id} with at least one of\n" +
			"--name / --description. Missing fields are left unchanged.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), g, args[0], req)
		},
	}
	cmd.Flags().StringVar(&req.Name, "name", "", "New board name")
	cmd.Flags().StringVar(&req.Description, "description", "", "New board description")
	return cmd
}

func runUpdate(ctx context.Context, g *clictx.Globals, boardID string, req updateRequest) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	if req.Name == "" && req.Description == "" {
		return errors.New("--name or --description must be set")
	}
	path := "/v2/boards/" + boardID
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

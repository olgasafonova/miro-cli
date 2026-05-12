package boards

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/tools/clictx"
)

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var req createRequest
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Miro board",
		Long: "Calls POST /v2/boards with --name (required) and optional\n" +
			"--description / --team-id. Returns the new board's id, name,\n" +
			"and view link.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, req)
		},
	}
	cmd.Flags().StringVar(&req.Name, "name", "", "Board name (required)")
	cmd.Flags().StringVar(&req.Description, "description", "", "Board description")
	cmd.Flags().StringVar(&req.TeamID, "team-id", "", "Owning team ID (defaults to the token's primary team)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, req createRequest) error {
	if req.Name == "" {
		return errors.New("--name is required")
	}
	const path = "/v2/boards"
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

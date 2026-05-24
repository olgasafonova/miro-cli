package boards

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newCopyCmd(g *clictx.Globals) *cobra.Command {
	var req createRequest // create-shaped body: name/description/teamId
	cmd := &cobra.Command{
		Use:   "copy <board_id>",
		Short: "Copy an existing board",
		Long: "Calls PUT /v2/boards?copy_from=<board_id>. Optional flags\n" +
			"override the copy's name, description, or owning team; omitted\n" +
			"flags inherit from the source.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd.Context(), g, args[0], req)
		},
	}
	cmd.Flags().StringVar(&req.Name, "name", "", "New name for the copy (default: inherits)")
	cmd.Flags().StringVar(&req.Description, "description", "", "New description")
	cmd.Flags().StringVar(&req.TeamID, "team-id", "", "Owning team ID")
	return cmd
}

func runCopy(ctx context.Context, g *clictx.Globals, boardID string, req createRequest) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	path := "/v2/boards?copy_from=" + url.QueryEscape(boardID)
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

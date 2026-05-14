package boards

import (
	"context"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// pictureResult is the JSON envelope emitted by `boards picture`. The
// API returns the picture URL nested inside the board resource at
// .picture.imageUrl; we lift it to the top level so agents and shell
// pipelines don't have to jq through two levels.
type pictureResult struct {
	BoardID  string `json:"board_id"`
	ImageURL string `json:"image_url"`
	Message  string `json:"message,omitempty"`
}

func newPictureCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "picture <board_id>",
		Short: "Get the preview image URL for a board",
		Long: "Calls GET /v2/boards/{board_id} and projects out the board's\n" +
			"preview image URL. Output: { board_id, image_url, message }.\n\n" +
			"Boards without a generated thumbnail return image_url=\"\" plus\n" +
			"a human-readable message; exit code is still 0 so this is not\n" +
			"an error path.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPicture(cmd.Context(), g, args[0])
		},
	}
	return cmd
}

func runPicture(ctx context.Context, g *clictx.Globals, boardID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	path := "/v2/boards/" + boardID
	if g.DryRun {
		return g.EmitDryRun("GET", path)
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Get(ctx, path, &resp); err != nil {
		return err
	}
	url := extractPictureURL(resp)
	out := pictureResult{BoardID: boardID, ImageURL: url}
	if url == "" {
		out.Message = "board has no picture available"
	}
	return g.EmitJSON(out)
}

// extractPictureURL reads board.picture.imageUrl from a Miro board
// response. Tolerates the picture object being absent (a freshly-
// created empty board has no thumbnail yet) and any of the intermediate
// types being wrong, returning "" in those cases so the verb stays
// non-fatal.
func extractPictureURL(resp map[string]any) string {
	pic, ok := resp["picture"].(map[string]any)
	if !ok {
		return ""
	}
	u, _ := pic["imageUrl"].(string)
	return u
}

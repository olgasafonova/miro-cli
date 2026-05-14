package mindmap

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// createFlags captures the per-invocation knobs for `miro mindmap create`.
type createFlags struct {
	boardID  string
	content  string
	x        float64
	y        float64
	parentID string
}

func newCreateCmd(g *clictx.Globals) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a mind map node on a board",
		Long: "Calls POST /v2-experimental/boards/{board_id}/mindmap_nodes with\n" +
			"--content (required) and optional --x / --y / --parent-id.\n" +
			"Returns the new node's id and metadata.\n\n" +
			"Tree shape: omit --parent-id to create a root node; pass\n" +
			"--parent-id to create a child of that node.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "Node text content (required)")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (board-absolute; defaults to 0,0 at board center)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Parent node ID (omit for a root node)")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func runCreate(ctx context.Context, g *clictx.Globals, f createFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if f.content == "" {
		return errors.New("--content is required")
	}
	req := buildCreateRequest(f)
	path := "/v2-experimental/boards/" + f.boardID + "/mindmap_nodes"
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

// buildCreateRequest is split out so tests can assert on the wire shape
// without spinning an httptest server. The parent envelope is omitted
// entirely when --parent-id is unset; this is how the API distinguishes
// "root node" from "child of node X."
func buildCreateRequest(f createFlags) createRequest {
	req := createRequest{
		Data: mindmapData{
			NodeView: nodeView{
				Data: nodeTextData{
					Type:    "text",
					Content: f.content,
				},
			},
		},
		Position: &positionData{X: f.x, Y: f.y, Origin: "center"},
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

package documents

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
	"miro-cli/internal/tools/uploads"
)

// updateFromFileFlags captures the knobs for `miro documents update-from-file`.
type updateFromFileFlags struct {
	boardID  string
	itemID   string
	file     string
	title    string
	x        float64
	y        float64
	parentID string
}

func newUpdateFromFileCmd(g *clictx.Globals) *cobra.Command {
	var f updateFromFileFlags
	cmd := &cobra.Command{
		Use:   "update-from-file",
		Short: "Replace a document item's file contents from disk",
		Long: "Calls PATCH /v2/boards/{board_id}/documents/{item_id} with\n" +
			"multipart/form-data to replace the existing document's bytes\n" +
			"with a file from disk. Optional --title / --x / --y /\n" +
			"--parent-id update the item's metadata in the same call.\n" +
			"Same 6 MB cap as upload.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateFromFile(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Document item ID to replace (required)")
	cmd.Flags().StringVar(&f.file, "file", "", "Local document file path (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "New document title")
	cmd.Flags().Float64Var(&f.x, "x", 0, "New X coordinate")
	cmd.Flags().Float64Var(&f.y, "y", 0, "New Y coordinate")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to move the document into")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func runUpdateFromFile(ctx context.Context, g *clictx.Globals, f updateFromFileFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", f.itemID); err != nil {
		return err
	}
	path := "/v2/boards/" + f.boardID + "/documents/" + f.itemID
	if g.DryRun {
		return g.EmitDryRun("PATCH", path)
	}
	file, err := uploads.OpenAndValidate(f.file, uploads.DocumentValidation)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	body, err := uploads.BuildMultipartBody(file, filepath.Base(f.file), uploads.Form{
		Title:    f.title,
		X:        f.x,
		Y:        f.y,
		ParentID: f.parentID,
	})
	if err != nil {
		return err
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var resp map[string]any
	if err := client.Patch(ctx, path, body, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

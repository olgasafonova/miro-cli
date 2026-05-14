package documents

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
	"miro-cli/internal/tools/uploads"
)

// uploadFlags captures the per-invocation knobs for `miro documents upload`.
type uploadFlags struct {
	boardID  string
	file     string
	title    string
	x        float64
	y        float64
	parentID string
}

func newUploadCmd(g *clictx.Globals) *cobra.Command {
	var f uploadFlags
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a local document file to a board",
		Long: "Calls POST /v2/boards/{board_id}/documents with\n" +
			"multipart/form-data to upload a file from disk. Allowed\n" +
			"extensions: pdf, doc, docx, ppt, pptx, xls, xlsx, txt, rtf,\n" +
			"csv. Max size 6 MB (Miro API limit). Use this instead of\n" +
			"`create` when the document isn't already hosted at a\n" +
			"public URL.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpload(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.file, "file", "", "Local document file path (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Document title")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the document inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func runUpload(ctx context.Context, g *clictx.Globals, f uploadFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	path := "/v2/boards/" + f.boardID + "/documents"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
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
	if err := client.Post(ctx, path, body, &resp); err != nil {
		return err
	}
	return g.EmitJSON(resp)
}

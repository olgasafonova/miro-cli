package images

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
	"miro-cli/internal/tools/uploads"
)

// uploadFlags captures the per-invocation knobs for `miro images upload`.
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
		Short: "Upload a local image file to a board",
		Long: "Calls POST /v2/boards/{board_id}/images with multipart/form-data\n" +
			"to upload a file from disk. Allowed extensions: png, jpg, jpeg,\n" +
			"gif, webp, svg. Use this instead of `create` when the image\n" +
			"isn't already hosted at a public URL.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpload(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.file, "file", "", "Local image file path (required)")
	cmd.Flags().StringVar(&f.title, "title", "", "Image title / alt text")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the image inside")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func runUpload(ctx context.Context, g *clictx.Globals, f uploadFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	path := "/v2/boards/" + f.boardID + "/images"
	if g.DryRun {
		// Dry-run reports the request shape without opening the file or
		// hitting the network. Matches the URL-variant create's dry-run.
		return g.EmitDryRun("POST", path)
	}
	file, err := uploads.OpenAndValidate(f.file, uploads.ImageValidation)
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

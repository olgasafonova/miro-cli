package documents

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// createDocFlags drives `miro documents create-doc`. Distinct from the
// URL-document create flow: --content (inline) or --content-file (read
// markdown from disk), no geometry, posts to /v2/boards/{id}/docs (not
// /documents).
//
// The miro-mcp-server exposes this as `miro_create_doc` and the URL
// variant as `miro_create_document`. The two share the documents-group
// command surface here because users think of them as "documents", but
// the wire APIs are different resources with different bodies.
type createDocFlags struct {
	boardID     string
	content     string
	contentFile string
	x           float64
	y           float64
	parentID    string
}

func newCreateDocCmd(g *clictx.Globals) *cobra.Command {
	var f createDocFlags
	cmd := &cobra.Command{
		Use:   "create-doc",
		Short: "Create a rich-text doc on a board from Markdown",
		Long: "Calls POST /v2/boards/{board_id}/docs with Markdown content.\n" +
			"Pass --content STRING for inline Markdown, or --content-file\n" +
			"PATH to read from disk. The Miro API renders the Markdown as\n" +
			"a rich-text doc item on the board.\n\n" +
			"Distinct from `documents create`: that verb attaches a remote\n" +
			"file URL via POST /v2/boards/{id}/documents. `create-doc` posts\n" +
			"raw Markdown to the separate /docs resource — different\n" +
			"endpoint, different body, no width/height.\n\n" +
			"Coordinates: when --parent-id is set, x/y are relative to the\n" +
			"frame's top-left and the doc's center is placed at (x, y). On\n" +
			"the canvas (no parent), they are absolute.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateDoc(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Target board ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "Inline Markdown content")
	cmd.Flags().StringVar(&f.contentFile, "content-file", "", "Path to a file containing Markdown content")
	cmd.Flags().Float64Var(&f.x, "x", 0, "X coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().Float64Var(&f.y, "y", 0, "Y coordinate (canvas-absolute, or frame-relative if --parent-id is set)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Frame ID to place the doc inside")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runCreateDoc(ctx context.Context, g *clictx.Globals, f createDocFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	content, err := loadDocContent(f)
	if err != nil {
		return err
	}
	req := buildCreateDocRequest(f, content)
	path := "/v2/boards/" + f.boardID + "/docs"
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

// loadDocContent reads the Markdown source from either --content or
// --content-file. Mutually exclusive; one is required.
func loadDocContent(f createDocFlags) (string, error) {
	if f.content == "" && f.contentFile == "" {
		return "", errors.New("one of --content or --content-file is required")
	}
	if f.content != "" && f.contentFile != "" {
		return "", errors.New("--content and --content-file are mutually exclusive")
	}
	if f.content != "" {
		return f.content, nil
	}
	raw, err := os.ReadFile(f.contentFile) //nolint:gosec // G304: path is operator-supplied; create-doc exists to load operator-curated Markdown
	if err != nil {
		return "", fmt.Errorf("read --content-file: %w", err)
	}
	if len(raw) == 0 {
		return "", errors.New("--content-file is empty")
	}
	return string(raw), nil
}

// buildCreateDocRequest assembles the POST body. The /docs resource
// rejects geometry, so this is the one create envelope in this package
// that has no width/height surface. Position remains explicit so
// frame-relative placement works the same as elsewhere.
func buildCreateDocRequest(f createDocFlags, content string) createDocRequest {
	req := createDocRequest{
		Data:     createDocData{ContentType: "markdown", Content: content},
		Position: &positionData{X: f.x, Y: f.y, Origin: "center"},
	}
	if f.parentID != "" {
		req.Parent = &parentRef{ID: f.parentID}
	}
	return req
}

// createDocRequest is the POST /v2/boards/{id}/docs body. Defined here
// rather than in types.go because no other verb in this package targets
// the /docs resource — the URL-document family in types.go is for the
// distinct /documents resource.
type createDocRequest struct {
	Data     createDocData `json:"data"`
	Position *positionData `json:"position,omitempty"`
	Parent   *parentRef    `json:"parent,omitempty"`
}

type createDocData struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

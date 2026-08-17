package documents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// updateDocFlags drives `miro documents update-doc`. Two content modes,
// mutually exclusive: full replacement (--content / --content-file) or
// find-and-replace (--old-content + --new-content, optionally
// --replace-all).
type updateDocFlags struct {
	boardID     string
	itemID      string
	content     string
	contentFile string
	oldContent  string
	newContent  string
	replaceAll  bool
}

// updateDocResult is the JSON envelope `update-doc` emits. The item ID
// changes on every update because the API has no PATCH for doc items —
// the verb deletes the original and recreates it at the same position.
type updateDocResult struct {
	ID       string `json:"id"`
	OldID    string `json:"old_id"`
	Replaced int    `json:"replaced,omitempty"`
	Message  string `json:"message"`
}

// updateDocRecovery is emitted when the recreate step fails after the
// original was already deleted: it carries the resolved content so
// nothing is lost.
type updateDocRecovery struct {
	OldID   string `json:"old_id"`
	Content string `json:"content"`
	Message string `json:"message"`
}

func newUpdateDocCmd(g *clictx.Globals) *cobra.Command {
	var f updateDocFlags
	cmd := &cobra.Command{
		Use:   "update-doc",
		Short: "Update a rich-text doc's Markdown content (delete + recreate)",
		Long: "The Miro API has no PATCH for doc items, so this verb reads the\n" +
			"doc at GET /v2/boards/{board_id}/docs/{item_id}, deletes the\n" +
			"original, and recreates it at the same position with the new\n" +
			"content. The item ID changes; the response carries both ids.\n\n" +
			"Content modes (mutually exclusive):\n" +
			"  --content / --content-file      full replacement\n" +
			"  --old-content [--new-content]   find-and-replace (first match,\n" +
			"                                  or every match with --replace-all)\n\n" +
			"Destructive under the hood (the original is deleted): refuses\n" +
			"without --yes (or --agent). If the recreate step fails after the\n" +
			"delete, the resolved content is emitted so nothing is lost.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateDoc(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemID, "item-id", "", "Doc item ID (required)")
	cmd.Flags().StringVar(&f.content, "content", "", "New Markdown content (full replacement)")
	cmd.Flags().StringVar(&f.contentFile, "content-file", "", "Path to a file with the new Markdown content")
	cmd.Flags().StringVar(&f.oldContent, "old-content", "", "Text to find (find-and-replace mode)")
	cmd.Flags().StringVar(&f.newContent, "new-content", "", "Replacement text (empty removes the match)")
	cmd.Flags().BoolVar(&f.replaceAll, "replace-all", false, "Replace every occurrence instead of the first")
	_ = cmd.MarkFlagRequired("board-id")
	_ = cmd.MarkFlagRequired("item-id")
	cmd.MarkFlagsMutuallyExclusive("content", "old-content")
	cmd.MarkFlagsMutuallyExclusive("content-file", "old-content")
	return cmd
}

func runUpdateDoc(ctx context.Context, g *clictx.Globals, f updateDocFlags) error {
	if err := validateUpdateDocFlags(f); err != nil {
		return err
	}
	path := docPath(f.boardID, f.itemID)
	if g.DryRun {
		return g.EmitDryRun("GET+DELETE+POST", path+" (read, delete, recreate with new content)")
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "update-doc deletes and recreates the doc; pass --yes to confirm or --dry-run to preview"}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	var current map[string]any
	if err := client.Get(ctx, path, &current); err != nil {
		return fmt.Errorf("failed to read current doc: %w", err)
	}
	newContent, replaced, err := resolveNewContent(docContent(current), f)
	if err != nil {
		return err
	}
	x, y := docPosition(current)
	return recreateDoc(ctx, g, client, docRecreation{
		flags: f, x: x, y: y, content: newContent, replaced: replaced,
	})
}

// validateUpdateDocFlags checks the identifiers and that exactly one
// content mode is selected. The old-content/content exclusivity is
// enforced by Cobra flag groups; this covers the run-function callers
// (tests) and the "no mode at all" case.
func validateUpdateDocFlags(f updateDocFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	if err := miro.ValidateID("item_id", f.itemID); err != nil {
		return err
	}
	if f.oldContent != "" {
		return nil
	}
	if f.content == "" && f.contentFile == "" {
		return errors.New("supply --content / --content-file (full replace) or --old-content (find-and-replace)")
	}
	return nil
}

// resolveNewContent picks the doc's next content: find-and-replace over
// the current content, or a full replacement from --content /
// --content-file. Returns how many occurrences were replaced (0 for
// full replacement).
func resolveNewContent(current string, f updateDocFlags) (string, int, error) {
	if f.oldContent != "" {
		return findAndReplace(current, f)
	}
	content, err := loadDocContent(createDocFlags{content: f.content, contentFile: f.contentFile})
	return content, 0, err
}

// findAndReplace applies the --old-content / --new-content pair to the
// current content, failing loudly when the needle is absent so a typo
// doesn't silently recreate the doc unchanged.
func findAndReplace(current string, f updateDocFlags) (string, int, error) {
	if !strings.Contains(current, f.oldContent) {
		return "", 0, errors.New("--old-content not found in the document")
	}
	if f.replaceAll {
		count := strings.Count(current, f.oldContent)
		return strings.ReplaceAll(current, f.oldContent, f.newContent), count, nil
	}
	return strings.Replace(current, f.oldContent, f.newContent, 1), 1, nil
}

// docRecreation carries everything the delete-and-recreate step needs:
// the identifying flags, the original position, and the resolved
// content with its replacement count.
type docRecreation struct {
	flags    updateDocFlags
	x, y     float64
	content  string
	replaced int
}

// recreateDoc deletes the original doc and posts a replacement at the
// same position. On a recreate failure after the delete, the resolved
// content is emitted before the error returns so it stays recoverable.
func recreateDoc(ctx context.Context, g *clictx.Globals, client *miro.Client, rec docRecreation) error {
	f := rec.flags
	if err := client.Delete(ctx, "/v2/boards/"+f.boardID+"/items/"+f.itemID); err != nil {
		return fmt.Errorf("failed to delete original doc: %w", err)
	}

	req := createDocRequest{
		Data:     createDocData{ContentType: "markdown", Content: rec.content},
		Position: &positionData{X: rec.x, Y: rec.y, Origin: "center"},
	}
	var resp map[string]any
	if err := client.Post(ctx, "/v2/boards/"+f.boardID+"/docs", req, &resp); err != nil {
		_ = g.EmitJSON(updateDocRecovery{
			OldID:   f.itemID,
			Content: rec.content,
			Message: "original deleted but recreate failed; content preserved in this output",
		})
		return fmt.Errorf("failed to recreate doc with updated content: %w", err)
	}

	newID, _ := resp["id"].(string)
	return g.EmitJSON(updateDocResult{
		ID:       newID,
		OldID:    f.itemID,
		Replaced: rec.replaced,
		Message:  updateDocMessage(rec.replaced),
	})
}

// updateDocMessage describes the outcome of a doc update.
func updateDocMessage(replaced int) string {
	if replaced > 0 {
		return fmt.Sprintf("Replaced %d occurrence(s); doc recreated under a new id", replaced)
	}
	return "Doc content replaced; doc recreated under a new id"
}

// docContent reads data.content from a doc response, tolerating absent
// or wrongly-typed intermediates.
func docContent(resp map[string]any) string {
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		return ""
	}
	s, _ := data["content"].(string)
	return s
}

// docPosition reads position.x/y from a doc response so the recreate
// lands where the original stood.
func docPosition(resp map[string]any) (x, y float64) {
	position, _ := resp["position"].(map[string]any)
	if position == nil {
		return 0, 0
	}
	x, _ = position["x"].(float64)
	y, _ = position["y"].(float64)
	return x, y
}

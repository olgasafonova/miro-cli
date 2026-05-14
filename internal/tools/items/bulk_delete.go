package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// bulkDeleteFlags drives `items bulk-delete`. Mirrors bulk-create's flag
// pattern (file or inline JSON) and adds the comma-separated --ids
// shorthand that's idiomatic for "I have a handful of IDs in a shell
// pipeline". Exactly one of the three input flags must be set.
type bulkDeleteFlags struct {
	boardID string
	ids     string // comma-separated
	idsFile string
	idsJSON string
}

func newBulkDeleteCmd(g *clictx.Globals) *cobra.Command {
	var f bulkDeleteFlags
	cmd := &cobra.Command{
		Use:   "bulk-delete",
		Short: "Delete many items by ID (destructive)",
		Long: "Calls DELETE /v2/boards/{board_id}/items/{item_id} once per ID,\n" +
			"in order, and emits an aggregate {requested, deleted, failed,\n" +
			"results[]} envelope. There is no native Miro bulk-delete\n" +
			"endpoint; this command is a serial loop over the regular delete\n" +
			"verb so callers can drive it from a shell pipeline.\n\n" +
			"Inputs: pass --ids as a comma-separated list, --ids-json as an\n" +
			"inline JSON array, or --ids-file as a path to a JSON array.\n" +
			"Exactly one is required.\n\n" +
			"Destructive: refuses without --yes (or --agent, which implies\n" +
			"--yes). Use --dry-run to preview without sending.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBulkDelete(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.ids, "ids", "", "Comma-separated list of item IDs")
	cmd.Flags().StringVar(&f.idsFile, "ids-file", "", "Path to a JSON array of item IDs")
	cmd.Flags().StringVar(&f.idsJSON, "ids-json", "", "Inline JSON array of item IDs")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runBulkDelete(ctx context.Context, g *clictx.Globals, f bulkDeleteFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	ids, err := loadIDs(f)
	if err != nil {
		return err
	}
	if g.DryRun {
		// Preview the first path so the user can confirm shape; the
		// envelope itself is just len(ids) of these.
		return g.EmitDryRun("DELETE", "/v2/boards/"+f.boardID+"/items/{item_id} x "+itoa(len(ids)))
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: fmt.Sprintf("refusing to bulk-delete %d items without --yes; pass --yes to confirm or --dry-run to preview", len(ids))}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	out := bulkOpResponse{
		BoardID:   f.boardID,
		Requested: len(ids),
		Results:   make([]bulkOpResult, 0, len(ids)),
	}
	for _, id := range ids {
		if cerr := ctx.Err(); cerr != nil {
			out.Results = append(out.Results, bulkOpResult{ID: id, Status: "error", Error: cerr.Error()})
			out.Failed++
			continue
		}
		path := "/v2/boards/" + f.boardID + "/items/" + id
		if derr := client.Delete(ctx, path); derr != nil {
			out.Results = append(out.Results, bulkOpResult{ID: id, Status: "error", Error: derr.Error()})
			out.Failed++
			continue
		}
		out.Results = append(out.Results, bulkOpResult{ID: id, Status: "success"})
		out.Succeeded++
	}
	return g.EmitJSON(out)
}

// loadIDs parses the three input flags into a single ID slice, enforcing
// exactly-one. Empty / duplicate IDs are kept as-is so the per-call API
// surfaces the same error a single delete would.
func loadIDs(f bulkDeleteFlags) ([]string, error) {
	set := 0
	if f.ids != "" {
		set++
	}
	if f.idsFile != "" {
		set++
	}
	if f.idsJSON != "" {
		set++
	}
	if set == 0 {
		return nil, errors.New("one of --ids, --ids-file, or --ids-json is required")
	}
	if set > 1 {
		return nil, errors.New("--ids, --ids-file, and --ids-json are mutually exclusive")
	}

	if f.ids != "" {
		out := splitTrim(f.ids)
		if len(out) == 0 {
			return nil, errors.New("--ids parsed to an empty list")
		}
		return out, nil
	}

	var raw []byte
	if f.idsFile != "" {
		var err error
		raw, err = os.ReadFile(f.idsFile) //nolint:gosec // G304: path is operator-supplied; bulk verbs exist to load operator-curated payloads
		if err != nil {
			return nil, fmt.Errorf("read --ids-file: %w", err)
		}
	} else {
		raw = []byte(f.idsJSON)
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parse ids JSON as array of strings: %w", err)
	}
	if len(arr) == 0 {
		return nil, errors.New("ids array is empty")
	}
	return arr, nil
}

// splitTrim splits s on commas and trims whitespace, dropping empties.
func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// itoa wraps strconv.Itoa via fmt to avoid importing strconv here.
// Tiny helper; intentionally local rather than a util package.
func itoa(n int) string { return fmt.Sprintf("%d", n) }

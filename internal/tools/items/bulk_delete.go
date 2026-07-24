package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
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
	cmd.Flags().StringVar(&f.idsFile, "ids-file", "", "Path to a JSON array of item IDs (use - to read from stdin)")
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
		return g.EmitDryRun("DELETE", "/v2/boards/"+f.boardID+"/items/{item_id} x "+strconv.Itoa(len(ids)))
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: fmt.Sprintf("refusing to bulk-delete %d items without --yes; pass --yes to confirm or --dry-run to preview", len(ids))}
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	results := miro.FanOut(ctx, ids, g.Concurrency, bulkDeleteWorker(client, f.boardID))
	return g.EmitJSON(tallyBulk(f.boardID, results))
}

// bulkDeleteWorker returns the per-ID FanOut callback. Each call
// validates the ID before it is spliced into the URL path, so a
// malformed ID fails item-locally instead of reaching the API.
func bulkDeleteWorker(client *miro.Client, boardID string) func(context.Context, int, string) bulkOpResult {
	return func(ctx context.Context, i int, id string) bulkOpResult {
		if cerr := ctx.Err(); cerr != nil {
			return bulkOpResult{ID: id, Status: "error", Error: cerr.Error()}
		}
		if verr := miro.ValidateID("id", id); verr != nil {
			return bulkOpResult{ID: id, Status: "error", Error: fmt.Sprintf("ids[%d]: %s", i, verr)}
		}
		path := "/v2/boards/" + boardID + "/items/" + id
		if derr := client.Delete(ctx, path); derr != nil {
			return bulkOpResult{ID: id, Status: "error", Error: derr.Error()}
		}
		return bulkOpResult{ID: id, Status: "success"}
	}
}

// loadIDs parses the three input flags into a single ID slice, enforcing
// exactly-one. Empty / duplicate IDs are kept as-is so the per-call API
// surfaces the same error a single delete would.
func loadIDs(f bulkDeleteFlags) ([]string, error) {
	if err := validateIDSource(f); err != nil {
		return nil, err
	}
	if f.ids != "" {
		return parseCommaIDs(f.ids)
	}
	return parseJSONIDs(f)
}

// validateIDSource enforces that exactly one of the three ID input flags
// is set. bulk-delete has a third source (--ids shorthand) on top of the
// file/inline pair, so it carries its own three-way messages instead of
// requireExactlyOneSource.
func validateIDSource(f bulkDeleteFlags) error {
	set := 0
	for _, v := range []string{f.ids, f.idsFile, f.idsJSON} {
		if v != "" {
			set++
		}
	}
	if set == 0 {
		return errors.New("one of --ids, --ids-file, or --ids-json is required")
	}
	if set > 1 {
		return errors.New("--ids, --ids-file, and --ids-json are mutually exclusive")
	}
	return nil
}

// parseCommaIDs parses the --ids comma-separated shorthand.
func parseCommaIDs(s string) ([]string, error) {
	out := splitTrim(s)
	if len(out) == 0 {
		return nil, errors.New("--ids parsed to an empty list")
	}
	return out, nil
}

// parseJSONIDs decodes the JSON-array form from --ids-file or --ids-json.
func parseJSONIDs(f bulkDeleteFlags) ([]string, error) {
	raw, err := readRawJSONSource("ids-file", f.idsFile, f.idsJSON)
	if err != nil {
		return nil, err
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

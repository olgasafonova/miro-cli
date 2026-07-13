package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// bulkCreateFlags drives `items bulk-create`. The wire body is a JSON
// array of typed-item create payloads, so we accept either a file path
// (--items-file) or an inline string (--items-json). Tests use the
// inline form to avoid filesystem dependencies.
type bulkCreateFlags struct {
	boardID   string
	itemsFile string
	itemsJSON string
}

func newBulkCreateCmd(g *clictx.Globals) *cobra.Command {
	var f bulkCreateFlags
	cmd := &cobra.Command{
		Use:   "bulk-create",
		Short: "Create up to 20 items in one call",
		Long: "Calls POST /v2/boards/{board_id}/items/bulk. The request body\n" +
			"is a JSON array of typed-item create payloads (sticky, shape,\n" +
			"text, etc.). Pass --items-file PATH to read the array from a\n" +
			"file, or --items-json STRING to inline it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBulkCreate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.itemsFile, "items-file", "", "Path to a JSON file containing the items array (use - to read from stdin)")
	cmd.Flags().StringVar(&f.itemsJSON, "items-json", "", "Inline JSON array of items")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runBulkCreate(ctx context.Context, g *clictx.Globals, f bulkCreateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	body, err := loadBulkItems(f)
	if err != nil {
		return err
	}
	path := "/v2/boards/" + f.boardID + "/items/bulk"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
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

// loadBulkItems reads the items payload from --items-file or
// --items-json and validates it parses as a JSON array. The validation
// catches obvious shape errors before the request hits the network, and
// also rejects the empty-string / both-flags-set cases the API would
// 400 on anyway.
func loadBulkItems(f bulkCreateFlags) ([]json.RawMessage, error) {
	if f.itemsFile == "" && f.itemsJSON == "" {
		return nil, errors.New("one of --items-file or --items-json is required")
	}
	if f.itemsFile != "" && f.itemsJSON != "" {
		return nil, errors.New("--items-file and --items-json are mutually exclusive")
	}
	var raw []byte
	if f.itemsFile != "" {
		var err error
		raw, err = clictx.ReadFileOrStdin(f.itemsFile)
		if err != nil {
			return nil, fmt.Errorf("read --items-file: %w", err)
		}
	} else {
		raw = []byte(f.itemsJSON)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parse items JSON as array: %w", err)
	}
	if len(arr) == 0 {
		return nil, errors.New("items array is empty")
	}
	return arr, nil
}

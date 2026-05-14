package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// bulkUpdateFlags drives `items bulk-update`. The wire body is a JSON
// array of per-item patches; each entry must include an "id" plus any
// subset of {x, y, width, height, parent_id}. Absent keys leave the
// corresponding field untouched. Pointer types preserve absent-vs-zero,
// which matters because Miro treats an empty parent_id as "detach".
type bulkUpdateFlags struct {
	boardID     string
	patchesFile string
	patchesJSON string
}

// bulkUpdateItem is one entry in the --patches-* JSON array. The
// per-field pointers distinguish "user set this to zero" from "user
// omitted the key". parent_id is a *string so the empty-string-detaches
// semantic from update.go carries through.
type bulkUpdateItem struct {
	ID       string   `json:"id"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
	Width    *float64 `json:"width,omitempty"`
	Height   *float64 `json:"height,omitempty"`
	ParentID *string  `json:"parent_id,omitempty"`
}

func newBulkUpdateCmd(g *clictx.Globals) *cobra.Command {
	var f bulkUpdateFlags
	cmd := &cobra.Command{
		Use:   "bulk-update",
		Short: "Update many items in one call (serial PATCH loop)",
		Long: "Calls PATCH /v2/boards/{board_id}/items/{item_id} once per\n" +
			"patch in the input array, in order, and emits an aggregate\n" +
			"{requested, succeeded, failed, results[]} envelope. There is\n" +
			"no native Miro bulk-update endpoint; this command is a serial\n" +
			"loop over the regular update verb.\n\n" +
			"Patch shape: each array entry is a JSON object with an \"id\"\n" +
			"and any subset of {x, y, width, height, parent_id}. Absent\n" +
			"keys leave the corresponding field untouched. parent_id=\"\"\n" +
			"detaches the item from its frame, matching `items update`.\n\n" +
			"Inputs: pass --patches-json as an inline JSON array or\n" +
			"--patches-file as a path to a JSON array. Exactly one is\n" +
			"required.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBulkUpdate(cmd.Context(), g, f)
		},
	}
	cmd.Flags().StringVar(&f.boardID, "board-id", "", "Board ID (required)")
	cmd.Flags().StringVar(&f.patchesFile, "patches-file", "", "Path to a JSON array of patch objects")
	cmd.Flags().StringVar(&f.patchesJSON, "patches-json", "", "Inline JSON array of patch objects")
	_ = cmd.MarkFlagRequired("board-id")
	return cmd
}

func runBulkUpdate(ctx context.Context, g *clictx.Globals, f bulkUpdateFlags) error {
	if err := miro.ValidateID("board_id", f.boardID); err != nil {
		return err
	}
	patches, err := loadPatches(f)
	if err != nil {
		return err
	}
	if g.DryRun {
		return g.EmitDryRun("PATCH", "/v2/boards/"+f.boardID+"/items/{item_id} x "+itoa(len(patches)))
	}
	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	out := bulkOpResponse{
		BoardID:   f.boardID,
		Requested: len(patches),
		Results:   make([]bulkOpResult, 0, len(patches)),
	}
	for i, p := range patches {
		if cerr := ctx.Err(); cerr != nil {
			out.Results = append(out.Results, bulkOpResult{ID: p.ID, Status: "error", Error: cerr.Error()})
			out.Failed++
			continue
		}
		if p.ID == "" {
			out.Results = append(out.Results, bulkOpResult{ID: p.ID, Status: "error", Error: fmt.Sprintf("patches[%d]: missing \"id\"", i)})
			out.Failed++
			continue
		}
		body, ok := buildBulkUpdateBody(p)
		if !ok {
			out.Results = append(out.Results, bulkOpResult{ID: p.ID, Status: "error", Error: "no mutable fields set"})
			out.Failed++
			continue
		}
		path := "/v2/boards/" + f.boardID + "/items/" + p.ID
		var resp map[string]any
		if perr := client.Patch(ctx, path, body, &resp); perr != nil {
			out.Results = append(out.Results, bulkOpResult{ID: p.ID, Status: "error", Error: perr.Error()})
			out.Failed++
			continue
		}
		out.Results = append(out.Results, bulkOpResult{ID: p.ID, Status: "success"})
		out.Succeeded++
	}
	return g.EmitJSON(out)
}

// loadPatches reads the patches array from --patches-file or
// --patches-json and decodes it. Exactly-one enforcement matches
// bulk-delete's loadIDs.
func loadPatches(f bulkUpdateFlags) ([]bulkUpdateItem, error) {
	if f.patchesFile == "" && f.patchesJSON == "" {
		return nil, errors.New("one of --patches-file or --patches-json is required")
	}
	if f.patchesFile != "" && f.patchesJSON != "" {
		return nil, errors.New("--patches-file and --patches-json are mutually exclusive")
	}
	var raw []byte
	if f.patchesFile != "" {
		var err error
		raw, err = os.ReadFile(f.patchesFile) //nolint:gosec // G304: path is operator-supplied; bulk verbs exist to load operator-curated payloads
		if err != nil {
			return nil, fmt.Errorf("read --patches-file: %w", err)
		}
	} else {
		raw = []byte(f.patchesJSON)
	}
	var arr []bulkUpdateItem
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parse patches JSON as array: %w", err)
	}
	if len(arr) == 0 {
		return nil, errors.New("patches array is empty")
	}
	return arr, nil
}

// buildBulkUpdateBody projects one bulkUpdateItem into the PATCH wire
// body shared by `items update`. Returns ok=false when no mutable
// fields are set so the caller can skip the HTTP round-trip; Miro 400s
// an empty PATCH anyway, and the pre-flight skip yields a clearer
// per-item error.
func buildBulkUpdateBody(p bulkUpdateItem) (updateRequest, bool) {
	var req updateRequest
	any := false

	if p.X != nil || p.Y != nil {
		req.Position = &positionData{Origin: "center"}
		if p.X != nil {
			req.Position.X = *p.X
		}
		if p.Y != nil {
			req.Position.Y = *p.Y
		}
		any = true
	}
	if p.Width != nil || p.Height != nil {
		req.Geometry = &geometryData{}
		if p.Width != nil {
			req.Geometry.Width = *p.Width
		}
		if p.Height != nil {
			req.Geometry.Height = *p.Height
		}
		any = true
	}
	if p.ParentID != nil {
		// Same detach semantic as `items update`: empty string detaches,
		// non-empty re-parents. Both paths emit the envelope.
		req.Parent = &parentRef{ID: *p.ParentID}
		any = true
	}
	return req, any
}

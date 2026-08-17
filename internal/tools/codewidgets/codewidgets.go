// Package codewidgets holds the hand-authored Cobra subcommands for the
// /v2-experimental/boards/{board_id}/code_widgets endpoint family:
// create, list, get, update, move, delete.
//
// Note: code widgets live under /v2-experimental, not /v2; the API
// surface may evolve before promotion. Position moves have a dedicated
// endpoint (PATCH .../code_widgets/{item_id}/position), so `move` is a
// separate verb rather than an update field.
package codewidgets

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// maxCodeLength is the API's cap on the code field (spec:
// CodeWidgetData.code maxLength).
const maxCodeLength = 6000

// maxTitleLength is the API's cap on the title field (spec:
// CodeWidgetData.title maxLength).
const maxTitleLength = 100

// NewCmd returns the `codewidgets` parent command covering the
// /v2-experimental/boards/{board_id}/code_widgets family.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codewidgets",
		Short: "Manage code widget items on a Miro board (v2-experimental)",
	}
	cmd.AddCommand(
		newListCmd(g),
		newCreateCmd(g),
		newGetCmd(g),
		newUpdateCmd(g),
		newMoveCmd(g),
		newDeleteCmd(g),
	)
	return cmd
}

// basePath is the code_widgets collection path for a board.
func basePath(boardID string) string {
	return "/v2-experimental/boards/" + boardID + "/code_widgets"
}

// widgetPath addresses one code widget.
func widgetPath(boardID, itemID string) string {
	return basePath(boardID) + "/" + itemID
}

// wrapExperimentalErr adds the experimental-availability hint to
// ambiguous 403/404 responses: on a v2-experimental endpoint they can
// mean "your plan doesn't have this API" as easily as "wrong ID".
// fmt.Errorf's %w keeps errors.As working, so miro.ExitCode still maps
// the underlying status.
func wrapExperimentalErr(err error) error {
	var apiErr *miro.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.Status == http.StatusForbidden || apiErr.Status == http.StatusNotFound {
		return fmt.Errorf("%w (note: code_widgets is a v2-experimental Miro API and may be unavailable for your account or plan)", err)
	}
	return err
}

// validateWidgetRef checks the board/item ID pair shared by get,
// update, move, and delete.
func validateWidgetRef(boardID, itemID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	return miro.ValidateID("item_id", itemID)
}

// validateFieldCaps checks the API-documented length caps shared by
// create and update.
func validateFieldCaps(code, title string) error {
	if len(code) > maxCodeLength {
		return fmt.Errorf("--code exceeds %d characters (got %d); split the snippet across multiple widgets", maxCodeLength, len(code))
	}
	if len(title) > maxTitleLength {
		return fmt.Errorf("--title exceeds %d characters (got %d)", maxTitleLength, len(title))
	}
	return nil
}

// Package comments holds the hand-authored Cobra subcommands for the
// /v2-experimental/boards/{board_id}/comments endpoint family: create,
// list, get, reply, resolve.
//
// The comments endpoints are live on the v2-experimental API but absent
// from Miro's OpenAPI spec (verified by live probe 13-08-2026). Wire
// facts the implementation relies on:
//   - POST  /boards/{id}/comments                opens a thread; content required
//   - GET   /boards/{id}/comments                offset-paginated envelope
//   - GET   /boards/{id}/comments/{cid}          one thread with messages[]
//   - POST  /boards/{id}/comments/{cid}/messages appends a reply
//   - PATCH /boards/{id}/comments/{cid}          {"resolved":bool} both directions
//   - DELETE returns 405, so no delete verb is offered
//   - x/y in the create body are accepted but ignored (comment lands at
//     0,0); itemId genuinely anchors the thread (position.type becomes
//     "attached"), so --item-id is the only placement flag exposed
package comments

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// NewCmd returns the `comments` parent command with the five thread
// verbs. There is no delete: the API answers 405.
func NewCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comments",
		Short: "Manage comment threads on a Miro board (v2-experimental)",
	}
	cmd.AddCommand(
		newCreateCmd(g),
		newListCmd(g),
		newGetCmd(g),
		newReplyCmd(g),
		newResolveCmd(g),
	)
	return cmd
}

// basePath is the comments collection path for a board.
func basePath(boardID string) string {
	return "/v2-experimental/boards/" + boardID + "/comments"
}

// threadPath addresses one comment thread.
func threadPath(boardID, commentID string) string {
	return basePath(boardID) + "/" + commentID
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
		return fmt.Errorf("%w (note: comments is a v2-experimental Miro API and may be unavailable for your account or plan)", err)
	}
	return err
}

// validateThreadRef checks the board/comment ID pair shared by get,
// reply, and resolve.
func validateThreadRef(boardID, commentID string) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	return miro.ValidateID("comment_id", commentID)
}

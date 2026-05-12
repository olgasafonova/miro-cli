package boards

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// shareDeps holds the side-channel dependencies the share verb needs.
// Pulled into a struct so tests can inject a custom allowlist without
// reaching for env vars.
type shareDeps struct {
	allowlist *miro.ShareAllowlist
}

func newShareCmd(g *clictx.Globals) *cobra.Command {
	var (
		email   string
		role    string
		message string
	)
	cmd := &cobra.Command{
		Use:   "share <board_id>",
		Short: "Invite a user to a board by email (destructive)",
		Long: "Calls POST /v2/boards/{board_id}/members with the given email +\n" +
			"role. Sharing a board grants access to an external party, so this\n" +
			"verb is gated:\n\n" +
			"  1. The email's domain must appear in MIRO_SHARE_ALLOWED_DOMAINS\n" +
			"     (comma-separated). An unset env var blocks every share with\n" +
			"     a clear error — fail-closed.\n" +
			"  2. --yes (or --agent, which implies --yes) is required, since\n" +
			"     sharing is destructive in the same sense delete is.\n\n" +
			"Use --dry-run to preview without sending. --dry-run still runs the\n" +
			"allowlist check so you can validate config before issuing the real\n" +
			"call.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShare(cmd.Context(), g, shareDeps{}, args[0], shareRequest{
				Emails:  []string{email},
				Role:    role,
				Message: message,
			})
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Invitee email address (required)")
	cmd.Flags().StringVar(&role, "role", "viewer", "Role to grant (viewer|commenter|editor)")
	cmd.Flags().StringVar(&message, "message", "", "Optional invitation message")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

// runShare is the testable entry point. deps.allowlist may be nil, in
// which case we lazily load from the env. Tests inject a pre-built
// allowlist to avoid depending on process-wide env state.
func runShare(ctx context.Context, g *clictx.Globals, deps shareDeps, boardID string, req shareRequest) error {
	if boardID == "" {
		return errors.New("board_id is required")
	}
	if len(req.Emails) != 1 || req.Emails[0] == "" {
		return errors.New("--email is required")
	}
	if err := validateShareRole(req.Role); err != nil {
		return err
	}

	allowlist := deps.allowlist
	if allowlist == nil {
		allowlist = miro.LoadShareAllowlistFromEnv()
	}
	if err := allowlist.Validate(req.Emails[0]); err != nil {
		// Map allowlist refusals to ExitConfig (10) rather than the
		// default ExitAPI (5): the user can't fix this by retrying,
		// they need to adjust MIRO_SHARE_ALLOWED_DOMAINS (or accept
		// the gate). Matches the delete-without-yes refusal shape.
		return &miro.ConfigError{Reason: err.Error()}
	}

	path := "/v2/boards/" + boardID + "/members"
	if g.DryRun {
		return g.EmitDryRun("POST", path)
	}
	if !g.Yes {
		return &miro.ConfigError{Reason: "refusing to share board without --yes; pass --yes to confirm or --dry-run to preview"}
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

// validateShareRole guards the verb against shipping unsupported roles
// to the API. The API would 400 anyway; we just produce a friendlier
// error here.
func validateShareRole(role string) error {
	switch role {
	case "viewer", "commenter", "editor":
		return nil
	default:
		return fmt.Errorf("invalid --role %q: must be viewer, commenter, or editor", role)
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// authStatus is the machine-readable shape emitted by `miro auth status`.
// It is deliberately honest about how much was checked: Verified is only
// true after a successful --verify round-trip, so an agent can tell "a
// token is configured" from "the token currently works."
type authStatus struct {
	TokenPresent bool   `json:"token_present"`
	Source       string `json:"source"` // flag | env | none
	Verified     bool   `json:"verified"`
	Status       string `json:"status"` // ok | token_present | no_token | invalid_or_expired | insufficient_scope | error
}

// newAuthCmd groups authentication-inspection subcommands. There is no
// `auth login`: the CLI authenticates via --token / $MIRO_ACCESS_TOKEN,
// so status is the only affordance an agent needs (Agent-Friendly CLI
// Checklist §7).
func newAuthCmd(g *clictx.Globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect authentication state",
	}
	cmd.AddCommand(newAuthStatusCmd(g))
	return cmd
}

func newAuthStatusCmd(g *clictx.Globals) *cobra.Command {
	var verify bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report whether a Miro token is configured (and optionally valid)",
		Long: "Report authentication state without performing any board operation.\n\n" +
			"By default this is a local, network-free check: it reports whether a\n" +
			"token is present and where it came from (--token flag or the\n" +
			"$MIRO_ACCESS_TOKEN environment variable). Pass --verify to make one\n" +
			"lightweight authenticated request and confirm the token is currently\n" +
			"valid, distinguishing an invalid/expired token from insufficient scope.\n\n" +
			"Exit codes: 0 = token present (and, with --verify, valid); 10 = no\n" +
			"token configured; 4 = --verify rejected by the API.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthStatus(cmd.Context(), g, verify)
		},
	}
	cmd.Flags().BoolVar(&verify, "verify", false, "Make a lightweight authenticated request to confirm the token is currently valid")
	return cmd
}

func runAuthStatus(ctx context.Context, g *clictx.Globals, verify bool) error {
	st := authStatus{}
	st.TokenPresent, st.Source = detectToken(g)

	if !st.TokenPresent {
		st.Status = "no_token"
		_ = emitAuthStatus(g, st)
		return &miro.ConfigError{Reason: "no token: pass --token or set " + miro.EnvAccessToken}
	}

	if !verify {
		st.Status = "token_present"
		return emitAuthStatus(g, st)
	}

	return verifyToken(ctx, g, st)
}

// detectToken reports whether a token is configured and where it came
// from, without touching the network. The --token flag wins over the
// environment variable, mirroring the client's own config precedence.
func detectToken(g *clictx.Globals) (present bool, source string) {
	if strings.TrimSpace(g.Token) != "" {
		return true, "flag"
	}
	if strings.TrimSpace(os.Getenv(miro.EnvAccessToken)) != "" {
		return true, "env"
	}
	return false, "none"
}

// verifyToken performs the --verify probe and emits the outcome.
// GET /v1/oauth-token returns metadata about the access token itself.
// It is read-only and cheap, which makes it the right probe for a
// verify check that must not cause side effects.
func verifyToken(ctx context.Context, g *clictx.Globals, st authStatus) error {
	client, err := g.BuildClient()
	if err != nil {
		return err
	}
	var out any
	verr := client.Get(ctx, "/v1/oauth-token", &out)
	if verr == nil {
		st.Verified = true
		st.Status = "ok"
		return emitAuthStatus(g, st)
	}

	st.Status = classifyVerifyError(verr)
	_ = emitAuthStatus(g, st)
	return verr
}

// classifyVerifyError maps a failed verify probe onto the machine-readable
// status vocabulary: 401 means the token itself is bad, 403 means it works
// but lacks scope, anything else is an unclassified error.
func classifyVerifyError(verr error) string {
	var api *miro.APIError
	if !errors.As(verr, &api) {
		return "error"
	}
	switch api.Status {
	case 401:
		return "invalid_or_expired"
	case 403:
		return "insufficient_scope"
	default:
		return "error"
	}
}

// emitAuthStatus writes the status to stdout: JSON under --json/--agent,
// otherwise a short human-readable block. Data always goes to stdout so
// the command composes; the caller returns any error separately so the
// exit code and stderr message follow the CLI-wide contract.
func emitAuthStatus(g *clictx.Globals, st authStatus) error {
	if g.JSON {
		return g.EmitJSON(st)
	}
	_, err := fmt.Fprintf(g.Stdout, "token:    %s (source: %s)\nverified: %t\nstatus:   %s\n",
		presentWord(st.TokenPresent), st.Source, st.Verified, st.Status)
	return err
}

func presentWord(present bool) string {
	if present {
		return "present"
	}
	return "absent"
}

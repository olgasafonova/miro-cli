package miro

import (
	"fmt"
	"os"
	"strings"
)

// EnvShareAllowedDomains is the environment variable that configures the
// allowlist of email domains permitted to receive board-share invitations.
//
// Sharing a Miro board grants access to an external party; a prompt-injected
// agent could therefore exfiltrate board content by inviting an
// attacker-controlled address. The allowlist is the operator-side guardrail:
// only emails whose domain matches the allowlist are permitted through to
// the Miro API, regardless of what the agent was told to do.
//
// This is the CLI counterpart to the same gate documented in
// rules/code-review-prompts.md HG-3 and originally shipped in
// miro-mcp-server/tools/share_allowlist.go.
const EnvShareAllowedDomains = "MIRO_SHARE_ALLOWED_DOMAINS"

// ShareAllowlist holds the set of email domains that share-board invitations
// may target. Domains are stored lowercased and compared case-insensitively.
// A zero-value allowlist rejects every email — that is the safe default.
type ShareAllowlist struct {
	domains map[string]struct{}
	source  string
}

// NewShareAllowlist builds an allowlist from an explicit list of domains.
// Entries are trimmed, lowercased, and deduplicated. Empty entries are
// skipped. The source string is surfaced in rejection errors so the user
// knows which config to adjust.
func NewShareAllowlist(domains []string, source string) *ShareAllowlist {
	set := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(d))
		if d == "" {
			continue
		}
		set[d] = struct{}{}
	}
	return &ShareAllowlist{domains: set, source: source}
}

// LoadShareAllowlistFromEnv reads MIRO_SHARE_ALLOWED_DOMAINS (comma-separated)
// and returns a populated ShareAllowlist. The returned allowlist may be empty
// if the env var is unset; an empty allowlist blocks every share attempt with
// a clear error. This is a deliberate fail-closed default: an agent cannot
// quietly invite external parties unless the operator opts in.
func LoadShareAllowlistFromEnv() *ShareAllowlist {
	raw := strings.TrimSpace(os.Getenv(EnvShareAllowedDomains))
	if raw != "" {
		return NewShareAllowlist(strings.Split(raw, ","), EnvShareAllowedDomains)
	}
	return NewShareAllowlist(nil, "unset")
}

// IsEmpty reports whether the allowlist has no domains (blocks all sharing).
func (a *ShareAllowlist) IsEmpty() bool {
	return len(a.domains) == 0
}

// Source returns a human-readable description of where the allowlist came
// from, used in rejection error messages.
func (a *ShareAllowlist) Source() string {
	return a.source
}

// Validate checks whether email's domain is permitted. Returns nil on success
// or a descriptive error that names the offending domain and the configured
// source so the operator knows what to fix.
func (a *ShareAllowlist) Validate(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	domain, ok := extractEmailDomain(email)
	if !ok {
		return fmt.Errorf("invalid email address %q: missing '@' or domain", email)
	}

	if len(a.domains) == 0 {
		return fmt.Errorf(
			"share blocked: the allowlist is empty (source: %s). "+
				"Set %s to a comma-separated list of permitted domains (e.g. \"tietoevry.com,example.com\") and try again",
			a.source, EnvShareAllowedDomains,
		)
	}

	if _, allowed := a.domains[domain]; !allowed {
		return fmt.Errorf(
			"email domain %q is not in the share allowlist (source: %s). "+
				"Add it to %s and try again, or ask the operator to do so",
			domain, a.source, EnvShareAllowedDomains,
		)
	}
	return nil
}

// extractEmailDomain returns the lowercase domain portion of an email.
// Returns ok=false when the input does not contain exactly one '@' or
// either side is empty. Intentionally strict — we validate form before
// trusting the value against the allowlist.
func extractEmailDomain(email string) (string, bool) {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.Index(email, "@")
	if at <= 0 || at != strings.LastIndex(email, "@") || at == len(email)-1 {
		return "", false
	}
	return email[at+1:], true
}

package miro

import (
	"strings"
	"testing"
)

func TestNewShareAllowlistNormalizesEntries(t *testing.T) {
	a := NewShareAllowlist([]string{"  Tietoevry.com  ", "", "EXAMPLE.com", "tietoevry.com"}, "test")
	if a.IsEmpty() {
		t.Fatal("expected non-empty allowlist after normalization")
	}
	// Duplicate (case-folded) plus empty entry should yield 2 unique domains.
	if got := len(a.domains); got != 2 {
		t.Errorf("got %d domains, want 2 (case-folded + dedup): %v", got, a.domains)
	}
	if err := a.Validate("user@tietoevry.com"); err != nil {
		t.Errorf("lowercased entry rejected an exact match: %v", err)
	}
}

func TestShareAllowlistValidate(t *testing.T) {
	tests := []struct {
		name      string
		allowlist []string
		source    string
		email     string
		wantErr   bool
		wantHints []string
	}{
		{
			name:      "empty allowlist rejects everything",
			allowlist: nil,
			source:    "unset",
			email:     "alice@example.com",
			wantErr:   true,
			wantHints: []string{"allowlist is empty", "MIRO_SHARE_ALLOWED_DOMAINS", "unset"},
		},
		{
			name:      "domain in allowlist passes",
			allowlist: []string{"tietoevry.com"},
			email:     "alice@tietoevry.com",
			wantErr:   false,
		},
		{
			name:      "domain match is case-insensitive",
			allowlist: []string{"tietoevry.com"},
			email:     "ALICE@TIETOEVRY.COM",
			wantErr:   false,
		},
		{
			name:      "subdomain is NOT allowed",
			allowlist: []string{"tietoevry.com"},
			email:     "alice@inner.tietoevry.com",
			wantErr:   true,
			wantHints: []string{"inner.tietoevry.com", "not in the share allowlist"},
		},
		{
			name:      "domain outside allowlist is rejected with source hint",
			allowlist: []string{"tietoevry.com"},
			source:    "MIRO_SHARE_ALLOWED_DOMAINS",
			email:     "bob@evil.com",
			wantErr:   true,
			wantHints: []string{"evil.com", "MIRO_SHARE_ALLOWED_DOMAINS"},
		},
		{
			name:      "empty email is rejected",
			allowlist: []string{"x.com"},
			email:     "   ",
			wantErr:   true,
			wantHints: []string{"email is required"},
		},
		{
			name:      "missing @ is rejected as invalid",
			allowlist: []string{"x.com"},
			email:     "notanemail",
			wantErr:   true,
			wantHints: []string{"invalid email"},
		},
		{
			name:      "multiple @ is rejected as invalid",
			allowlist: []string{"x.com"},
			email:     "a@b@x.com",
			wantErr:   true,
			wantHints: []string{"invalid email"},
		},
		{
			name:      "missing domain is rejected",
			allowlist: []string{"x.com"},
			email:     "alice@",
			wantErr:   true,
			wantHints: []string{"invalid email"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.source
			if source == "" {
				source = "test"
			}
			a := NewShareAllowlist(tc.allowlist, source)
			err := a.Validate(tc.email)
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%q) returned nil, want error", tc.email)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%q) returned %v, want nil", tc.email, err)
			}
			for _, hint := range tc.wantHints {
				if !strings.Contains(err.Error(), hint) {
					t.Errorf("error %q missing expected hint %q", err.Error(), hint)
				}
			}
		})
	}
}

func TestLoadShareAllowlistFromEnv(t *testing.T) {
	t.Run("env unset yields empty allowlist", func(t *testing.T) {
		t.Setenv(EnvShareAllowedDomains, "")
		a := LoadShareAllowlistFromEnv()
		if !a.IsEmpty() {
			t.Errorf("expected empty allowlist when env unset")
		}
		if a.Source() != "unset" {
			t.Errorf("source = %q, want 'unset'", a.Source())
		}
	})
	t.Run("env populated parses CSV", func(t *testing.T) {
		t.Setenv(EnvShareAllowedDomains, "tietoevry.com, example.com ,  ,DUPLICATE.com,duplicate.com")
		a := LoadShareAllowlistFromEnv()
		if a.IsEmpty() {
			t.Fatalf("expected non-empty allowlist when env populated")
		}
		// "tietoevry.com", "example.com", "duplicate.com" — 3 unique after dedup.
		if got := len(a.domains); got != 3 {
			t.Errorf("env CSV yielded %d domains, want 3: %v", got, a.domains)
		}
		if a.Source() != EnvShareAllowedDomains {
			t.Errorf("source = %q, want %q", a.Source(), EnvShareAllowedDomains)
		}
	})
}

// TestShareAllowlistRejectsEmptyAllowlistEvenForObviouslyValidEmail nails
// the fail-closed contract — the most-important property of this gate.
// Tomorrow's reviewer should be able to read this single test and conclude
// "if the env var is missing, the share verb cannot send anything." Don't
// loosen this default without explicit operator opt-in.
func TestShareAllowlistRejectsEmptyAllowlistEvenForObviouslyValidEmail(t *testing.T) {
	a := NewShareAllowlist(nil, "unset")
	if err := a.Validate("alice@tietoevry.com"); err == nil {
		t.Fatal("empty allowlist accepted a share — fail-closed default violated")
	}
}

// TestShareAllowlistExactEmailLayer verifies that the exact-email allowlist is
// authoritative when configured: it overrides (never widens) the domain layer.
func TestShareAllowlistExactEmailLayer(t *testing.T) {
	t.Run("exact match allowed, case-insensitive", func(t *testing.T) {
		t.Setenv(EnvShareAllowedDomains, "")
		t.Setenv(EnvShareAllowedEmails, "alice@example.com, BOB@example.com")
		a := LoadShareAllowlistFromEnv()
		if err := a.Validate("alice@example.com"); err != nil {
			t.Errorf("exact email rejected: %v", err)
		}
		if err := a.Validate("Bob@Example.com"); err != nil {
			t.Errorf("exact email (mixed case) rejected: %v", err)
		}
	})
	t.Run("non-listed email blocked even if domain would match", func(t *testing.T) {
		// Domain layer permits example.com, but the email layer is set and
		// authoritative — only carol@example.com is listed, so dave is blocked.
		t.Setenv(EnvShareAllowedDomains, "example.com")
		t.Setenv(EnvShareAllowedEmails, "carol@example.com")
		a := LoadShareAllowlistFromEnv()
		if err := a.Validate("dave@example.com"); err == nil {
			t.Error("email layer did not override domain layer — _NoWeakening violated")
		}
		if err := a.Validate("carol@example.com"); err != nil {
			t.Errorf("listed email rejected: %v", err)
		}
	})
	t.Run("configured-but-empty email layer fails closed", func(t *testing.T) {
		t.Setenv(EnvShareAllowedDomains, "example.com")
		t.Setenv(EnvShareAllowedEmails, "")
		a := LoadShareAllowlistFromEnv()
		if err := a.Validate("alice@example.com"); err == nil {
			t.Error("empty-but-configured email allowlist accepted a share — fail-closed violated")
		}
	})
}

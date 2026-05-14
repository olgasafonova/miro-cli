package miro

import (
	"fmt"
	"strings"
	"testing"
)

func TestRedactTokenReplacesEveryOccurrence(t *testing.T) {
	tok := "sk-secret-xyz"
	s := "Authorization: Bearer sk-secret-xyz, retry with sk-secret-xyz"
	got := RedactToken(s, tok)
	if strings.Contains(got, tok) {
		t.Fatalf("RedactToken left token in %q", got)
	}
	if strings.Count(got, redactedPlaceholder) != 2 {
		t.Errorf("expected 2 redactions, got %q", got)
	}
}

func TestRedactTokenEmptyTokenIsNoOp(t *testing.T) {
	in := "anything goes"
	if got := RedactToken(in, ""); got != in {
		t.Errorf("empty-token redaction changed input: %q", got)
	}
}

func TestConfigStringHidesToken(t *testing.T) {
	cfg := &Config{Token: "sk-secret-xyz", BaseURL: DefaultBaseURL}
	s := cfg.String()
	if strings.Contains(s, "sk-secret-xyz") {
		t.Fatalf("Config.String leaked token: %q", s)
	}
	if !strings.Contains(s, redactedPlaceholder) {
		t.Errorf("Config.String missing redaction marker: %q", s)
	}
	// fmt.Sprintf("%v", cfg) also exercises Stringer.
	if v := fmt.Sprintf("%v", cfg); strings.Contains(v, "sk-secret-xyz") {
		t.Fatalf("%%v leaked token: %q", v)
	}
}

func TestClientStringHidesToken(t *testing.T) {
	c := New(&Config{Token: "sk-secret-xyz", BaseURL: DefaultBaseURL})
	s := c.String()
	if strings.Contains(s, "sk-secret-xyz") {
		t.Fatalf("Client.String leaked token: %q", s)
	}
	if !strings.Contains(s, redactedPlaceholder) {
		t.Errorf("Client.String missing redaction marker: %q", s)
	}
}

func TestNilConfigAndClientStringerDoNotPanic(t *testing.T) {
	var cfg *Config
	if cfg.String() == "" {
		t.Error("nil Config.String returned empty")
	}
	var c *Client
	if c.String() == "" {
		t.Error("nil Client.String returned empty")
	}
}

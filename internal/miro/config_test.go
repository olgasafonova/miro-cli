package miro

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadConfigFlagWins(t *testing.T) {
	t.Setenv(EnvAccessToken, "from-env")
	cfg, err := LoadConfig("from-flag")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "from-flag" {
		t.Errorf("Token = %q, want flag value", cfg.Token)
	}
}

func TestLoadConfigEnvFallback(t *testing.T) {
	t.Setenv(EnvAccessToken, "from-env")
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "from-env" {
		t.Errorf("Token = %q, want env value", cfg.Token)
	}
}

func TestLoadConfigDefaultBaseURL(t *testing.T) {
	t.Setenv(EnvAccessToken, "t")
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultBaseURL)
	}
}

func TestLoadConfigMissingTokenReturnsConfigError(t *testing.T) {
	t.Setenv(EnvAccessToken, "")
	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected ConfigError, got nil")
	}
	var cfg *ConfigError
	if !errors.As(err, &cfg) {
		t.Fatalf("err type = %T, want *ConfigError", err)
	}
	if ExitCode(err) != ExitConfig {
		t.Errorf("ExitCode = %d, want %d", ExitCode(err), ExitConfig)
	}
}

func TestLoadConfigTrimsWhitespace(t *testing.T) {
	t.Setenv(EnvAccessToken, "  spaced  ")
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "spaced" {
		t.Errorf("Token = %q, want trimmed", cfg.Token)
	}
}

func TestLoadConfigErrorNamesEnvVar(t *testing.T) {
	// If a user runs the CLI with no token, the error should tell them
	// which env var to set. Hardcoding it in the message means future
	// renames break this test, which is the point.
	t.Setenv(EnvAccessToken, "")
	_, err := LoadConfig("")
	if !strings.Contains(err.Error(), EnvAccessToken) {
		t.Errorf("error %q should name %s", err.Error(), EnvAccessToken)
	}
}

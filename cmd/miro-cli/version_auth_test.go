package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func newTestGlobals(buf *bytes.Buffer) *clictx.Globals {
	g := clictx.New()
	g.Stdout = buf
	g.Stderr = new(bytes.Buffer)
	return g
}

func TestVersionCommandText(t *testing.T) {
	var buf bytes.Buffer
	g := newTestGlobals(&buf)
	cmd := newVersionCmd(g)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version execute: %v", err)
	}
	if !strings.Contains(buf.String(), "miro-cli") {
		t.Errorf("version text output = %q, want it to contain miro-cli", buf.String())
	}
}

func TestVersionCommandJSON(t *testing.T) {
	var buf bytes.Buffer
	g := newTestGlobals(&buf)
	g.JSON = true
	cmd := newVersionCmd(g)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version execute: %v", err)
	}
	if !strings.Contains(buf.String(), "installed_version") {
		t.Errorf("version --json output = %q, want installed_version key", buf.String())
	}
}

func TestAuthStatusNoToken(t *testing.T) {
	t.Setenv(miro.EnvAccessToken, "")
	var buf bytes.Buffer
	g := newTestGlobals(&buf)
	g.JSON = true
	err := runAuthStatus(context.Background(), g, false)
	var cfg *miro.ConfigError
	if !errors.As(err, &cfg) {
		t.Fatalf("no-token auth status err = %v, want *miro.ConfigError", err)
	}
	if !strings.Contains(buf.String(), "no_token") {
		t.Errorf("output = %q, want no_token status", buf.String())
	}
}

func TestAuthStatusTokenPresentNoVerify(t *testing.T) {
	var buf bytes.Buffer
	g := newTestGlobals(&buf)
	g.Token = "some-token"
	g.JSON = true
	if err := runAuthStatus(context.Background(), g, false); err != nil {
		t.Fatalf("token-present auth status err = %v, want nil", err)
	}
	out := buf.String()
	if !strings.Contains(out, "token_present") || !strings.Contains(out, `"source": "flag"`) {
		t.Errorf("output = %q, want token_present + source flag", out)
	}
}

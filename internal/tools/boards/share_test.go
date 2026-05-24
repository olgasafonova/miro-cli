package boards

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// allowed is a permissive helper allowlist for tests that don't care
// about the gate itself — only about the verb's HTTP behaviour.
func allowed() *miro.ShareAllowlist {
	return miro.NewShareAllowlist([]string{"tietoevry.com", "example.com"}, "test")
}

// blocked is the fail-closed default — used to assert the gate fires.
func blocked() *miro.ShareAllowlist {
	return miro.NewShareAllowlist(nil, "test")
}

func TestRunShareRejectsEmailOutsideAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("share blocked by allowlist still hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	deps := shareDeps{allowlist: allowed()}
	err := runShare(context.Background(), g, deps, "abc", shareRequest{
		Emails: []string{"attacker@evil.com"}, Role: "viewer",
	})
	if err == nil {
		t.Fatal("runShare to disallowed domain returned nil, want allowlist error")
	}
	if !strings.Contains(err.Error(), "not in the share allowlist") {
		t.Errorf("error %q did not mention the allowlist", err.Error())
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("allowlist refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
}

func TestRunShareEmptyAllowlistBlocksEverything(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("share with empty allowlist hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	deps := shareDeps{allowlist: blocked()}
	err := runShare(context.Background(), g, deps, "abc", shareRequest{
		Emails: []string{"alice@tietoevry.com"}, Role: "viewer",
	})
	if err == nil {
		t.Fatal("empty allowlist accepted share — fail-closed default violated")
	}
	if !strings.Contains(err.Error(), "allowlist is empty") {
		t.Errorf("error %q did not signal empty allowlist", err.Error())
	}
}

func TestRunShareRefusesWithoutYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("share without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	deps := shareDeps{allowlist: allowed()}
	err := runShare(context.Background(), g, deps, "abc", shareRequest{
		Emails: []string{"alice@tietoevry.com"}, Role: "viewer",
	})
	if err == nil {
		t.Fatal("share without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
}

func TestRunShareHappyPath(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   shareRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"member-1","email":"alice@tietoevry.com"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	deps := shareDeps{allowlist: allowed()}
	err := runShare(context.Background(), g, deps, "abc", shareRequest{
		Emails: []string{"alice@tietoevry.com"}, Role: "commenter", Message: "join us",
	})
	if err != nil {
		t.Fatalf("runShare: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("server saw method %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/abc/members" {
		t.Errorf("server saw path %q, want /v2/boards/abc/members", gotPath)
	}
	if len(gotBody.Emails) != 1 || gotBody.Emails[0] != "alice@tietoevry.com" {
		t.Errorf("server saw emails %v, want [alice@tietoevry.com]", gotBody.Emails)
	}
	if gotBody.Role != "commenter" {
		t.Errorf("server saw role %q, want commenter", gotBody.Role)
	}
	if gotBody.Message != "join us" {
		t.Errorf("server saw message %q, want 'join us'", gotBody.Message)
	}
	if !strings.Contains(stdout.String(), "member-1") {
		t.Errorf("stdout missing member id: %q", stdout.String())
	}
}

func TestRunShareDryRunRunsAllowlistButSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}

	// First: blocked-by-allowlist should still error in dry-run, since
	// dry-run is for previewing valid requests, not bypassing the gate.
	if err := runShare(context.Background(), g, shareDeps{allowlist: allowed()}, "abc", shareRequest{
		Emails: []string{"attacker@evil.com"}, Role: "viewer",
	}); err == nil {
		t.Error("dry-run with disallowed domain returned nil, want allowlist error")
	}

	// Then: allowed email in dry-run prints preview line, no HTTP.
	if err := runShare(context.Background(), g, shareDeps{allowlist: allowed()}, "abc", shareRequest{
		Emails: []string{"alice@tietoevry.com"}, Role: "viewer",
	}); err != nil {
		t.Fatalf("runShare dry-run with allowed email: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/abc/members") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunShareRejectsInvalidRole(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	err := runShare(context.Background(), g, shareDeps{allowlist: allowed()}, "abc", shareRequest{
		Emails: []string{"alice@tietoevry.com"}, Role: "admin",
	})
	if err == nil {
		t.Fatal("runShare with invalid role returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --role") {
		t.Errorf("error %q did not flag the role", err.Error())
	}
}

func TestRunShareRequiresEmail(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	err := runShare(context.Background(), g, shareDeps{allowlist: allowed()}, "abc", shareRequest{
		Emails: []string{""}, Role: "viewer",
	})
	if err == nil {
		t.Fatal("runShare with empty email returned nil, want error")
	}
}

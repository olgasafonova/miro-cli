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

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"uXjVK1","name":"Sprint","viewLink":"https://miro.com/app/board/uXjVK1/"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "uXjVK1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/uXjVK1" {
		t.Errorf("server saw path %q, want /v2/boards/uXjVK1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"Sprint"`) {
		t.Errorf("stdout did not contain board name: %q", stdout.String())
	}
}

func TestRunGetEmptyIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, ""); err == nil {
		t.Fatal("runGet with empty board_id returned nil, want error")
	}
}

func TestRunGetDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runGet(context.Background(), g, "abc"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/abc") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunGetNotFoundMapsToExitCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runGet(context.Background(), g, "missing")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if code := miro.ExitCode(err); code != miro.ExitNotFound {
		t.Errorf("404 mapped to exit %d, want %d (not-found)", code, miro.ExitNotFound)
	}
}

// ----- create ---------------------------------------------------------------

func TestRunCreateSendsBody(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   createRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"new-1","name":"Made by CLI"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	req := createRequest{Name: "Made by CLI", Description: "hi"}
	if err := runCreate(context.Background(), g, req); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("server saw method %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards" {
		t.Errorf("server saw path %q, want /v2/boards", gotPath)
	}
	if gotBody.Name != "Made by CLI" || gotBody.Description != "hi" {
		t.Errorf("server saw body %+v, want name+description set", gotBody)
	}
	if !strings.Contains(stdout.String(), `"new-1"`) {
		t.Errorf("stdout missing new board id: %q", stdout.String())
	}
}

func TestRunCreateOmitsEmptyFields(t *testing.T) {
	var gotRaw json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"id":"abc","name":"X"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createRequest{Name: "X"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	body := string(gotRaw)
	if strings.Contains(body, "description") || strings.Contains(body, "teamId") {
		t.Errorf("empty optional fields leaked into body: %s", body)
	}
}

func TestRunCreateRejectsEmptyName(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createRequest{}); err == nil {
		t.Fatal("runCreate with empty name returned nil, want error")
	}
}

func TestRunCreateDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCreate(context.Background(), g, createRequest{Name: "X"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- update ---------------------------------------------------------------

func TestRunUpdatePatchesAndReturnsBoard(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   updateRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"abc","name":"New"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runUpdate(context.Background(), g, "abc", updateRequest{Name: "New"}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("server saw method %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/abc" {
		t.Errorf("server saw path %q, want /v2/boards/abc", gotPath)
	}
	if gotBody.Name != "New" {
		t.Errorf("server saw body %+v, want Name=New", gotBody)
	}
	if !strings.Contains(stdout.String(), `"New"`) {
		t.Errorf("stdout missing new name: %q", stdout.String())
	}
}

func TestRunUpdateRequiresOneField(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, "abc", updateRequest{}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateEmptyIDIsUsageError(t *testing.T) {
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, "", updateRequest{Name: "X"}); err == nil {
		t.Fatal("runUpdate with empty board_id returned nil, want error")
	}
}

// ----- delete ---------------------------------------------------------------

func TestRunDeleteRefusesWithoutYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDelete(context.Background(), g, "abc")
	if err == nil {
		t.Fatal("runDelete without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
}

func TestRunDeleteWithYesCallsAPI(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	if err := runDelete(context.Background(), g, "abc"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("server saw method %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/abc" {
		t.Errorf("server saw path %q, want /v2/boards/abc", gotPath)
	}
	if !strings.Contains(stdout.String(), `"deleted": true`) {
		t.Errorf("stdout missing deleted envelope: %q", stdout.String())
	}
}

func TestRunDeleteDryRunSkipsHTTPAndYesCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	// Note: no Yes flag. Dry-run does not require it because no destructive
	// call is made.
	if err := runDelete(context.Background(), g, "abc"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/abc") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunDeleteAgentImpliesYes(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: new(bytes.Buffer),
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Agent:  true,
	}
	g.Normalize() // root command does this in PersistentPreRunE
	if err := runDelete(context.Background(), g, "abc"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllCRUDVerbs(t *testing.T) {
	cmd := NewCmd(clictx.New())
	want := map[string]bool{
		"list": false, "get": false, "create": false, "copy": false,
		"update": false, "delete": false, "share": false,
	}
	for _, sub := range cmd.Commands() {
		// sub.Use may be "list" or "get <board_id>" — use Name() which
		// strips the args portion.
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`boards` parent missing subcommand %q", verb)
		}
	}
}

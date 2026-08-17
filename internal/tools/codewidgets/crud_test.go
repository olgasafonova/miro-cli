package codewidgets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// newTestGlobals wires a Globals at the given httptest server with a
// stdout buffer for output assertions.
func newTestGlobals(srvURL string) (*clictx.Globals, *bytes.Buffer) {
	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srvURL})}
	return g, &stdout
}

func TestRunCreateSendsDataAndPosition(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "cw-1", "type": "code_widget"})
	}))
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	err := runCreate(context.Background(), g, createFlags{
		boardID: "abc", code: "fmt.Println()", language: "go", title: "Demo",
		x: 10, y: 20, width: 300,
	})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if want := "POST /v2-experimental/boards/abc/code_widgets"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	data, _ := gotBody["data"].(map[string]any)
	if data["code"] != "fmt.Println()" || data["language"] != "go" || data["title"] != "Demo" {
		t.Errorf("data block = %v", data)
	}
	if _, has := data["lineNumbersVisible"]; has {
		t.Error("lineNumbersVisible sent though --line-numbers was not passed")
	}
	position, _ := gotBody["position"].(map[string]any)
	if position["x"] != 10.0 || position["y"] != 20.0 || position["origin"] != "center" {
		t.Errorf("position = %v, want center (10,20)", position)
	}
	geometry, _ := gotBody["geometry"].(map[string]any)
	if geometry["width"] != 300.0 {
		t.Errorf("geometry = %v, want width 300", geometry)
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if out["id"] != "cw-1" {
		t.Errorf("emitted id = %v, want cw-1", out["id"])
	}
}

func TestRunCreateExplicitLineNumbersFalseReachesWire(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "cw-1"})
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runCreate(context.Background(), g, createFlags{
		boardID: "abc", code: "x", lineNumbers: false, lineNumbersSet: true,
	})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	data, _ := gotBody["data"].(map[string]any)
	if got, has := data["lineNumbersVisible"]; !has || got != false {
		t.Errorf("lineNumbersVisible = %v (present=%v), want explicit false", got, has)
	}
}

func TestRunCreateValidation(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{boardID: "abc"}); err == nil {
		t.Error("empty --code accepted")
	}
	if err := runCreate(context.Background(), g, createFlags{code: "x"}); err == nil {
		t.Error("empty --board-id accepted")
	}
	long := strings.Repeat("a", maxCodeLength+1)
	if err := runCreate(context.Background(), g, createFlags{boardID: "abc", code: long}); err == nil {
		t.Error("over-cap --code accepted")
	}
	longTitle := strings.Repeat("t", maxTitleLength+1)
	if err := runCreate(context.Background(), g, createFlags{boardID: "abc", code: "x", title: longTitle}); err == nil {
		t.Error("over-cap --title accepted")
	}
}

func TestRunGetHitsWidgetPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "cw-1", "data": map[string]any{"code": "x"}})
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	if err := runGet(context.Background(), g, "abc", "cw-1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if want := "GET /v2-experimental/boards/abc/code_widgets/cw-1"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
}

func TestRunUpdateSendsOnlyChangedFields(t *testing.T) {
	var gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "cw-1"})
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runUpdate(context.Background(), g, updateFlags{boardID: "abc", itemID: "cw-1", title: "New"})
	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", gotMethod)
	}
	data, _ := gotBody["data"].(map[string]any)
	if data["title"] != "New" {
		t.Errorf("data = %v, want title New", data)
	}
	if _, has := data["code"]; has {
		t.Error("unset --code leaked into the PATCH body")
	}
	for _, section := range []string{"geometry", "parent", "position"} {
		if _, has := gotBody[section]; has {
			t.Errorf("unset section %q leaked into the PATCH body", section)
		}
	}
}

func TestRunUpdateNoFieldsIsUsageError(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpdate(context.Background(), g, updateFlags{boardID: "abc", itemID: "cw-1"})
	if err == nil || !strings.Contains(err.Error(), "no fields to update") {
		t.Fatalf("empty update: err = %v, want no-fields error", err)
	}
}

func TestRunMovePatchesPositionEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "cw-1"})
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runMove(context.Background(), g, moveFlags{boardID: "abc", itemID: "cw-1", x: 15, y: -25})
	if err != nil {
		t.Fatalf("runMove: %v", err)
	}
	if want := "PATCH /v2-experimental/boards/abc/code_widgets/cw-1/position"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	if gotBody["x"] != 15.0 || gotBody["y"] != -25.0 || gotBody["origin"] != "center" {
		t.Errorf("body = %v, want center (15,-25)", gotBody)
	}
}

func TestRunDeleteRefusesWithoutYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runDelete(context.Background(), g, "abc", "cw-1")
	if err == nil {
		t.Fatal("delete without --yes returned nil")
	}
	var cfgErr *miro.ConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *miro.ConfigError, got %T: %v", err, err)
	}
}

func TestRunDeleteWithYes(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	g, stdout := newTestGlobals(srv.URL)
	g.Yes = true
	if err := runDelete(context.Background(), g, "abc", "cw-1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if want := "DELETE /v2-experimental/boards/abc/code_widgets/cw-1"; gotPath != want {
		t.Errorf("server saw %q, want %q", gotPath, want)
	}
	var out deleteResult
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if !out.Deleted || out.ID != "cw-1" {
		t.Errorf("envelope = %+v, want deleted cw-1", out)
	}
}

func TestVerbsDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	tests := []struct {
		name string
		run  func(g *clictx.Globals) error
		want string
	}{
		{
			name: "create",
			run: func(g *clictx.Globals) error {
				return runCreate(context.Background(), g, createFlags{boardID: "abc", code: "x"})
			},
			want: "DRY-RUN POST /v2-experimental/boards/abc/code_widgets",
		},
		{
			name: "get",
			run: func(g *clictx.Globals) error {
				return runGet(context.Background(), g, "abc", "cw-1")
			},
			want: "DRY-RUN GET /v2-experimental/boards/abc/code_widgets/cw-1",
		},
		{
			name: "update",
			run: func(g *clictx.Globals) error {
				return runUpdate(context.Background(), g, updateFlags{boardID: "abc", itemID: "cw-1", title: "t"})
			},
			want: "DRY-RUN PATCH /v2-experimental/boards/abc/code_widgets/cw-1",
		},
		{
			name: "move",
			run: func(g *clictx.Globals) error {
				return runMove(context.Background(), g, moveFlags{boardID: "abc", itemID: "cw-1", x: 1, y: 2})
			},
			want: "DRY-RUN PATCH /v2-experimental/boards/abc/code_widgets/cw-1/position",
		},
		{
			name: "delete",
			run: func(g *clictx.Globals) error {
				return runDelete(context.Background(), g, "abc", "cw-1")
			},
			want: "DRY-RUN DELETE /v2-experimental/boards/abc/code_widgets/cw-1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			g := &clictx.Globals{Stdout: &stdout, Client: client, DryRun: true}
			if err := tc.run(g); err != nil {
				t.Fatalf("dry run: %v", err)
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Errorf("dry-run output %q, want substring %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestErrorsCarryExperimentalHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":403,"message":"forbidden"}`))
	}))
	defer srv.Close()

	g, _ := newTestGlobals(srv.URL)
	err := runGet(context.Background(), g, "abc", "cw-1")
	if err == nil {
		t.Fatal("403 response returned nil error")
	}
	if !strings.Contains(err.Error(), "v2-experimental") {
		t.Errorf("403 error lacks the experimental-availability hint: %v", err)
	}
	// The wrap must not break the exit-code contract.
	if got := miro.ExitCode(err); got != miro.ExitAuth {
		t.Errorf("ExitCode(403) = %d, want %d (ExitAuth)", got, miro.ExitAuth)
	}
}

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	got := map[string]bool{}
	for _, sub := range cmd.Commands() {
		got[sub.Name()] = true
	}
	for _, want := range []string{"list", "create", "get", "update", "move", "delete"} {
		if !got[want] {
			t.Errorf("codewidgets parent did not register %q", want)
		}
	}
}

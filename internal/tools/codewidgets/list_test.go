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

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

func TestBuildListPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   ListFlags
		want string
	}{
		{
			name: "minimal",
			in:   ListFlags{BoardID: "uXjV-board-1"},
			want: "/v2-experimental/boards/uXjV-board-1/code_widgets",
		},
		{
			name: "with limit",
			in:   ListFlags{BoardID: "abc", Limit: 25},
			want: "/v2-experimental/boards/abc/code_widgets?limit=25",
		},
		{
			name: "with limit + cursor",
			in:   ListFlags{BoardID: "abc", Limit: 50, Cursor: "c-1"},
			want: "/v2-experimental/boards/abc/code_widgets?cursor=c-1&limit=50",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildListPath(tc.in); got != tc.want {
				t.Errorf("BuildListPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunListHappyPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "cw-1", "type": "code_widget"},
				{"id": "cw-2", "type": "code_widget"}
			],
			"total": 2,
			"size": 2,
			"cursor": "next-page",
			"limit": 50
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runList(context.Background(), g, ListFlags{BoardID: "abc", Limit: 50}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	wantPath := "/v2-experimental/boards/abc/code_widgets?limit=50"
	if gotPath != wantPath {
		t.Errorf("server saw path %q, want %q", gotPath, wantPath)
	}
	var out ListResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 {
		t.Errorf("emitted %d widgets, want 2", len(out.Data))
	}
	if out.Cursor != "next-page" {
		t.Errorf("cursor = %q, want next-page", out.Cursor)
	}
}

func TestRunListEmptyBoardIDIsUsageError(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runList(context.Background(), g, ListFlags{})
	if err == nil {
		t.Fatal("runList with empty --board-id returned nil, want error")
	}
	if !strings.Contains(err.Error(), "board-id") {
		t.Errorf("error %q does not mention --board-id", err)
	}
}

func TestRunListDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		DryRun: true,
	}
	if err := runList(context.Background(), g, ListFlags{BoardID: "abc", Limit: 10}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	want := "DRY-RUN GET /v2-experimental/boards/abc/code_widgets?limit=10"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("dry-run output: %q, want substring %q", stdout.String(), want)
	}
}

func TestRunListMaps404ToNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404,"message":"board not found"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: io.Discard,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	err := runList(context.Background(), g, ListFlags{BoardID: "missing"})
	if err == nil {
		t.Fatal("404 response returned nil error")
	}
	var apiErr *miro.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *miro.APIError, got %T: %v", err, err)
	}
	if got := miro.ExitCode(err); got != miro.ExitNotFound {
		t.Errorf("ExitCode(404) = %d, want %d (ExitNotFound)", got, miro.ExitNotFound)
	}
}

func TestRunListPropagatesContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"cursor":""}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: io.Discard,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the call
	err := runList(ctx, g, ListFlags{BoardID: "abc"})
	if err == nil {
		t.Fatal("runList with cancelled context returned nil")
	}
}

func TestNewCmdRegistersList(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "codewidgets" {
		t.Errorf("Use = %q, want codewidgets", cmd.Use)
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Errorf("codewidgets parent did not register list")
	}
}

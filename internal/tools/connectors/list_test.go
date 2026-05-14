package connectors

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

func TestBuildListPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   ListFlags
		want string
	}{
		{
			name: "minimal",
			in:   ListFlags{BoardID: "abc"},
			want: "/v2/boards/abc/connectors",
		},
		{
			name: "with limit",
			in:   ListFlags{BoardID: "abc", Limit: 25},
			want: "/v2/boards/abc/connectors?limit=25",
		},
		{
			name: "with limit + cursor",
			in:   ListFlags{BoardID: "abc", Limit: 25, Cursor: "c-1"},
			want: "/v2/boards/abc/connectors?cursor=c-1&limit=25",
		},
		{
			name: "cursor only",
			in:   ListFlags{BoardID: "abc", Cursor: "c-2"},
			want: "/v2/boards/abc/connectors?cursor=c-2",
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
				{"id": "1", "type": "connector", "shape": "curved"},
				{"id": "2", "type": "connector", "shape": "straight"}
			],
			"total": 2,
			"cursor": "next-page"
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runList(context.Background(), g, ListFlags{BoardID: "abc", Limit: 50}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if gotPath != "/v2/boards/abc/connectors?limit=50" {
		t.Errorf("server saw path %q", gotPath)
	}
	var out ListResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 {
		t.Errorf("emitted %d connectors, want 2", len(out.Data))
	}
	if out.Cursor != "next-page" {
		t.Errorf("cursor = %q, want next-page", out.Cursor)
	}
}

func TestRunListEmptyBoardIDIsUsageError(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runList(context.Background(), g, ListFlags{}); err == nil {
		t.Fatal("runList with empty board_id returned nil, want error")
	}
}

func TestRunListDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runList(context.Background(), g, ListFlags{BoardID: "abc", Cursor: "c-1"}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/abc/connectors?cursor=c-1") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestFetchReturnsDecodedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"x","shape":"elbowed"}],"total":1}`))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	resp, err := Fetch(context.Background(), client, ListFlags{BoardID: "abc"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0]["id"] != "x" {
		t.Errorf("Fetch returned unexpected data: %+v", resp)
	}
}

func TestNewCmdRegistersList(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Errorf("connectors parent did not register list")
	}
}

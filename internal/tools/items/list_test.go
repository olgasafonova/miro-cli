package items

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
	tests := []struct {
		name string
		in   ListFlags
		want string
	}{
		{
			name: "minimal",
			in:   ListFlags{BoardID: "abc"},
			want: "/v2/boards/abc/items",
		},
		{
			name: "with type filter",
			in:   ListFlags{BoardID: "abc", ItemType: "sticky_note"},
			want: "/v2/boards/abc/items?type=sticky_note",
		},
		{
			name: "with limit + cursor",
			in:   ListFlags{BoardID: "abc", Limit: 25, Cursor: "c-1"},
			want: "/v2/boards/abc/items?cursor=c-1&limit=25",
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
				{"id": "1", "type": "sticky_note"},
				{"id": "2", "type": "shape"}
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
	if gotPath != "/v2/boards/abc/items?limit=50" {
		t.Errorf("server saw path %q", gotPath)
	}
	var out ListResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 {
		t.Errorf("emitted %d items, want 2", len(out.Data))
	}
	if out.Cursor != "next-page" {
		t.Errorf("cursor = %q, want next-page", out.Cursor)
	}
}

func TestRunListEmptyBoardIDIsUsageError(t *testing.T) {
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
	if err := runList(context.Background(), g, ListFlags{BoardID: "abc", ItemType: "shape"}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/abc/items?type=shape") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestFetchReturnsDecodedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"x"}],"total":1}`))
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
	cmd := NewCmd(clictx.New())
	if cmd.Use != "items" {
		t.Errorf("Use = %q, want items", cmd.Use)
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list" {
			found = true
		}
	}
	if !found {
		t.Errorf("items parent did not register list")
	}
}

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

func TestFetchAllPaginatesUntilEmptyCursor(t *testing.T) {
	pageCounter := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCounter++
		switch pageCounter {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"1"},{"id":"2"}],"cursor":"page-2"}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":[{"id":"3"}],"cursor":"page-3"}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":[{"id":"4"}],"cursor":""}`)) // last page
		default:
			t.Errorf("server got extra request after end of pagination")
		}
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	all, truncated, err := FetchAll(context.Background(), client, ListFlags{BoardID: "abc"}, FetchAllOptions{})
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if truncated {
		t.Error("FetchAll under cap should not be truncated")
	}
	if len(all) != 4 {
		t.Fatalf("got %d items, want 4", len(all))
	}
	ids := []string{
		all[0]["id"].(string), all[1]["id"].(string),
		all[2]["id"].(string), all[3]["id"].(string),
	}
	want := []string{"1", "2", "3", "4"}
	for i := range ids {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestFetchAllRespectsMaxItemsCap(t *testing.T) {
	// Server returns 5 items per page with a forever-cursor.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"a"},{"id":"b"},{"id":"c"},{"id":"d"},{"id":"e"}],"cursor":"more"}`))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	all, truncated, err := FetchAll(context.Background(), client, ListFlags{BoardID: "abc"}, FetchAllOptions{MaxItems: 7})
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if !truncated {
		t.Error("FetchAll past cap should be truncated")
	}
	if len(all) != 7 {
		t.Errorf("got %d items, want 7 (cap)", len(all))
	}
}

func TestFetchAllUsesDefaultCapWhenZero(t *testing.T) {
	// Each page returns 1000 items + cursor; 5 pages would be 5000;
	// 6th would breach the default cap of 5000. We verify the default
	// kicks in by counting requests — server stops after 5.
	requests := 0
	body := `{"data":[`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			body += ","
		}
		body += `{"id":"x"}`
	}
	body += `],"cursor":"more"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	all, truncated, err := FetchAll(context.Background(), client, ListFlags{BoardID: "abc"}, FetchAllOptions{})
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if !truncated {
		t.Error("default cap should truncate at DefaultFetchAllCap")
	}
	if len(all) != DefaultFetchAllCap {
		t.Errorf("got %d items, want %d (default cap)", len(all), DefaultFetchAllCap)
	}
	// 5 pages * 1000 = 5000, exact cap. Either 5 or possibly 5 (since
	// the loop checks AFTER appending). Allow 5 or 6 if implementation
	// nuance changes; today's loop should make exactly 5.
	if requests < 5 || requests > 6 {
		t.Errorf("made %d requests, want ~5", requests)
	}
}

func TestFetchAllPropagatesContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"x"}],"cursor":"more"}`))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the call

	_, _, err := FetchAll(ctx, client, ListFlags{BoardID: "abc"}, FetchAllOptions{})
	if err == nil {
		t.Fatal("FetchAll with cancelled context returned nil error")
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

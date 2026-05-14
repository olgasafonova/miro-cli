package tables

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// ----- buildListPath --------------------------------------------------------

func TestBuildListPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   listFlags
		want string
	}{
		{
			name: "minimal",
			in:   listFlags{boardID: "b"},
			want: "/v2/boards/b/data_table_formats",
		},
		{
			name: "limit only",
			in:   listFlags{boardID: "b", limit: 25},
			want: "/v2/boards/b/data_table_formats?limit=25",
		},
		{
			name: "limit + cursor",
			in:   listFlags{boardID: "b", limit: 10, cursor: "c-2"},
			want: "/v2/boards/b/data_table_formats?cursor=c-2&limit=10",
		},
		{
			name: "cursor only",
			in:   listFlags{boardID: "b", cursor: "c-9"},
			want: "/v2/boards/b/data_table_formats?cursor=c-9",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := buildListPath(c.in); got != c.want {
				t.Errorf("path = %q, want %q", got, c.want)
			}
		})
	}
}

// ----- list ----------------------------------------------------------------

func TestRunListHappyPath(t *testing.T) {
	t.Parallel()
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"data":[{"id":"t1","type":"data_table_format"}],"total":1,"size":1}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runList(context.Background(), g, listFlags{boardID: "b", limit: 25}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if gotURI != "/v2/boards/b/data_table_formats?limit=25" {
		t.Errorf("URI = %q", gotURI)
	}
	if !strings.Contains(stdout.String(), `"t1"`) {
		t.Errorf("stdout missing table id: %q", stdout.String())
	}
}

func TestRunListRequiresBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runList(context.Background(), g, listFlags{}); err == nil {
		t.Fatal("missing --board-id should error")
	}
}

func TestRunListDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runList(context.Background(), g, listFlags{boardID: "b", limit: 5}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/data_table_formats?limit=5") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get -----------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"t1","type":"data_table_format","position":{"x":10,"y":20}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "uXjV1", "t1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/data_table_formats/t1" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/data_table_formats/t1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"t1"`) {
		t.Errorf("stdout missing table id: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "t1"); err == nil {
		t.Error("empty board ID should error")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Error("empty item ID should error")
	}
}

func TestRunGetNotFoundMapsToExitCode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runGet(context.Background(), g, "b", "missing")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if code := miro.ExitCode(err); code != miro.ExitNotFound {
		t.Errorf("404 mapped to exit %d, want %d (not-found)", code, miro.ExitNotFound)
	}
}

func TestRunGetDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runGet(context.Background(), g, "b", "t1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards/b/data_table_formats/t1") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersListAndGet(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{"list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`tables` parent missing subcommand %q", verb)
		}
	}
}

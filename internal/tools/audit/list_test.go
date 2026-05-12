package audit

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

func TestBuildListLogsPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   ListLogsFlags
		want string
	}{
		{
			name: "minimal required",
			in: ListLogsFlags{
				CreatedAfter:  "2026-04-01T00:00:00Z",
				CreatedBefore: "2026-05-12T23:59:59Z",
			},
			want: "/v2/audit/logs?createdAfter=2026-04-01T00%3A00%3A00Z&createdBefore=2026-05-12T23%3A59%3A59Z",
		},
		{
			name: "all params",
			in: ListLogsFlags{
				CreatedAfter:  "2026-04-01T00:00:00Z",
				CreatedBefore: "2026-05-12T23:59:59Z",
				Cursor:        "c-1",
				Limit:         50,
				Sorting:       "DESC",
				UserID:        "user-123",
			},
			want: "/v2/audit/logs?createdAfter=2026-04-01T00%3A00%3A00Z&createdBefore=2026-05-12T23%3A59%3A59Z&cursor=c-1&limit=50&sorting=DESC&userId=user-123",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildListLogsPath(tc.in); got != tc.want {
				t.Errorf("BuildListLogsPath = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunListLogsHappyPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{
			"type": "cursor-list",
			"limit": 100,
			"size": 2,
			"cursor": "next-page",
			"data": [
				{"id": "evt-1", "event": "sign_in_succeeded"},
				{"id": "evt-2", "event": "board_created"}
			]
		}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	lf := ListLogsFlags{
		CreatedAfter:  "2026-04-01T00:00:00Z",
		CreatedBefore: "2026-05-12T23:59:59Z",
		Limit:         100,
	}
	if err := runListLogs(context.Background(), g, lf); err != nil {
		t.Fatalf("runListLogs: %v", err)
	}
	wantPath := "/v2/audit/logs?createdAfter=2026-04-01T00%3A00%3A00Z&createdBefore=2026-05-12T23%3A59%3A59Z&limit=100"
	if gotPath != wantPath {
		t.Errorf("server saw path %q, want %q", gotPath, wantPath)
	}
	var out ListLogsResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 {
		t.Errorf("emitted %d events, want 2", len(out.Data))
	}
	if out.Cursor != "next-page" {
		t.Errorf("cursor = %q, want next-page", out.Cursor)
	}
}

func TestRunListLogsRequiresCreatedAfter(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runListLogs(context.Background(), g, ListLogsFlags{
		CreatedBefore: "2026-05-12T23:59:59Z",
	})
	if err == nil {
		t.Fatal("runListLogs without --created-after returned nil, want error")
	}
	if !strings.Contains(err.Error(), "created-after") {
		t.Errorf("error %q does not mention --created-after", err)
	}
}

func TestRunListLogsRequiresCreatedBefore(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runListLogs(context.Background(), g, ListLogsFlags{
		CreatedAfter: "2026-04-01T00:00:00Z",
	})
	if err == nil {
		t.Fatal("runListLogs without --created-before returned nil, want error")
	}
	if !strings.Contains(err.Error(), "created-before") {
		t.Errorf("error %q does not mention --created-before", err)
	}
}

func TestRunListLogsRejectsInvalidRFC3339(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		lf   ListLogsFlags
	}{
		{
			name: "bad created-after",
			lf: ListLogsFlags{
				CreatedAfter:  "yesterday",
				CreatedBefore: "2026-05-12T23:59:59Z",
			},
		},
		{
			name: "bad created-before",
			lf: ListLogsFlags{
				CreatedAfter:  "2026-04-01T00:00:00Z",
				CreatedBefore: "soon",
			},
		},
		{
			name: "wrong format (mm-dd-yyyy)",
			lf: ListLogsFlags{
				CreatedAfter:  "04-01-2026",
				CreatedBefore: "2026-05-12T23:59:59Z",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &clictx.Globals{Stdout: io.Discard}
			err := runListLogs(context.Background(), g, tc.lf)
			if err == nil {
				t.Fatal("invalid RFC3339 input returned nil error")
			}
			if !strings.Contains(err.Error(), "RFC3339") {
				t.Errorf("error %q does not mention RFC3339", err)
			}
		})
	}
}

func TestRunListLogsDryRunSkipsHTTP(t *testing.T) {
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
	lf := ListLogsFlags{
		CreatedAfter:  "2026-04-01T00:00:00Z",
		CreatedBefore: "2026-05-12T23:59:59Z",
		Sorting:       "DESC",
	}
	if err := runListLogs(context.Background(), g, lf); err != nil {
		t.Fatalf("runListLogs: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/audit/logs?") {
		t.Errorf("dry-run output missing expected prefix: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "sorting=DESC") {
		t.Errorf("dry-run path missing sorting filter: %q", stdout.String())
	}
}

func TestRunListLogsMaps403ToAuth(t *testing.T) {
	// Enterprise endpoint: a non-Enterprise token typically gets 403.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":403,"message":"forbidden"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: io.Discard,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	lf := ListLogsFlags{
		CreatedAfter:  "2026-04-01T00:00:00Z",
		CreatedBefore: "2026-05-12T23:59:59Z",
	}
	err := runListLogs(context.Background(), g, lf)
	if err == nil {
		t.Fatal("403 response returned nil error")
	}
	var apiErr *miro.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *miro.APIError, got %T: %v", err, err)
	}
	if got := miro.ExitCode(err); got != miro.ExitAuth {
		t.Errorf("ExitCode(403) = %d, want %d (ExitAuth)", got, miro.ExitAuth)
	}
}

func TestRunListLogsMaps404ToNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404,"message":"not found"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: io.Discard,
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
	}
	lf := ListLogsFlags{
		CreatedAfter:  "2026-04-01T00:00:00Z",
		CreatedBefore: "2026-05-12T23:59:59Z",
	}
	err := runListLogs(context.Background(), g, lf)
	if err == nil {
		t.Fatal("404 response returned nil error")
	}
	if got := miro.ExitCode(err); got != miro.ExitNotFound {
		t.Errorf("ExitCode(404) = %d, want %d (ExitNotFound)", got, miro.ExitNotFound)
	}
}

func TestNewCmdRegistersListLogs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Use != "audit" {
		t.Errorf("Use = %q, want audit", cmd.Use)
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "list-logs" {
			found = true
		}
	}
	if !found {
		t.Errorf("audit parent did not register list-logs")
	}
}

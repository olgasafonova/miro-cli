package boards

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func TestBuildListPathNoFilters(t *testing.T) {
	got := buildListPath(listFlags{})
	if got != "/v2/boards" {
		t.Errorf("buildListPath empty = %q, want /v2/boards", got)
	}
}

func TestBuildListPathEncodesQueryParams(t *testing.T) {
	tests := []struct {
		name string
		in   listFlags
		want string
	}{
		{
			name: "team filter",
			in:   listFlags{teamID: "t-1"},
			want: "/v2/boards?team_id=t-1",
		},
		{
			name: "limit and offset",
			in:   listFlags{limit: 25, offset: 50},
			want: "/v2/boards?limit=25&offset=50",
		},
		{
			name: "query escapes spaces",
			in:   listFlags{query: "design sprint"},
			want: "/v2/boards?query=design+sprint",
		},
		{
			name: "all filters together",
			in: listFlags{
				teamID:    "t-1",
				projectID: "p-2",
				query:     "alpha",
				owner:     "u-3",
				sort:      "last_modified",
				limit:     10,
				offset:    20,
			},
			// url.Values.Encode sorts keys alphabetically.
			want: "/v2/boards?limit=10&offset=20&owner=u-3&project_id=p-2&query=alpha&sort=last_modified&team_id=t-1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildListPath(tc.in)
			if got != tc.want {
				t.Errorf("buildListPath = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRunListHappyPath wires the runList helper against an httptest
// server and asserts on the assembled path, auth header, and the JSON
// the command emits to stdout. This is the canonical pattern Phase 3
// tools follow.
func TestRunListHappyPath(t *testing.T) {
	const sampleResp = `{
		"data": [
			{"id": "uXjVK1", "name": "Alpha", "viewLink": "https://miro.com/app/board/uXjVK1/"},
			{"id": "uXjVK2", "name": "Beta",  "viewLink": "https://miro.com/app/board/uXjVK2/"}
		],
		"total": 2,
		"size": 2,
		"offset": 0,
		"limit": 50
	}`

	var (
		gotMethod string
		gotPath   string
		gotAuth   string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleResp))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})
	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: client}

	err := runList(context.Background(), g, listFlags{limit: 50})
	if err != nil {
		t.Fatalf("runList: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("server saw method %q, want GET", gotMethod)
	}
	if gotPath != "/v2/boards?limit=50" {
		t.Errorf("server saw path %q, want /v2/boards?limit=50", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("server saw auth %q, want Bearer test-token", gotAuth)
	}

	var out ListResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if len(out.Data) != 2 {
		t.Fatalf("emitted %d boards, want 2", len(out.Data))
	}
	if out.Data[0]["id"] != "uXjVK1" {
		t.Errorf("first board id = %v, want uXjVK1", out.Data[0]["id"])
	}
	if out.Total != 2 {
		t.Errorf("total = %d, want 2", out.Total)
	}
}

func TestRunListDryRunSkipsHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run must not call the API; got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: client, DryRun: true}

	if err := runList(context.Background(), g, listFlags{teamID: "t-1"}); err != nil {
		t.Fatalf("runList dry-run: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN GET /v2/boards?team_id=t-1") {
		t.Errorf("dry-run stdout = %q, want it to contain the GET line", stdout.String())
	}
}

func TestRunListPropagatesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "bad", BaseURL: srv.URL})
	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: client}

	err := runList(context.Background(), g, listFlags{})
	if err == nil {
		t.Fatal("runList returned nil on 401, want APIError")
	}
	if code := miro.ExitCode(err); code != miro.ExitAuth {
		t.Errorf("401 mapped to exit %d, want %d (auth)", code, miro.ExitAuth)
	}
}

func TestRunListWithSelectFiltersOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"abc","name":"X","viewLink":"https://miro.com/app/board/abc/"}]}`))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: client, Select: "data"}

	if err := runList(context.Background(), g, listFlags{}); err != nil {
		t.Fatalf("runList: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "\"data\"") {
		t.Errorf("expected data key in output: %q", out)
	}
	if strings.Contains(out, "\"total\"") || strings.Contains(out, "\"size\"") || strings.Contains(out, "\"limit\"") {
		t.Errorf("--select data dropped other top-level fields, but output still has them: %q", out)
	}
}

func TestNewCmdRegistersList(t *testing.T) {
	g := clictx.New()
	cmd := NewCmd(g)
	if cmd.Use != "boards" {
		t.Errorf("NewCmd().Use = %q, want boards", cmd.Use)
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("`boards` parent does not register `list` subcommand")
	}
}

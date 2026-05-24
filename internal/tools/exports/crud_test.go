package exports

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

// ----- validateFormat -------------------------------------------------------

func TestValidateFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"SVG", false},
		{"HTML", false},
		{"PDF", false},
		{"", true},      // required
		{"svg", true},   // case-sensitive — API accepts uppercase only
		{"PNG", true},   // not in enum
		{"JPEG", true},  // not in enum
		{"CSV", true},   // not in enum (task spec drift)
		{" PDF ", true}, // whitespace rejected
	}
	for _, c := range cases {
		err := validateFormat(c.in)
		if c.wantErr && err == nil {
			t.Errorf("validateFormat(%q) = nil, want error", c.in)
		}
		if !c.wantErr && err != nil {
			t.Errorf("validateFormat(%q) = %v, want nil", c.in, err)
		}
	}
}

// ----- create-job -----------------------------------------------------------

func TestRunCreateJobHappyPath(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotQuery  string
		gotBody   createRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"jobId":"job-1"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID:     "org-1",
		requestID: "req-uuid",
		boardIDs:  []string{"b1", "b2"},
		format:    "PDF",
	})
	if err != nil {
		t.Fatalf("runCreateJob: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/orgs/org-1/boards/export/jobs" {
		t.Errorf("path = %q, want /v2/orgs/org-1/boards/export/jobs", gotPath)
	}
	if !strings.Contains(gotQuery, "request_id=req-uuid") {
		t.Errorf("query = %q, want request_id=req-uuid", gotQuery)
	}
	if len(gotBody.BoardIDs) != 2 || gotBody.BoardIDs[0] != "b1" || gotBody.BoardIDs[1] != "b2" {
		t.Errorf("boardIds = %+v, want [b1 b2]", gotBody.BoardIDs)
	}
	if gotBody.BoardFormat != "PDF" {
		t.Errorf("boardFormat = %q, want PDF", gotBody.BoardFormat)
	}
	if !strings.Contains(stdout.String(), `"jobId"`) {
		t.Errorf("stdout missing job id: %q", stdout.String())
	}
}

func TestRunCreateJobRejectsEmptyOrgID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreateJob(context.Background(), g, createJobFlags{
		requestID: "u", boardIDs: []string{"b"}, format: "PDF",
	})
	if err == nil {
		t.Fatal("runCreateJob with empty --org-id returned nil, want error")
	}
}

func TestRunCreateJobRejectsEmptyRequestID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID: "o", boardIDs: []string{"b"}, format: "PDF",
	})
	if err == nil {
		t.Fatal("runCreateJob with empty --request-id returned nil, want error")
	}
}

func TestRunCreateJobRejectsEmptyFormat(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID: "o", requestID: "u", boardIDs: []string{"b"},
	})
	if err == nil {
		t.Fatal("runCreateJob with empty --format returned nil, want error")
	}
}

func TestRunCreateJobRejectsZeroBoardIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID: "o", requestID: "u", format: "PDF",
	})
	if err == nil {
		t.Fatal("runCreateJob with no --board-id returned nil, want error")
	}
	if !strings.Contains(err.Error(), "board-id") {
		t.Errorf("error = %q, want mention of board-id", err.Error())
	}
}

func TestRunCreateJobRejectsInvalidFormat(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID: "o", requestID: "u", boardIDs: []string{"b"}, format: "PNG",
	})
	if err == nil {
		t.Fatal("runCreateJob with --format=PNG returned nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("error = %q, want invalid --format prefix", err.Error())
	}
}

func TestRunCreateJobDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID: "o", requestID: "u", boardIDs: []string{"b"}, format: "SVG",
	})
	if err != nil {
		t.Fatalf("runCreateJob: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/orgs/o/boards/export/jobs") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunCreateJobEscapesRequestID(t *testing.T) {
	t.Parallel()
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: new(bytes.Buffer), Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	// Use a value with '&' and '=' to verify query-string escaping. ValidateID
	// rejects whitespace, so spaces can't be the escape-triggering character.
	err := runCreateJob(context.Background(), g, createJobFlags{
		orgID: "o", requestID: "req&id=value", boardIDs: []string{"b"}, format: "PDF",
	})
	if err != nil {
		t.Fatalf("runCreateJob: %v", err)
	}
	// url.QueryEscape encodes '&' as %26 and '=' as %3D in query strings.
	if !strings.Contains(gotQuery, "request_id=req%26id%3Dvalue") {
		t.Errorf("query = %q, want encoded request_id", gotQuery)
	}
}

// ----- get-job-status -------------------------------------------------------

func TestRunGetJobStatusHappyPath(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"status":"IN_PROGRESS"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGetJobStatus(context.Background(), g, "org-1", "job-1"); err != nil {
		t.Fatalf("runGetJobStatus: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/orgs/org-1/boards/export/jobs/job-1" {
		t.Errorf("path = %q, want /v2/orgs/org-1/boards/export/jobs/job-1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"IN_PROGRESS"`) {
		t.Errorf("stdout missing status: %q", stdout.String())
	}
}

func TestRunGetJobStatusRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetJobStatus(context.Background(), g, "", "j"); err == nil {
		t.Fatal("runGetJobStatus with empty org returned nil, want error")
	}
	if err := runGetJobStatus(context.Background(), g, "o", ""); err == nil {
		t.Fatal("runGetJobStatus with empty job returned nil, want error")
	}
}

func TestRunGetJobStatusNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runGetJobStatus(context.Background(), g, "o", "missing")
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if code := miro.ExitCode(err); code != miro.ExitNotFound {
		t.Errorf("404 mapped to exit %d, want %d", code, miro.ExitNotFound)
	}
}

// ----- get-job-results ------------------------------------------------------

func TestBuildResultsPathDefaults(t *testing.T) {
	t.Parallel()
	p := buildResultsPath(getJobResultsFlags{orgID: "o", jobID: "j"})
	if p != "/v2/orgs/o/boards/export/jobs/j/results" {
		t.Errorf("path = %q, want bare path with no query", p)
	}
}

func TestBuildResultsPathWithLimitOffset(t *testing.T) {
	t.Parallel()
	p := buildResultsPath(getJobResultsFlags{orgID: "o", jobID: "j", limit: 25, offset: 50})
	if !strings.HasPrefix(p, "/v2/orgs/o/boards/export/jobs/j/results?") {
		t.Errorf("path = %q, want prefix /v2/orgs/o/.../results?", p)
	}
	if !strings.Contains(p, "limit=25") {
		t.Errorf("path = %q, want limit=25", p)
	}
	if !strings.Contains(p, "offset=50") {
		t.Errorf("path = %q, want offset=50", p)
	}
}

func TestRunGetJobResultsHappyPath(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"boardId":"b1"}]}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runGetJobResults(context.Background(), g, getJobResultsFlags{orgID: "o", jobID: "j"})
	if err != nil {
		t.Fatalf("runGetJobResults: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/orgs/o/boards/export/jobs/j/results" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(stdout.String(), `"b1"`) {
		t.Errorf("stdout missing board id: %q", stdout.String())
	}
}

func TestRunGetJobResultsRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetJobResults(context.Background(), g, getJobResultsFlags{jobID: "j"}); err == nil {
		t.Fatal("runGetJobResults with empty org returned nil, want error")
	}
	if err := runGetJobResults(context.Background(), g, getJobResultsFlags{orgID: "o"}); err == nil {
		t.Fatal("runGetJobResults with empty job returned nil, want error")
	}
}

// ----- list-job-tasks -------------------------------------------------------

func TestBuildTasksPathDefaults(t *testing.T) {
	t.Parallel()
	p := buildTasksPath(listJobTasksFlags{orgID: "o", jobID: "j"})
	if p != "/v2/orgs/o/boards/export/jobs/j/tasks" {
		t.Errorf("path = %q, want bare path with no query", p)
	}
}

func TestBuildTasksPathWithLimitOffset(t *testing.T) {
	t.Parallel()
	p := buildTasksPath(listJobTasksFlags{orgID: "o", jobID: "j", limit: 10, offset: 100})
	if !strings.Contains(p, "limit=10") {
		t.Errorf("path = %q, want limit=10", p)
	}
	if !strings.Contains(p, "offset=100") {
		t.Errorf("path = %q, want offset=100", p)
	}
}

func TestRunListJobTasksHappyPath(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"taskId":"t1"}]}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runListJobTasks(context.Background(), g, listJobTasksFlags{orgID: "o", jobID: "j"})
	if err != nil {
		t.Fatalf("runListJobTasks: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v2/orgs/o/boards/export/jobs/j/tasks" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(stdout.String(), `"t1"`) {
		t.Errorf("stdout missing task id: %q", stdout.String())
	}
}

func TestRunListJobTasksRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runListJobTasks(context.Background(), g, listJobTasksFlags{jobID: "j"}); err == nil {
		t.Fatal("runListJobTasks with empty org returned nil, want error")
	}
	if err := runListJobTasks(context.Background(), g, listJobTasksFlags{orgID: "o"}); err == nil {
		t.Fatal("runListJobTasks with empty job returned nil, want error")
	}
}

// ----- get-task-link --------------------------------------------------------

func TestRunGetTaskLinkHappyPath(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"url":"https://s3.example.com/file.pdf"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGetTaskLink(context.Background(), g, "o", "j", "t"); err != nil {
		t.Fatalf("runGetTaskLink: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST (per spec)", gotMethod)
	}
	if gotPath != "/v2/orgs/o/boards/export/jobs/j/tasks/t/export-link" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(stdout.String(), "s3.example.com") {
		t.Errorf("stdout missing url: %q", stdout.String())
	}
}

func TestRunGetTaskLinkRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGetTaskLink(context.Background(), g, "", "j", "t"); err == nil {
		t.Fatal("runGetTaskLink with empty org returned nil, want error")
	}
	if err := runGetTaskLink(context.Background(), g, "o", "", "t"); err == nil {
		t.Fatal("runGetTaskLink with empty job returned nil, want error")
	}
	if err := runGetTaskLink(context.Background(), g, "o", "j", ""); err == nil {
		t.Fatal("runGetTaskLink with empty task returned nil, want error")
	}
}

func TestRunGetTaskLinkRejectsMissingTaskID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runGetTaskLink(context.Background(), g, "o", "j", "")
	if err == nil {
		t.Fatal("runGetTaskLink with empty --task-id returned nil, want error")
	}
	if !strings.Contains(err.Error(), "task_id") {
		t.Errorf("error = %q, want mention of task_id", err.Error())
	}
}

func TestRunGetTaskLinkDryRun(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runGetTaskLink(context.Background(), g, "o", "j", "t"); err != nil {
		t.Fatalf("runGetTaskLink: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/orgs/o/boards/export/jobs/j/tasks/t/export-link") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- update-job (cancel) --------------------------------------------------

func TestRunUpdateJobRequiresCancel(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpdateJob(context.Background(), g, updateJobFlags{orgID: "o", jobID: "j"})
	if err == nil {
		t.Fatal("runUpdateJob with no operation returned nil, want error")
	}
	if !strings.Contains(err.Error(), "--cancel") {
		t.Errorf("error = %q, want mention of --cancel", err.Error())
	}
}

func TestRunUpdateJobRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdateJob(context.Background(), g, updateJobFlags{jobID: "j", cancel: true}); err == nil {
		t.Fatal("runUpdateJob with empty org returned nil, want error")
	}
	if err := runUpdateJob(context.Background(), g, updateJobFlags{orgID: "o", cancel: true}); err == nil {
		t.Fatal("runUpdateJob with empty job returned nil, want error")
	}
}

func TestRunUpdateJobRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("update-job without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runUpdateJob(context.Background(), g, updateJobFlags{orgID: "o", jobID: "j", cancel: true})
	if err == nil {
		t.Fatal("runUpdateJob --cancel without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d", code, miro.ExitConfig)
	}
}

func TestRunUpdateJobWithYesCallsAPI(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   updateRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"status":"CANCELLED"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	err := runUpdateJob(context.Background(), g, updateJobFlags{orgID: "o", jobID: "j", cancel: true})
	if err != nil {
		t.Fatalf("runUpdateJob: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT (per spec)", gotMethod)
	}
	if gotPath != "/v2/orgs/o/boards/export/jobs/j/status" {
		t.Errorf("path = %q, want .../jobs/j/status", gotPath)
	}
	if gotBody.Status != "CANCELLED" {
		t.Errorf("body status = %q, want CANCELLED", gotBody.Status)
	}
	if !strings.Contains(stdout.String(), `"CANCELLED"`) {
		t.Errorf("stdout missing CANCELLED: %q", stdout.String())
	}
}

func TestRunUpdateJobDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	err := runUpdateJob(context.Background(), g, updateJobFlags{orgID: "o", jobID: "j", cancel: true})
	if err != nil {
		t.Fatalf("runUpdateJob: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN PUT /v2/orgs/o/boards/export/jobs/j/status") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunUpdateJobAgentImpliesYes(t *testing.T) {
	t.Parallel()
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_, _ = w.Write([]byte(`{"status":"CANCELLED"}`))
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: new(bytes.Buffer),
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Agent:  true,
	}
	g.Normalize()
	err := runUpdateJob(context.Background(), g, updateJobFlags{orgID: "o", jobID: "j", cancel: true})
	if err != nil {
		t.Fatalf("runUpdateJob: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("--agent did not allow PUT; server saw method %q", gotMethod)
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{
		"create-job":      false,
		"get-job-status":  false,
		"get-job-results": false,
		"list-job-tasks":  false,
		"get-task-link":   false,
		"update-job":      false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`exports` parent missing subcommand %q", verb)
		}
	}
}

func TestNewCmdParentHasShortDescription(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	if cmd.Short == "" {
		t.Error("exports parent missing Short description")
	}
	if cmd.Use != "exports" {
		t.Errorf("parent Use = %q, want exports", cmd.Use)
	}
}

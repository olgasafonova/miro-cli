package boards

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// stubServer returns a tiny Miro-shaped fake: each call to a POST
// /v2/boards/{id}/(shapes|connectors|frames|groups) endpoint returns
// a sequential JSON id, so the orchestration can wire connectors to
// the right shape ids.
func stubServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s on %s", r.Method, r.URL.Path)
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		id := counter.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "item-" + intToStr(id)})
	}))
	t.Cleanup(srv.Close)
	return srv, &counter
}

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

const simpleFlowchart = `flowchart TD
A[Start] --> B[Middle]
B --> C[End]
`

func TestDiagramHappyPath(t *testing.T) {
	srv, counter := stubServer(t)
	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})

	var stdout, stderr bytes.Buffer
	g := &clictx.Globals{
		Stdout: &stdout,
		Stderr: &stderr,
		Client: client,
	}

	f := diagramFlags{diagram: simpleFlowchart, outputMode: "discrete"}
	if err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", f); err != nil {
		t.Fatalf("runDiagram: %v", err)
	}

	// 3 shapes + 2 connectors = 5 POSTs.
	if got := counter.Load(); got != 5 {
		t.Errorf("server saw %d POSTs, want 5", got)
	}

	var res diagramResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if res.NodesCreated != 3 {
		t.Errorf("NodesCreated = %d, want 3", res.NodesCreated)
	}
	if res.ConnectorsCreated != 2 {
		t.Errorf("ConnectorsCreated = %d, want 2", res.ConnectorsCreated)
	}
	if res.OutputMode != "discrete" {
		t.Errorf("OutputMode = %q, want discrete", res.OutputMode)
	}
	if !strings.Contains(res.Message, "3 nodes") || !strings.Contains(res.Message, "2 connectors") {
		t.Errorf("Message = %q, want both counts", res.Message)
	}
}

func TestDiagramGroupedMode(t *testing.T) {
	srv, counter := stubServer(t)
	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})

	var stdout, stderr bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Stderr: &stderr, Client: client}
	f := diagramFlags{diagram: simpleFlowchart, outputMode: "grouped"}

	if err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", f); err != nil {
		t.Fatalf("runDiagram: %v", err)
	}

	// 3 shapes + 2 connectors + 1 group = 6 POSTs.
	if got := counter.Load(); got != 6 {
		t.Errorf("server saw %d POSTs, want 6", got)
	}
	var res diagramResult
	_ = json.Unmarshal(stdout.Bytes(), &res)
	if res.DiagramType != "group" {
		t.Errorf("DiagramType = %q, want group", res.DiagramType)
	}
	if res.DiagramID == "" {
		t.Error("DiagramID empty in grouped mode")
	}
}

func TestDiagramFramedMode(t *testing.T) {
	srv, counter := stubServer(t)
	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})

	var stdout, stderr bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Stderr: &stderr, Client: client}
	f := diagramFlags{diagram: simpleFlowchart, outputMode: "framed"}

	if err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", f); err != nil {
		t.Fatalf("runDiagram: %v", err)
	}

	// 3 shapes + 2 connectors + 1 frame = 6 POSTs.
	if got := counter.Load(); got != 6 {
		t.Errorf("server saw %d POSTs, want 6", got)
	}
	var res diagramResult
	_ = json.Unmarshal(stdout.Bytes(), &res)
	if res.DiagramType != "frame" {
		t.Errorf("DiagramType = %q, want frame", res.DiagramType)
	}
	if res.FramesCreated < 1 {
		t.Errorf("FramesCreated = %d, want >= 1", res.FramesCreated)
	}
}

func TestDiagramRejectsEmptyBoardID(t *testing.T) {
	g := &clictx.Globals{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)}
	err := runDiagram(t.Context(), g, strings.NewReader(""), "", diagramFlags{diagram: simpleFlowchart})
	if err == nil {
		t.Fatal("empty board_id should error")
	}
	if !strings.Contains(err.Error(), "board_id") {
		t.Errorf("error = %q, want board_id mention", err.Error())
	}
}

func TestDiagramRejectsMissingSource(t *testing.T) {
	g := &clictx.Globals{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)}
	err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", diagramFlags{})
	if err == nil {
		t.Fatal("missing --diagram/--diagram-file/--diagram-stdin should error")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q, want 'required' mention", err.Error())
	}
}

func TestDiagramRejectsMutuallyExclusiveSources(t *testing.T) {
	g := &clictx.Globals{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)}
	f := diagramFlags{diagram: simpleFlowchart, diagramStdin: true}
	err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", f)
	if err == nil {
		t.Fatal("both --diagram and --diagram-stdin should error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want 'mutually exclusive'", err.Error())
	}
}

func TestDiagramRejectsInvalidOutputMode(t *testing.T) {
	srv, _ := stubServer(t)
	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})
	g := &clictx.Globals{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer), Client: client}
	f := diagramFlags{diagram: simpleFlowchart, outputMode: "weird"}
	err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", f)
	if err == nil {
		t.Fatal("invalid --output-mode should error")
	}
	if !strings.Contains(err.Error(), "output-mode") {
		t.Errorf("error = %q, want 'output-mode'", err.Error())
	}
}

func TestDiagramDryRunSkipsHTTP(t *testing.T) {
	srv, counter := stubServer(t)
	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})

	var stdout, stderr bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Stderr: &stderr, Client: client, DryRun: true}
	f := diagramFlags{diagram: simpleFlowchart, outputMode: "discrete"}

	if err := runDiagram(t.Context(), g, strings.NewReader(""), "board-abc", f); err != nil {
		t.Fatalf("dry-run runDiagram: %v", err)
	}
	if got := counter.Load(); got != 0 {
		t.Errorf("server saw %d POSTs during --dry-run, want 0", got)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST") {
		t.Errorf("stdout missing DRY-RUN line: %q", stdout.String())
	}
}

func TestDiagramStdinSource(t *testing.T) {
	srv, _ := stubServer(t)
	client := miro.New(&miro.Config{Token: "test-token", BaseURL: srv.URL})

	var stdout, stderr bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Stderr: &stderr, Client: client}
	f := diagramFlags{diagramStdin: true, outputMode: "discrete"}

	stdin := strings.NewReader(simpleFlowchart)
	if err := runDiagram(t.Context(), g, stdin, "board-abc", f); err != nil {
		t.Fatalf("stdin runDiagram: %v", err)
	}
	var res diagramResult
	_ = json.Unmarshal(stdout.Bytes(), &res)
	if res.NodesCreated != 3 {
		t.Errorf("NodesCreated = %d, want 3", res.NodesCreated)
	}
}

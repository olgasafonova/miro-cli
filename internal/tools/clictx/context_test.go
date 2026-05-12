package clictx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"miro-cli/internal/miro"
)

func TestNormalizeAgentExpands(t *testing.T) {
	g := &Globals{Agent: true}
	g.Normalize()
	if !g.JSON {
		t.Errorf("--agent should imply --json")
	}
	if !g.Yes {
		t.Errorf("--agent should imply --yes")
	}
}

func TestNormalizeWithoutAgentIsNoop(t *testing.T) {
	g := &Globals{}
	g.Normalize()
	if g.JSON || g.Yes {
		t.Errorf("Normalize on zero Globals should not touch flags")
	}
}

func TestNormalizeAgentDoesNotClobberExplicitFlags(t *testing.T) {
	g := &Globals{Agent: true, JSON: true, Yes: true}
	g.Normalize()
	if !g.JSON || !g.Yes {
		t.Errorf("Normalize must not clear flags that were already set")
	}
}

func TestBuildClientUsesInjectedClient(t *testing.T) {
	want := miro.New(&miro.Config{Token: "t", BaseURL: "http://x"})
	g := &Globals{Client: want}
	got, err := g.BuildClient()
	if err != nil {
		t.Fatalf("BuildClient: %v", err)
	}
	if got != want {
		t.Errorf("BuildClient returned %p, want injected %p", got, want)
	}
}

func TestBuildClientReadsToken(t *testing.T) {
	g := &Globals{Token: "from-flag"}
	c, err := g.BuildClient()
	if err != nil {
		t.Fatalf("BuildClient: %v", err)
	}
	if c == nil {
		t.Fatal("BuildClient returned nil with valid token")
	}
}

func TestBuildClientNoTokenIsConfigError(t *testing.T) {
	t.Setenv(miro.EnvAccessToken, "")
	g := &Globals{}
	_, err := g.BuildClient()
	if err == nil {
		t.Fatal("BuildClient should fail without a token")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("missing-token error mapped to exit %d, want %d", code, miro.ExitConfig)
	}
}

func TestEmitJSONIndents(t *testing.T) {
	var buf bytes.Buffer
	g := &Globals{Stdout: &buf}
	if err := g.EmitJSON(map[string]string{"id": "abc", "name": "Board"}); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "  \"id\": \"abc\"") {
		t.Errorf("expected indented JSON, got: %q", out)
	}
}

func TestEmitJSONSelectFiltersTopLevel(t *testing.T) {
	var buf bytes.Buffer
	g := &Globals{Stdout: &buf, Select: "id,name"}
	if err := g.EmitJSON(map[string]any{"id": "abc", "name": "B", "secret": "k"}); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode emitted JSON: %v", err)
	}
	if _, ok := out["secret"]; ok {
		t.Errorf("--select did not drop unrelated field: %v", out)
	}
	if out["id"] != "abc" || out["name"] != "B" {
		t.Errorf("--select dropped requested fields: %v", out)
	}
}

func TestEmitJSONSelectFiltersArrayElements(t *testing.T) {
	var buf bytes.Buffer
	g := &Globals{Stdout: &buf, Select: "id"}
	in := []map[string]any{
		{"id": "1", "name": "A"},
		{"id": "2", "name": "B"},
	}
	if err := g.EmitJSON(in); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	var out []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(out))
	}
	for i, elem := range out {
		if _, ok := elem["name"]; ok {
			t.Errorf("elem %d: --select did not drop name: %v", i, elem)
		}
		if elem["id"] == nil {
			t.Errorf("elem %d: --select dropped requested id: %v", i, elem)
		}
	}
}

func TestEmitDryRunWritesMethodAndPath(t *testing.T) {
	var buf bytes.Buffer
	g := &Globals{Stdout: &buf}
	if err := g.EmitDryRun("GET", "/v2/boards"); err != nil {
		t.Fatalf("EmitDryRun: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "DRY-RUN GET /v2/boards") {
		t.Errorf("EmitDryRun output = %q, want it to contain method+path", got)
	}
}

// TestEmitJSON_EndToEndWithClient confirms that Globals plays well with
// a real *miro.Client served by httptest. This is the pattern that
// per-tool tests follow: build a Globals with an injected Client whose
// BaseURL points at the test server.
func TestEmitJSON_EndToEndWithClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/boards" {
			t.Errorf("server got path %q, want /v2/boards", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"abc"}],"total":1}`))
	}))
	defer srv.Close()

	client := miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})
	g := &Globals{Stdout: new(bytes.Buffer), Client: client}

	var resp map[string]any
	if err := client.Get(context.Background(), "/v2/boards", &resp); err != nil {
		t.Fatalf("client.Get: %v", err)
	}
	if err := g.EmitJSON(resp); err != nil {
		t.Fatalf("EmitJSON: %v", err)
	}
	if !strings.Contains(g.Stdout.(*bytes.Buffer).String(), `"abc"`) {
		t.Errorf("expected emitted JSON to contain board id, got: %q", g.Stdout.(*bytes.Buffer).String())
	}
}

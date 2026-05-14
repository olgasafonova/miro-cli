package shapes

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

func TestBuildCreateFlowchartRequestMinimal(t *testing.T) {
	t.Parallel()
	req := buildCreateFlowchartRequest(createFlowchartFlags{shape: "rhombus"})
	if req.Data.Shape != "rhombus" {
		t.Errorf("shape = %q, want rhombus", req.Data.Shape)
	}
	if req.Style != nil {
		t.Errorf("style should be nil with no color flags: %+v", req.Style)
	}
	if req.Geometry != nil {
		t.Errorf("geometry should be nil with no width/height: %+v", req.Geometry)
	}
	if req.Position == nil || req.Position.Origin != "center" {
		t.Errorf("position should default to center origin: %+v", req.Position)
	}
}

func TestBuildCreateFlowchartRequestSetsFillAndBorder(t *testing.T) {
	t.Parallel()
	req := buildCreateFlowchartRequest(createFlowchartFlags{
		shape:       "pentagon",
		content:     "Decide",
		fillColor:   "#006400",
		borderColor: "#000000",
		width:       240,
		height:      120,
		parentID:    "frame-1",
	})
	if req.Style == nil {
		t.Fatal("style should be set when fill/border colors are passed")
	}
	if req.Style.FillColor != "#006400" {
		t.Errorf("style.fillColor = %q", req.Style.FillColor)
	}
	if req.Style.BorderColor != "#000000" {
		t.Errorf("style.borderColor = %q", req.Style.BorderColor)
	}
	// Flowchart endpoint does not accept text styling; the builder must
	// leave those slots empty even if the underlying struct supports them.
	if req.Style.Color != "" || req.Style.TextAlign != "" || req.Style.TextAlignVertical != "" {
		t.Errorf("text-styling fields should be unset on flowchart create: %+v", req.Style)
	}
	if req.Geometry == nil || req.Geometry.Width != 240 || req.Geometry.Height != 120 {
		t.Errorf("geometry = %+v", req.Geometry)
	}
	if req.Parent == nil || req.Parent.ID != "frame-1" {
		t.Errorf("parent = %+v", req.Parent)
	}
}

func TestBuildCreateFlowchartRequestOmitsStyleWhenColorsUnset(t *testing.T) {
	t.Parallel()
	req := buildCreateFlowchartRequest(createFlowchartFlags{
		shape:    "rectangle",
		content:  "Step",
		width:    180,
		parentID: "frame-x",
	})
	if req.Style != nil {
		t.Errorf("style should remain nil without fill/border colors: %+v", req.Style)
	}
}

func TestRunCreateFlowchartHitsExperimentalEndpoint(t *testing.T) {
	t.Parallel()
	var (
		gotMethod string
		gotPath   string
		gotBody   createRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"sh-1","data":{"shape":"rhombus"}}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runCreateFlowchart(context.Background(), g, createFlowchartFlags{
		boardID:   "uXjV1",
		shape:     "rhombus",
		content:   "Approve?",
		fillColor: "#ffe066",
	})
	if err != nil {
		t.Fatalf("runCreateFlowchart: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2-experimental/boards/uXjV1/shapes" {
		t.Errorf("path = %q, want /v2-experimental/boards/uXjV1/shapes", gotPath)
	}
	if gotBody.Data.Shape != "rhombus" || gotBody.Data.Content != "Approve?" {
		t.Errorf("body data = %+v", gotBody.Data)
	}
	if gotBody.Style == nil || gotBody.Style.FillColor != "#ffe066" {
		t.Errorf("body style = %+v", gotBody.Style)
	}
	if !strings.Contains(stdout.String(), `"sh-1"`) {
		t.Errorf("stdout missing new shape id: %q", stdout.String())
	}
}

func TestRunCreateFlowchartRejectsEmptyBoardID(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateFlowchart(context.Background(), g, createFlowchartFlags{shape: "rhombus"}); err == nil {
		t.Fatal("empty board ID should error")
	}
}

func TestRunCreateFlowchartRejectsEmptyShape(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreateFlowchart(context.Background(), g, createFlowchartFlags{boardID: "b"}); err == nil {
		t.Fatal("empty shape should error")
	}
}

func TestRunCreateFlowchartDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCreateFlowchart(context.Background(), g, createFlowchartFlags{boardID: "b", shape: "rhombus"}); err != nil {
		t.Fatalf("runCreateFlowchart: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2-experimental/boards/b/shapes") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestNewCmdRegistersCreateFlowchart(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	for _, sub := range cmd.Commands() {
		if sub.Name() == "create-flowchart" {
			return
		}
	}
	t.Error("`shapes` parent missing create-flowchart subcommand")
}

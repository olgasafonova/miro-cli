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

	"github.com/olgasafonova/miro-cli/internal/miro"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// ----- enum validators -----------------------------------------------------

func TestValidateShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"curved", false},
		{"straight", false},
		{"elbowed", false},
		{"CURVED", true},
		{"zigzag", true},
		{" curved", true},
	}
	for _, c := range cases {
		err := validateShape(c.in)
		if c.wantErr && err == nil {
			t.Errorf("validateShape(%q) = nil, want error", c.in)
		}
		if !c.wantErr && err != nil {
			t.Errorf("validateShape(%q) = %v, want nil", c.in, err)
		}
	}
}

func TestValidateSnapTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"auto", false},
		{"top", false},
		{"right", false},
		{"bottom", false},
		{"left", false},
		{"middle", true},
		{"TOP", true},
	}
	for _, c := range cases {
		err := validateSnapTo(c.in, "start-snap-to")
		if c.wantErr && err == nil {
			t.Errorf("validateSnapTo(%q) = nil, want error", c.in)
		}
		if !c.wantErr && err != nil {
			t.Errorf("validateSnapTo(%q) = %v, want nil", c.in, err)
		}
	}
}

func TestValidateStrokeStyle(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"", "normal", "dotted", "dashed"} {
		if err := validateStrokeStyle(ok); err != nil {
			t.Errorf("validateStrokeStyle(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"solid", "double", "DOTTED"} {
		if err := validateStrokeStyle(bad); err == nil {
			t.Errorf("validateStrokeStyle(%q) = nil, want error", bad)
		}
	}
}

func TestValidateStrokeCap(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"", "none", "stealth", "arrow", "filled_diamond", "erd_zero_or_many", "unknown"} {
		if err := validateStrokeCap(ok, "end-stroke-cap"); err != nil {
			t.Errorf("validateStrokeCap(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"ARROW", "hook", "circle"} {
		if err := validateStrokeCap(bad, "end-stroke-cap"); err == nil {
			t.Errorf("validateStrokeCap(%q) = nil, want error", bad)
		}
	}
}

func TestValidateTextOrientation(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"", "horizontal", "aligned"} {
		if err := validateTextOrientation(ok); err != nil {
			t.Errorf("validateTextOrientation(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"vertical", "HORIZONTAL"} {
		if err := validateTextOrientation(bad); err == nil {
			t.Errorf("validateTextOrientation(%q) = nil, want error", bad)
		}
	}
}

// ----- parsePosition --------------------------------------------------------

func TestParsePositionEmpty(t *testing.T) {
	t.Parallel()
	off, err := parsePosition("")
	if err != nil {
		t.Fatalf("parsePosition(\"\"): %v", err)
	}
	if off != nil {
		t.Errorf("parsePosition(\"\") = %+v, want nil", off)
	}
}

func TestParsePositionHappy(t *testing.T) {
	t.Parallel()
	off, err := parsePosition("50%,25%")
	if err != nil {
		t.Fatalf("parsePosition: %v", err)
	}
	if off.X != "50%" || off.Y != "25%" {
		t.Errorf("parsed = %+v, want x=50%% y=25%%", off)
	}
}

func TestParsePositionWhitespace(t *testing.T) {
	t.Parallel()
	off, err := parsePosition("  50% , 0%  ")
	if err != nil {
		t.Fatalf("parsePosition: %v", err)
	}
	if off.X != "50%" || off.Y != "0%" {
		t.Errorf("parsed = %+v, want trimmed x=50%% y=0%%", off)
	}
}

func TestParsePositionPartial(t *testing.T) {
	t.Parallel()
	off, err := parsePosition("50%,")
	if err != nil {
		t.Fatalf("parsePosition X-only: %v", err)
	}
	if off.X != "50%" || off.Y != "" {
		t.Errorf("X-only = %+v, want x=50%% y=\"\"", off)
	}
}

func TestParsePositionRejectsNoComma(t *testing.T) {
	t.Parallel()
	if _, err := parsePosition("50%"); err == nil {
		t.Error("parsePosition(\"50%\") = nil, want error (missing comma)")
	}
}

func TestParsePositionRejectsBothEmpty(t *testing.T) {
	t.Parallel()
	if _, err := parsePosition(","); err == nil {
		t.Error("parsePosition(\",\") = nil, want error (both empty)")
	}
}

// ----- parseCaption ---------------------------------------------------------

func TestParseCaptionContentOnly(t *testing.T) {
	t.Parallel()
	c, err := parseCaption("Approve")
	if err != nil {
		t.Fatalf("parseCaption: %v", err)
	}
	if c.Content != "Approve" || c.Position != "" {
		t.Errorf("got %+v, want content=Approve position=\"\"", c)
	}
}

func TestParseCaptionWithPosition(t *testing.T) {
	t.Parallel()
	c, err := parseCaption("Approve@25%")
	if err != nil {
		t.Fatalf("parseCaption: %v", err)
	}
	if c.Content != "Approve" || c.Position != "25%" {
		t.Errorf("got %+v, want content=Approve position=25%%", c)
	}
}

func TestParseCaptionPositionWithoutPercent(t *testing.T) {
	t.Parallel()
	// "50" without % is still treated as a percentage shorthand.
	c, err := parseCaption("Reject@50")
	if err != nil {
		t.Fatalf("parseCaption: %v", err)
	}
	if c.Content != "Reject" || c.Position != "50" {
		t.Errorf("got %+v, want content=Reject position=50", c)
	}
}

func TestParseCaptionAtSignInContent(t *testing.T) {
	t.Parallel()
	// "@user" is not a numeric suffix, so the whole string is content.
	c, err := parseCaption("ping @alice")
	if err != nil {
		t.Fatalf("parseCaption: %v", err)
	}
	if c.Content != "ping @alice" || c.Position != "" {
		t.Errorf("got %+v, want full string as content", c)
	}
}

func TestParseCaptionEmpty(t *testing.T) {
	t.Parallel()
	if _, err := parseCaption(""); err == nil {
		t.Error("parseCaption(\"\") = nil, want error")
	}
}

func TestParseCaptionEmptyContentBeforeAt(t *testing.T) {
	t.Parallel()
	if _, err := parseCaption("@50%"); err == nil {
		t.Error("parseCaption(\"@50%\") = nil, want error (empty content)")
	}
}

// ----- create: buildCreateRequest ------------------------------------------

func TestBuildCreateRequestMinimal(t *testing.T) {
	t.Parallel()
	req, err := buildCreateRequest(createFlags{
		startItemID: "i1",
		endItemID:   "i2",
	})
	if err != nil {
		t.Fatalf("buildCreateRequest: %v", err)
	}
	if req.StartItem == nil || req.StartItem.ID != "i1" {
		t.Errorf("startItem = %+v, want id=i1", req.StartItem)
	}
	if req.EndItem == nil || req.EndItem.ID != "i2" {
		t.Errorf("endItem = %+v, want id=i2", req.EndItem)
	}
	if req.Shape != "" {
		t.Errorf("shape = %q, want empty (default)", req.Shape)
	}
	if req.Style != nil {
		t.Errorf("style = %+v, want nil when no style flags set", req.Style)
	}
	if req.Captions != nil {
		t.Errorf("captions = %+v, want nil when no --caption flags", req.Captions)
	}
}

func TestBuildCreateRequestFullPayload(t *testing.T) {
	t.Parallel()
	req, err := buildCreateRequest(createFlags{
		startItemID:     "i1",
		endItemID:       "i2",
		startSnapTo:     "right",
		endPos:          "50%,0%",
		shape:           "elbowed",
		strokeColor:     "#2d9bf0",
		strokeWidth:     "2.0",
		strokeStyle:     "dashed",
		startStrokeCap:  "none",
		endStrokeCap:    "arrow",
		fontSize:        "14",
		captionColor:    "#1a1a1a",
		textOrientation: "aligned",
		captions:        []string{"Step 1@25%", "Step 2"},
	})
	if err != nil {
		t.Fatalf("buildCreateRequest: %v", err)
	}
	if req.StartItem.SnapTo != "right" {
		t.Errorf("startItem.snapTo = %q, want right", req.StartItem.SnapTo)
	}
	if req.EndItem.Position == nil || req.EndItem.Position.X != "50%" || req.EndItem.Position.Y != "0%" {
		t.Errorf("endItem.position = %+v, want x=50%% y=0%%", req.EndItem.Position)
	}
	if req.Shape != "elbowed" {
		t.Errorf("shape = %q", req.Shape)
	}
	if req.Style == nil {
		t.Fatal("style should be non-nil with style flags set")
	}
	if req.Style.StrokeColor != "#2d9bf0" || req.Style.StrokeWidth != "2.0" || req.Style.StrokeStyle != "dashed" {
		t.Errorf("style stroke = %+v", req.Style)
	}
	if req.Style.EndStrokeCap != "arrow" || req.Style.FontSize != "14" || req.Style.Color != "#1a1a1a" {
		t.Errorf("style caption-related = %+v", req.Style)
	}
	if req.Style.TextOrientation != "aligned" {
		t.Errorf("style.textOrientation = %q", req.Style.TextOrientation)
	}
	if len(req.Captions) != 2 {
		t.Fatalf("captions len = %d, want 2", len(req.Captions))
	}
	if req.Captions[0].Content != "Step 1" || req.Captions[0].Position != "25%" {
		t.Errorf("captions[0] = %+v", req.Captions[0])
	}
	if req.Captions[1].Content != "Step 2" || req.Captions[1].Position != "" {
		t.Errorf("captions[1] = %+v", req.Captions[1])
	}
}

func TestBuildCreateRequestSerializesWithoutStyle(t *testing.T) {
	t.Parallel()
	req, err := buildCreateRequest(createFlags{startItemID: "a", endItemID: "b"})
	if err != nil {
		t.Fatalf("buildCreateRequest: %v", err)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "style") {
		t.Errorf("expected no style key when unset, got %s", raw)
	}
	if strings.Contains(string(raw), "captions") {
		t.Errorf("expected no captions key when unset, got %s", raw)
	}
}

// ----- create: runCreate ----------------------------------------------------

func TestRunCreateSendsBody(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"c1","shape":"curved"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runCreate(context.Background(), g, createFlags{
		boardID:     "uXjV1",
		startItemID: "i1",
		endItemID:   "i2",
		shape:       "curved",
		startSnapTo: "auto",
		endSnapTo:   "left",
	}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v2/boards/uXjV1/connectors" {
		t.Errorf("path = %q, want /v2/boards/uXjV1/connectors", gotPath)
	}
	if gotBody.StartItem == nil || gotBody.StartItem.ID != "i1" || gotBody.StartItem.SnapTo != "auto" {
		t.Errorf("body startItem = %+v", gotBody.StartItem)
	}
	if gotBody.EndItem == nil || gotBody.EndItem.ID != "i2" || gotBody.EndItem.SnapTo != "left" {
		t.Errorf("body endItem = %+v", gotBody.EndItem)
	}
	if gotBody.Shape != "curved" {
		t.Errorf("body shape = %q, want curved", gotBody.Shape)
	}
	if !strings.Contains(stdout.String(), `"c1"`) {
		t.Errorf("stdout missing new connector id: %q", stdout.String())
	}
}

func TestRunCreateRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runCreate(context.Background(), g, createFlags{startItemID: "a", endItemID: "b"}); err == nil {
		t.Error("runCreate with empty board returned nil")
	}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", endItemID: "x"}); err == nil {
		t.Error("runCreate with empty start returned nil")
	}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", startItemID: "x"}); err == nil {
		t.Error("runCreate with empty end returned nil")
	}
}

func TestRunCreateRejectsSameStartAndEnd(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreate(context.Background(), g, createFlags{boardID: "b", startItemID: "same", endItemID: "same"})
	if err == nil {
		t.Fatal("runCreate with start==end returned nil, want error")
	}
	if !strings.Contains(err.Error(), "must differ") {
		t.Errorf("error = %q, want \"must differ\" hint", err.Error())
	}
}

func TestRunCreateRejectsInvalidShape(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runCreate(context.Background(), g, createFlags{
		boardID: "b", startItemID: "i1", endItemID: "i2", shape: "zigzag",
	})
	if err == nil {
		t.Fatal("runCreate with shape=zigzag returned nil")
	}
	if !strings.Contains(err.Error(), "invalid --shape") {
		t.Errorf("error = %q, want invalid --shape prefix", err.Error())
	}
}

func TestRunCreateDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runCreate(context.Background(), g, createFlags{boardID: "b", startItemID: "i1", endItemID: "i2"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN POST /v2/boards/b/connectors") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

// ----- get ------------------------------------------------------------------

func TestRunGetHappyPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"c1","shape":"curved"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	if err := runGet(context.Background(), g, "b1", "c1"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if gotPath != "/v2/boards/b1/connectors/c1" {
		t.Errorf("path = %q, want /v2/boards/b1/connectors/c1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"curved"`) {
		t.Errorf("stdout missing shape: %q", stdout.String())
	}
}

func TestRunGetRejectsEmptyArgs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runGet(context.Background(), g, "", "c"); err == nil {
		t.Error("runGet with empty board ID returned nil")
	}
	if err := runGet(context.Background(), g, "b", ""); err == nil {
		t.Error("runGet with empty connector ID returned nil")
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

// ----- update: buildUpdateRequest ------------------------------------------

func TestBuildUpdateRequestNoFieldsReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok, err := buildUpdateRequest(updateFlags{})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if ok {
		t.Error("buildUpdateRequest with no fields should report ok=false")
	}
}

func TestBuildUpdateRequestOnlyShapeSet(t *testing.T) {
	t.Parallel()
	req, ok, err := buildUpdateRequest(updateFlags{shape: "straight", shapeSet: true})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if !ok {
		t.Fatal("ok should be true with shape set")
	}
	if req.Shape != "straight" {
		t.Errorf("shape = %q, want straight", req.Shape)
	}
	if req.StartItem != nil || req.EndItem != nil || req.Style != nil {
		t.Errorf("unset sections should be nil: %+v", req)
	}
}

func TestBuildUpdateRequestStyleSliceFields(t *testing.T) {
	t.Parallel()
	req, ok, err := buildUpdateRequest(updateFlags{
		strokeColor:    "#ff0000",
		strokeColorSet: true,
		fontSize:       "18",
		fontSizeSet:    true,
	})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if !ok {
		t.Fatal("ok should be true with style fields set")
	}
	if req.Style == nil || req.Style.StrokeColor != "#ff0000" || req.Style.FontSize != "18" {
		t.Errorf("style = %+v", req.Style)
	}
	// Unset style fields should stay empty (not zero-out the server side).
	if req.Style.StrokeWidth != "" || req.Style.EndStrokeCap != "" {
		t.Errorf("unset style fields leaked: %+v", req.Style)
	}
}

func TestBuildUpdateRequestEndpointUpdate(t *testing.T) {
	t.Parallel()
	req, ok, err := buildUpdateRequest(updateFlags{
		startItemID:    "new-start",
		startItemIDSet: true,
		endSnapTo:      "bottom",
		endSnapToSet:   true,
	})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if !ok {
		t.Fatal("ok should be true with endpoint updates")
	}
	if req.StartItem == nil || req.StartItem.ID != "new-start" {
		t.Errorf("startItem = %+v", req.StartItem)
	}
	if req.StartItem.SnapTo != "" {
		t.Errorf("startItem.snapTo leaked = %q", req.StartItem.SnapTo)
	}
	if req.EndItem == nil || req.EndItem.SnapTo != "bottom" {
		t.Errorf("endItem = %+v", req.EndItem)
	}
	if req.EndItem.ID != "" {
		t.Errorf("endItem.id leaked = %q", req.EndItem.ID)
	}
}

func TestBuildUpdateRequestCaptionsReplaceAll(t *testing.T) {
	t.Parallel()
	req, ok, err := buildUpdateRequest(updateFlags{
		captions:    []string{"New caption@50%"},
		captionsSet: true,
	})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if !ok {
		t.Fatal("ok should be true with captions set")
	}
	if len(req.Captions) != 1 || req.Captions[0].Content != "New caption" || req.Captions[0].Position != "50%" {
		t.Errorf("captions = %+v", req.Captions)
	}
}

func TestBuildUpdateRequestClearCaptionsEmitsEmptyArray(t *testing.T) {
	t.Parallel()
	req, ok, err := buildUpdateRequest(updateFlags{
		clearCaptions: true,
		captionsSet:   true,
	})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if !ok {
		t.Fatal("ok should be true with clear-captions set")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"captions":[]`) {
		t.Errorf("expected explicit empty captions array in %s", raw)
	}
}

func TestBuildUpdateRequestOmitsCaptionsWhenUnset(t *testing.T) {
	t.Parallel()
	req, _, err := buildUpdateRequest(updateFlags{shape: "straight", shapeSet: true})
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "captions") {
		t.Errorf("captions should be omitted when unset, got %s", raw)
	}
}

// ----- update: runUpdate ---------------------------------------------------

func TestRunUpdatePatchesAndReturnsConnector(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"c1","shape":"straight"}`))
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runUpdate(context.Background(), g, updateFlags{
		boardID:     "b",
		connectorID: "c1",
		shape:       "straight",
		shapeSet:    true,
	})
	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if gotPath != "/v2/boards/b/connectors/c1" {
		t.Errorf("path = %q, want /v2/boards/b/connectors/c1", gotPath)
	}
	if gotBody.Shape != "straight" {
		t.Errorf("body shape = %q, want straight", gotBody.Shape)
	}
	if !strings.Contains(stdout.String(), `"straight"`) {
		t.Errorf("stdout missing updated shape: %q", stdout.String())
	}
}

func TestRunUpdateRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", connectorID: "c"}); err == nil {
		t.Fatal("runUpdate with no fields returned nil, want error")
	}
}

func TestRunUpdateRequiresIDs(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	if err := runUpdate(context.Background(), g, updateFlags{connectorID: "c", shape: "curved", shapeSet: true}); err == nil {
		t.Error("runUpdate with empty board ID returned nil")
	}
	if err := runUpdate(context.Background(), g, updateFlags{boardID: "b", shape: "curved", shapeSet: true}); err == nil {
		t.Error("runUpdate with empty connector ID returned nil")
	}
}

func TestRunUpdateRejectsInvalidShape(t *testing.T) {
	t.Parallel()
	g := &clictx.Globals{Stdout: io.Discard}
	err := runUpdate(context.Background(), g, updateFlags{boardID: "b", connectorID: "c", shape: "zigzag", shapeSet: true})
	if err == nil {
		t.Fatal("runUpdate with bad shape returned nil")
	}
	if !strings.Contains(err.Error(), "invalid --shape") {
		t.Errorf("error = %q", err.Error())
	}
}

// ----- delete ---------------------------------------------------------------

func TestRunDeleteRefusesWithoutYes(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("delete without --yes hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	g := &clictx.Globals{Stdout: io.Discard, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL})}
	err := runDelete(context.Background(), g, "b", "c")
	if err == nil {
		t.Fatal("runDelete without --yes returned nil, want refusal")
	}
	if code := miro.ExitCode(err); code != miro.ExitConfig {
		t.Errorf("refusal mapped to exit %d, want %d (config)", code, miro.ExitConfig)
	}
}

func TestRunDeleteWithYesCallsAPI(t *testing.T) {
	t.Parallel()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), Yes: true}
	if err := runDelete(context.Background(), g, "b", "c1"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/v2/boards/b/connectors/c1" {
		t.Errorf("path = %q, want /v2/boards/b/connectors/c1", gotPath)
	}
	if !strings.Contains(stdout.String(), `"deleted": true`) {
		t.Errorf("stdout missing deleted envelope: %q", stdout.String())
	}
}

func TestRunDeleteDryRunSkipsHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("--dry-run hit the API: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	g := &clictx.Globals{Stdout: &stdout, Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}), DryRun: true}
	if err := runDelete(context.Background(), g, "b", "c"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(stdout.String(), "DRY-RUN DELETE /v2/boards/b/connectors/c") {
		t.Errorf("dry-run output: %q", stdout.String())
	}
}

func TestRunDeleteAgentImpliesYes(t *testing.T) {
	t.Parallel()
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	g := &clictx.Globals{
		Stdout: new(bytes.Buffer),
		Client: miro.New(&miro.Config{Token: "t", BaseURL: srv.URL}),
		Agent:  true,
	}
	g.Normalize()
	if err := runDelete(context.Background(), g, "b", "c"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("--agent did not allow DELETE; server saw method %q", gotMethod)
	}
}

// ----- registration ---------------------------------------------------------

func TestNewCmdRegistersAllCRUDVerbs(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(clictx.New())
	want := map[string]bool{"create": false, "get": false, "update": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := want[sub.Name()]; ok {
			want[sub.Name()] = true
		}
	}
	for verb, found := range want {
		if !found {
			t.Errorf("`connectors` parent missing subcommand %q", verb)
		}
	}
}

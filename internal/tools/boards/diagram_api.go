package boards

import (
	"context"
	"fmt"

	"github.com/olgasafonova/miro-cli/internal/diagrams"
	"github.com/olgasafonova/miro-cli/internal/miro"
)

// Request/response payloads for the Miro v2 endpoints the diagram
// command speaks to. Kept package-private so they don't collide with
// the shapes/connectors/frames/groups packages' own copies.

type diagShapeRequest struct {
	Data     diagShapeData   `json:"data"`
	Style    *diagShapeStyle `json:"style,omitempty"`
	Position *diagPosition   `json:"position,omitempty"`
	Geometry *diagGeometry   `json:"geometry,omitempty"`
	Parent   *diagParentRef  `json:"parent,omitempty"`
}

type diagShapeData struct {
	Content string `json:"content,omitempty"`
	Shape   string `json:"shape,omitempty"`
}

type diagShapeStyle struct {
	FillColor   string `json:"fillColor,omitempty"`
	BorderColor string `json:"borderColor,omitempty"`
}

type diagPosition struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin,omitempty"`
}

type diagGeometry struct {
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type diagParentRef struct {
	ID string `json:"id"`
}

type diagFrameRequest struct {
	Data     diagFrameData   `json:"data"`
	Style    *diagFrameStyle `json:"style,omitempty"`
	Position *diagPosition   `json:"position,omitempty"`
	Geometry *diagGeometry   `json:"geometry,omitempty"`
}

type diagFrameData struct {
	Title  string `json:"title,omitempty"`
	Type   string `json:"type,omitempty"`
	Format string `json:"format,omitempty"`
}

type diagFrameStyle struct {
	FillColor string `json:"fillColor,omitempty"`
}

type diagConnectorRequest struct {
	StartItem diagConnectorEndpoint `json:"startItem"`
	EndItem   diagConnectorEndpoint `json:"endItem"`
	Shape     string                `json:"shape,omitempty"`
	Captions  []diagCaption         `json:"captions,omitempty"`
	Style     *diagConnectorStyle   `json:"style,omitempty"`
}

type diagConnectorEndpoint struct {
	ID string `json:"id"`
}

type diagCaption struct {
	Content string `json:"content"`
}

type diagConnectorStyle struct {
	StartStrokeCap string `json:"startStrokeCap,omitempty"`
	EndStrokeCap   string `json:"endStrokeCap,omitempty"`
}

type diagGroupRequest struct {
	Data diagGroupData `json:"data"`
}

type diagGroupData struct {
	Items []string `json:"items"`
	Type  string   `json:"type,omitempty"`
}

type diagItemResponse struct {
	ID string `json:"id"`
}

// diagramWriter owns the board-scoped API calls the diagram command
// issues. Bundling the client, target board, and invocation flags keeps
// the per-step helpers' signatures small and cohesive.
type diagramWriter struct {
	client  *miro.Client
	boardID string
	flags   diagramFlags
}

// createFrames creates each frame and returns the IDs that succeeded.
// Failures are accumulated as warnings rather than aborting — partial
// progress on a diagram is more useful than nothing.
func (w diagramWriter) createFrames(ctx context.Context, frames []diagrams.MiroFrame) ([]string, []string) {
	ids := make([]string, 0, len(frames))
	warnings := make([]string, 0)
	for _, frame := range frames {
		req := diagFrameRequest{
			Data: diagFrameData{
				Title:  frame.Title,
				Type:   "freeform",
				Format: "custom",
			},
			Position: &diagPosition{X: frame.X, Y: frame.Y, Origin: "center"},
			Geometry: &diagGeometry{Width: frame.Width, Height: frame.Height},
		}
		if frame.Color != "" {
			req.Style = &diagFrameStyle{FillColor: frame.Color}
		}
		var resp diagItemResponse
		if err := w.client.Post(ctx, "/v2/boards/"+w.boardID+"/frames", req, &resp); err != nil {
			warnings = append(warnings, fmt.Sprintf("create frame %q: %v", frame.Title, err))
			continue
		}
		ids = append(ids, resp.ID)
	}
	return ids, warnings
}

// createShapes creates each shape (standard or experimental stencil
// endpoint per IsStencil) and returns the IDs plus the index→ID map
// needed to resolve connector endpoints.
func (w diagramWriter) createShapes(ctx context.Context, shapes []diagrams.MiroShape) ([]string, map[int]string, []string) {
	nodeIDs := make([]string, 0, len(shapes))
	idMap := make(map[int]string, len(shapes))
	warnings := make([]string, 0)
	for i, shape := range shapes {
		id, err := w.createOneShape(ctx, shape)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("create shape %q: %v", shape.Content, err))
			continue
		}
		idMap[i] = id
		nodeIDs = append(nodeIDs, id)
	}
	return nodeIDs, idMap, warnings
}

func (w diagramWriter) createOneShape(ctx context.Context, shape diagrams.MiroShape) (string, error) {
	req := diagShapeRequest{
		Data:     diagShapeData{Content: shape.Content, Shape: shape.Shape},
		Position: &diagPosition{X: shape.X, Y: shape.Y, Origin: "center"},
	}
	if shape.Width > 0 || shape.Height > 0 {
		req.Geometry = &diagGeometry{Width: shape.Width, Height: shape.Height}
	}
	if shape.Color != "" || shape.BorderColor != "" {
		req.Style = &diagShapeStyle{FillColor: shape.Color, BorderColor: shape.BorderColor}
	}
	if w.flags.parentID != "" {
		req.Parent = &diagParentRef{ID: w.flags.parentID}
	}

	path := "/v2/boards/" + w.boardID + "/shapes"
	if shape.IsStencil {
		path = "/v2-experimental/boards/" + w.boardID + "/shapes"
	}

	var resp diagItemResponse
	if err := w.client.Post(ctx, path, req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// createConnectors creates connectors using the index map. A connector
// whose endpoint shape failed to create is silently skipped: the
// underlying node never made it onto the board.
func (w diagramWriter) createConnectors(ctx context.Context, connectors []diagrams.MiroConnector, idMap map[int]string) ([]string, []string) {
	ids := make([]string, 0, len(connectors))
	warnings := make([]string, 0)
	for _, c := range connectors {
		startID, ok1 := idMap[c.StartItemIndex]
		endID, ok2 := idMap[c.EndItemIndex]
		if !ok1 || !ok2 {
			continue
		}
		req := diagConnectorRequest{
			StartItem: diagConnectorEndpoint{ID: startID},
			EndItem:   diagConnectorEndpoint{ID: endID},
			Shape:     c.Style,
		}
		if c.Caption != "" {
			req.Captions = []diagCaption{{Content: c.Caption}}
		}
		if c.StartCap != "" || c.EndCap != "" {
			req.Style = &diagConnectorStyle{StartStrokeCap: c.StartCap, EndStrokeCap: c.EndCap}
		}
		var resp diagItemResponse
		if err := w.client.Post(ctx, "/v2/boards/"+w.boardID+"/connectors", req, &resp); err != nil {
			warnings = append(warnings, fmt.Sprintf("create connector: %v", err))
			continue
		}
		ids = append(ids, resp.ID)
	}
	return ids, warnings
}

// finalizeGrouped bundles every created item into one Miro group.
// Skips when fewer than two items exist (groups of one are noisy and
// the API rejects empty groups). Reads the already-computed
// result.TotalItems for its messages.
func (w diagramWriter) finalizeGrouped(ctx context.Context, allItemIDs []string, result *diagramResult) {
	if len(allItemIDs) < 2 {
		result.Message = fmt.Sprintf("Created diagram with %d items (too few to group)", result.TotalItems)
		return
	}
	req := diagGroupRequest{Data: diagGroupData{Items: allItemIDs, Type: "normal"}}
	var resp diagItemResponse
	if err := w.client.Post(ctx, "/v2/boards/"+w.boardID+"/groups", req, &resp); err != nil {
		result.Message = fmt.Sprintf("Created diagram with %d items (grouping failed: %v)", result.TotalItems, err)
		return
	}
	result.DiagramID = resp.ID
	result.DiagramType = "group"
	result.Message = fmt.Sprintf("Created grouped diagram with %d items", result.TotalItems)
}

// finalizeFramed wraps every created item in a single containing frame
// sized to the diagram bounds plus a margin, positioned from the
// writer's start-x/start-y flags.
func (w diagramWriter) finalizeFramed(ctx context.Context, diagram *diagrams.Diagram, result *diagramResult) {
	const padding = 40.0
	frameWidth := diagram.Width + padding*2
	frameHeight := diagram.Height + padding*2
	centerX := w.flags.startX + diagram.Width/2
	centerY := w.flags.startY + diagram.Height/2

	req := diagFrameRequest{
		Data:     diagFrameData{Title: "Diagram", Type: "freeform", Format: "custom"},
		Position: &diagPosition{X: centerX, Y: centerY, Origin: "center"},
		Geometry: &diagGeometry{Width: frameWidth, Height: frameHeight},
	}
	var resp diagItemResponse
	if err := w.client.Post(ctx, "/v2/boards/"+w.boardID+"/frames", req, &resp); err != nil {
		result.Message = fmt.Sprintf("Created diagram with %d items (framing failed: %v)", result.TotalItems, err)
		return
	}
	result.DiagramID = resp.ID
	result.DiagramType = "frame"
	result.FrameIDs = append(result.FrameIDs, resp.ID)
	result.FramesCreated++
	result.Message = fmt.Sprintf("Created framed diagram with %d items", result.TotalItems)
}

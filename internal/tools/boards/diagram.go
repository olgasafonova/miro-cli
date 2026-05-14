package boards

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"miro-cli/internal/diagrams"
	"miro-cli/internal/miro"
	"miro-cli/internal/tools/clictx"
)

// diagramFlags captures every knob on `miro boards diagram`. The
// --diagram-file alternative lets callers feed a stored Mermaid file
// without quoting headaches; --diagram-stdin reads the source from
// standard input for piped use.
type diagramFlags struct {
	diagram      string
	diagramFile  string
	diagramStdin bool
	outputMode   string
	useStencils  bool
	startX       float64
	startY       float64
	nodeWidth    float64
	parentID     string
}

func newDiagramCmd(g *clictx.Globals) *cobra.Command {
	var f diagramFlags
	cmd := &cobra.Command{
		Use:   "diagram <board_id>",
		Short: "Generate a Mermaid-described diagram on a board",
		Long: "Parses Mermaid source (flowchart, sequence, mindmap) and creates\n" +
			"the corresponding shapes and connectors on the target board.\n\n" +
			"Source is read from --diagram <STRING>, --diagram-file <PATH>,\n" +
			"or --diagram-stdin (read from stdin). Output modes:\n" +
			"  discrete (default) — items stay independent\n" +
			"  grouped            — items are bundled into a Miro group\n" +
			"  framed             — items are placed inside a containing frame\n\n" +
			"--use-stencils selects the experimental v2 endpoint for flowchart\n" +
			"stencil shapes (data, document, etc.). Off by default; the standard\n" +
			"shape API is GA-stable.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagram(cmd.Context(), g, cmd.InOrStdin(), args[0], f)
		},
	}
	cmd.Flags().StringVar(&f.diagram, "diagram", "", "Mermaid source as a string")
	cmd.Flags().StringVar(&f.diagramFile, "diagram-file", "", "Path to a file containing Mermaid source")
	cmd.Flags().BoolVar(&f.diagramStdin, "diagram-stdin", false, "Read Mermaid source from stdin")
	cmd.Flags().StringVar(&f.outputMode, "output-mode", "discrete", "Output mode: discrete|grouped|framed")
	cmd.Flags().BoolVar(&f.useStencils, "use-stencils", false, "Use experimental v2 stencil shapes for flowcharts")
	cmd.Flags().Float64Var(&f.startX, "start-x", 0, "Top-left X coordinate of the diagram")
	cmd.Flags().Float64Var(&f.startY, "start-y", 0, "Top-left Y coordinate of the diagram")
	cmd.Flags().Float64Var(&f.nodeWidth, "node-width", 0, "Override the per-node width (default from layout package)")
	cmd.Flags().StringVar(&f.parentID, "parent-id", "", "Parent frame ID to drop nodes into (passed to each shape)")
	return cmd
}

// diagramResult is the JSON payload returned to stdout. Mirrors the
// miro-mcp-server's GenerateDiagramResult so existing consumers of the
// MCP variant see the same shape.
type diagramResult struct {
	NodesCreated      int      `json:"nodes_created"`
	ConnectorsCreated int      `json:"connectors_created"`
	FramesCreated     int      `json:"frames_created"`
	NodeIDs           []string `json:"node_ids"`
	ConnectorIDs      []string `json:"connector_ids"`
	FrameIDs          []string `json:"frame_ids,omitempty"`
	DiagramWidth      float64  `json:"diagram_width"`
	DiagramHeight     float64  `json:"diagram_height"`
	TotalItems        int      `json:"total_items"`
	OutputMode        string   `json:"output_mode"`
	DiagramID         string   `json:"diagram_id,omitempty"`
	DiagramType       string   `json:"diagram_type,omitempty"`
	Message           string   `json:"message"`
}

func runDiagram(ctx context.Context, g *clictx.Globals, stdin io.Reader, boardID string, f diagramFlags) error {
	if err := miro.ValidateID("board_id", boardID); err != nil {
		return err
	}
	source, err := loadDiagramSource(stdin, f)
	if err != nil {
		return err
	}
	if err := diagrams.ValidateDiagramInput(source); err != nil {
		return err
	}

	diagram, err := diagrams.ParseMermaid(source)
	if err != nil {
		if hint := diagrams.DiagramTypeHint(source); hint != "" {
			return fmt.Errorf("parse diagram: %w (hint: %s)", err, hint)
		}
		return fmt.Errorf("parse diagram: %w", err)
	}

	config := buildLayoutConfig(f)
	if diagram.Type == diagrams.TypeSequence {
		applySequenceDiagramOffset(diagram, config)
	} else {
		diagrams.Layout(diagram, config)
	}

	mode := normalizeOutputMode(f.outputMode)
	if mode != "discrete" && mode != "grouped" && mode != "framed" {
		return fmt.Errorf("invalid --output-mode %q: want discrete|grouped|framed", f.outputMode)
	}

	out := diagrams.ConvertToMiroWithOptions(diagram, f.useStencils)

	if g.DryRun {
		return g.EmitDryRun("POST",
			fmt.Sprintf("/v2/boards/%s/{shapes,connectors,frames,groups} × %d items",
				boardID, len(out.Shapes)+len(out.Connectors)+len(out.Frames)))
	}

	client, err := g.BuildClient()
	if err != nil {
		return err
	}

	frameIDs, frameWarnings := createDiagramFrames(ctx, client, boardID, out.Frames)
	nodeIDs, shapeIDMap, shapeWarnings := createDiagramShapes(ctx, client, boardID, out.Shapes, f)
	connectorIDs, connectorWarnings := createDiagramConnectors(ctx, client, boardID, out.Connectors, shapeIDMap)

	for _, w := range frameWarnings {
		_, _ = fmt.Fprintln(g.Stderr, "miro: "+w)
	}
	for _, w := range shapeWarnings {
		_, _ = fmt.Fprintln(g.Stderr, "miro: "+w)
	}
	for _, w := range connectorWarnings {
		_, _ = fmt.Fprintln(g.Stderr, "miro: "+w)
	}

	totalItems := len(nodeIDs) + len(connectorIDs)
	result := diagramResult{
		NodesCreated:      len(nodeIDs),
		ConnectorsCreated: len(connectorIDs),
		FramesCreated:     len(frameIDs),
		NodeIDs:           nodeIDs,
		ConnectorIDs:      connectorIDs,
		FrameIDs:          frameIDs,
		DiagramWidth:      diagram.Width,
		DiagramHeight:     diagram.Height,
		TotalItems:        totalItems,
		OutputMode:        mode,
	}

	switch mode {
	case "grouped":
		finalizeGroupedDiagram(ctx, client, boardID, append(nodeIDs, connectorIDs...), totalItems, &result)
	case "framed":
		finalizeFramedDiagram(ctx, client, boardID, diagram, f, totalItems, &result)
	default:
		result.Message = buildDiscreteMessage(len(nodeIDs), len(connectorIDs), len(frameIDs))
	}

	return g.EmitJSON(result)
}

// loadDiagramSource resolves the Mermaid input from exactly one of the
// three sources. Mutual exclusion is enforced up-front so users get a
// clear error instead of silent flag precedence.
func loadDiagramSource(stdin io.Reader, f diagramFlags) (string, error) {
	count := 0
	if f.diagram != "" {
		count++
	}
	if f.diagramFile != "" {
		count++
	}
	if f.diagramStdin {
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("one of --diagram, --diagram-file, or --diagram-stdin is required")
	}
	if count > 1 {
		return "", fmt.Errorf("--diagram, --diagram-file, and --diagram-stdin are mutually exclusive")
	}
	switch {
	case f.diagram != "":
		return strings.TrimSpace(f.diagram), nil
	case f.diagramFile != "":
		b, err := os.ReadFile(f.diagramFile) //nolint:gosec // G304: operator-supplied path; loading the source they asked us to load.
		if err != nil {
			return "", fmt.Errorf("read --diagram-file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	default:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read --diagram-stdin: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
}

func buildLayoutConfig(f diagramFlags) diagrams.LayoutConfig {
	cfg := diagrams.DefaultLayoutConfig()
	if f.startX != 0 {
		cfg.StartX = f.startX
	}
	if f.startY != 0 {
		cfg.StartY = f.startY
	}
	if f.nodeWidth > 0 {
		cfg.NodeWidth = f.nodeWidth
	}
	return cfg
}

func applySequenceDiagramOffset(d *diagrams.Diagram, cfg diagrams.LayoutConfig) {
	if cfg.StartX == 0 && cfg.StartY == 0 {
		return
	}
	for _, n := range d.Nodes {
		n.X += cfg.StartX
		n.Y += cfg.StartY
	}
	for _, e := range d.Edges {
		e.Y += cfg.StartY
	}
}

func normalizeOutputMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "discrete"
	}
	return mode
}

func buildDiscreteMessage(nodes, connectors, frames int) string {
	parts := make([]string, 0, 3)
	if nodes > 0 {
		parts = append(parts, fmt.Sprintf("%d nodes", nodes))
	}
	if connectors > 0 {
		parts = append(parts, fmt.Sprintf("%d connectors", connectors))
	}
	if frames > 0 {
		parts = append(parts, fmt.Sprintf("%d frames", frames))
	}
	if len(parts) == 0 {
		return "Created diagram"
	}
	return "Created diagram with " + strings.Join(parts, ", ")
}

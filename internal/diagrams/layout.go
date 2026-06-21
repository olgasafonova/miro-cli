package diagrams

import (
	"sort"
)

// LayoutConfig contains configuration for the layout algorithm.
type LayoutConfig struct {
	NodeWidth    float64 // Default node width
	NodeHeight   float64 // Default node height
	NodeSpacingX float64 // Horizontal spacing between nodes
	NodeSpacingY float64 // Vertical spacing between layers
	StartX       float64 // Starting X position
	StartY       float64 // Starting Y position
	Padding      float64 // Padding around subgraphs
}

// DefaultLayoutConfig returns sensible defaults.
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		NodeWidth:    180,
		NodeHeight:   70,
		NodeSpacingX: 80,
		NodeSpacingY: 120,
		StartX:       0,
		StartY:       0,
		Padding:      40,
	}
}

// Layout applies automatic layout to a diagram.
func Layout(diagram *Diagram, config LayoutConfig) {
	if len(diagram.Nodes) == 0 {
		return
	}

	// Assign layers to nodes
	layers := assignLayers(diagram)

	// Order nodes within each layer to minimize edge crossings
	orderLayers(diagram, layers)

	// Calculate positions based on layers and order
	positionNodes(diagram, layers, config)

	// Calculate diagram bounds
	calculateBounds(diagram)
}

// buildAdjacency builds outgoing/incoming adjacency lists for the
// diagram's nodes, ignoring edges whose endpoints aren't known nodes.
// Shared by assignLayers and orderLayers, which both need the same view.
func buildAdjacency(diagram *Diagram) (outgoing, incoming map[string][]string) {
	outgoing = make(map[string][]string)
	incoming = make(map[string][]string)

	for id := range diagram.Nodes {
		outgoing[id] = []string{}
		incoming[id] = []string{}
	}

	for _, edge := range diagram.Edges {
		if _, ok := diagram.Nodes[edge.FromID]; !ok {
			continue
		}
		if _, ok := diagram.Nodes[edge.ToID]; !ok {
			continue
		}
		outgoing[edge.FromID] = append(outgoing[edge.FromID], edge.ToID)
		incoming[edge.ToID] = append(incoming[edge.ToID], edge.FromID)
	}
	return outgoing, incoming
}

// findRoots returns the nodes with no incoming edges, falling back to a
// single arbitrary node when the graph is fully cyclic (no true roots).
func findRoots(diagram *Diagram, incoming map[string][]string) []string {
	var roots []string
	for id := range diagram.Nodes {
		if len(incoming[id]) == 0 {
			roots = append(roots, id)
		}
	}
	if len(roots) == 0 {
		for id := range diagram.Nodes {
			roots = append(roots, id)
			break
		}
	}
	return roots
}

// longestPathLayers assigns each reachable node a layer via BFS from the
// roots (longest path wins), then drops any unreached node into layer 0.
func longestPathLayers(diagram *Diagram, outgoing map[string][]string, roots []string) map[string]int {
	nodeLayer := make(map[string]int)
	queue := make([]string, 0)
	for _, root := range roots {
		nodeLayer[root] = 0
		queue = append(queue, root)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		layer := nodeLayer[node]

		for _, neighbor := range outgoing[node] {
			newLayer := layer + 1
			if existingLayer, ok := nodeLayer[neighbor]; !ok || existingLayer < newLayer {
				nodeLayer[neighbor] = newLayer
				queue = append(queue, neighbor)
			}
		}
	}

	for id := range diagram.Nodes {
		if _, ok := nodeLayer[id]; !ok {
			nodeLayer[id] = 0
		}
	}
	return nodeLayer
}

// assignLayers assigns each node to a layer using longest path.
func assignLayers(diagram *Diagram) map[int][]string {
	outgoing, incoming := buildAdjacency(diagram)
	roots := findRoots(diagram, incoming)
	nodeLayer := longestPathLayers(diagram, outgoing, roots)

	// Group nodes by layer
	layers := make(map[int][]string)
	for id, layer := range nodeLayer {
		layers[layer] = append(layers[layer], id)
	}

	return layers
}

// orderLayers orders nodes within each layer to minimize crossings.
func orderLayers(diagram *Diagram, layers map[int][]string) {
	outgoing, incoming := buildAdjacency(diagram)

	layerNums := sortedLayerNums(layers)

	// Simple barycenter ordering, seeded with each node's index in its
	// layer.
	nodePos := make(map[string]float64)
	for _, l := range layerNums {
		for i, id := range layers[l] {
			nodePos[id] = float64(i)
		}
	}

	// Iterate forward (order by predecessors) then backward (order by
	// successors) to improve crossing reduction.
	for iter := 0; iter < 4; iter++ {
		for i := 1; i < len(layerNums); i++ {
			reorderLayer(layers, layerNums[i], nodePos, incoming)
		}
		for i := len(layerNums) - 2; i >= 0; i-- {
			reorderLayer(layers, layerNums[i], nodePos, outgoing)
		}
	}
}

// reorderLayer recomputes each node's barycenter from its neighbours in
// the given adjacency map, sorts the layer by that barycenter, then
// re-indexes nodePos so the next pass sees a clean 0..n ordering.
func reorderLayer(layers map[int][]string, l int, nodePos map[string]float64, neighbours map[string][]string) {
	for _, id := range layers[l] {
		nodePos[id] = barycenter(id, nodePos, neighbours)
	}
	sort.Slice(layers[l], func(a, b int) bool {
		return nodePos[layers[l][a]] < nodePos[layers[l][b]]
	})
	for j, id := range layers[l] {
		nodePos[id] = float64(j)
	}
}

// barycenter returns the mean position of a node's neighbours, or the
// node's current position when it has none (so isolated nodes hold still).
func barycenter(id string, nodePos map[string]float64, neighbours map[string][]string) float64 {
	adj := neighbours[id]
	if len(adj) == 0 {
		return nodePos[id]
	}
	sum := 0.0
	for _, n := range adj {
		sum += nodePos[n]
	}
	return sum / float64(len(adj))
}

// sortedLayerNums returns the layer indices in ascending order.
func sortedLayerNums(layers map[int][]string) []int {
	layerNums := make([]int, 0, len(layers))
	for l := range layers {
		layerNums = append(layerNums, l)
	}
	sort.Ints(layerNums)
	return layerNums
}

// positionNodes calculates actual x,y positions for nodes.
func positionNodes(diagram *Diagram, layers map[int][]string, config LayoutConfig) {
	layerNums := sortedLayerNums(layers)
	maxLayerWidth := maxLayerWidthOf(layers)

	// Position based on direction
	isHorizontal := diagram.Direction == LeftToRight || diagram.Direction == RightToLeft
	isReversed := diagram.Direction == BottomToTop || diagram.Direction == RightToLeft

	for _, l := range layerNums {
		nodes := layers[l]
		layerIndex := l
		if isReversed {
			layerIndex = len(layerNums) - 1 - l
		}
		slot := layerSlot{
			layerIndex:   layerIndex,
			offset:       centeringOffset(len(nodes), maxLayerWidth, config),
			isHorizontal: isHorizontal,
			config:       config,
		}
		for i, nodeID := range nodes {
			node := diagram.Nodes[nodeID]
			if node == nil {
				continue
			}
			slot.place(node, i)
		}
	}
}

// layerSlot bundles the per-layer placement context so node positioning
// doesn't pass six positional arguments per node.
type layerSlot struct {
	layerIndex   int
	offset       float64
	isHorizontal bool
	config       LayoutConfig
}

// maxLayerWidthOf returns the node count of the widest layer.
func maxLayerWidthOf(layers map[int][]string) int {
	maxLayerWidth := 0
	for _, nodes := range layers {
		if len(nodes) > maxLayerWidth {
			maxLayerWidth = len(nodes)
		}
	}
	return maxLayerWidth
}

// centeringOffset returns the cross-axis offset that centres a layer of
// nodeCount nodes against the widest layer in the diagram.
func centeringOffset(nodeCount, maxLayerWidth int, config LayoutConfig) float64 {
	layerWidth := float64(nodeCount) * (config.NodeWidth + config.NodeSpacingX)
	return (float64(maxLayerWidth)*(config.NodeWidth+config.NodeSpacingX) - layerWidth) / 2
}

// place sets a node's dimensions and x,y position from its index within
// the layer plus the slot's layer index, centering offset, and flow
// direction.
func (s layerSlot) place(node *Node, nodeIndexInLayer int) {
	config := s.config
	node.Width = config.NodeWidth
	node.Height = config.NodeHeight

	nodeIndex := float64(nodeIndexInLayer)
	if s.isHorizontal {
		node.X = config.StartX + float64(s.layerIndex)*(config.NodeWidth+config.NodeSpacingY)
		node.Y = config.StartY + s.offset + nodeIndex*(config.NodeHeight+config.NodeSpacingX)
	} else {
		node.X = config.StartX + s.offset + nodeIndex*(config.NodeWidth+config.NodeSpacingX)
		node.Y = config.StartY + float64(s.layerIndex)*(config.NodeHeight+config.NodeSpacingY)
	}
}

// calculateBounds calculates the overall diagram bounds.
func calculateBounds(diagram *Diagram) {
	if len(diagram.Nodes) == 0 {
		return
	}

	minX, minY := float64(1e9), float64(1e9)
	maxX, maxY := float64(-1e9), float64(-1e9)

	for _, node := range diagram.Nodes {
		if node.X < minX {
			minX = node.X
		}
		if node.Y < minY {
			minY = node.Y
		}
		if node.X+node.Width > maxX {
			maxX = node.X + node.Width
		}
		if node.Y+node.Height > maxY {
			maxY = node.Y + node.Height
		}
	}

	diagram.Width = maxX - minX
	diagram.Height = maxY - minY
}

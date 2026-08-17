package canvas

// boardItem is the geometry-and-label projection of one listed item —
// just enough to draw it. Parsed from the map-shaped item listing the
// CLI works with everywhere else.
type boardItem struct {
	ID        string
	Type      string
	Content   string
	X, Y      float64
	Width     float64
	Height    float64
	FillColor string
	ShapeKind string
}

// boardConnector is the endpoints-and-caption projection of one listed
// connector.
type boardConnector struct {
	ID          string
	StartItemID string
	EndItemID   string
	Caption     string
}

// subMap reads a nested object field, tolerating absence and wrong types.
func subMap(m map[string]any, key string) map[string]any {
	sub, _ := m[key].(map[string]any)
	return sub
}

// str reads a string field from a possibly-nil map.
func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// num reads a numeric field from a possibly-nil map. JSON decoding into
// map[string]any always produces float64 for numbers.
func num(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	f, _ := m[key].(float64)
	return f
}

// parseBoardItem projects one wire item to its drawable fields. The
// shape kind arrives in data.shape, not style (same wire fact the MCP
// server's item parser encodes); content falls back to data.title so
// frames and cards get their labels.
func parseBoardItem(m map[string]any) boardItem {
	data := subMap(m, "data")
	content := str(data, "content")
	if content == "" {
		content = str(data, "title")
	}
	position := subMap(m, "position")
	geometry := subMap(m, "geometry")
	return boardItem{
		ID:        str(m, "id"),
		Type:      str(m, "type"),
		Content:   content,
		X:         num(position, "x"),
		Y:         num(position, "y"),
		Width:     num(geometry, "width"),
		Height:    num(geometry, "height"),
		FillColor: str(subMap(m, "style"), "fillColor"),
		ShapeKind: str(data, "shape"),
	}
}

// parseBoardItems projects a whole listing.
func parseBoardItems(raw []map[string]any) []boardItem {
	items := make([]boardItem, 0, len(raw))
	for _, m := range raw {
		items = append(items, parseBoardItem(m))
	}
	return items
}

// parseBoardConnector projects one wire connector: endpoints live at
// startItem.id / endItem.id, the caption is the first captions[] entry.
func parseBoardConnector(m map[string]any) boardConnector {
	conn := boardConnector{
		ID:          str(m, "id"),
		StartItemID: str(subMap(m, "startItem"), "id"),
		EndItemID:   str(subMap(m, "endItem"), "id"),
	}
	if captions, _ := m["captions"].([]any); len(captions) > 0 {
		first, _ := captions[0].(map[string]any)
		conn.Caption = str(first, "content")
	}
	return conn
}

// parseBoardConnectors projects a whole connector listing.
func parseBoardConnectors(raw []map[string]any) []boardConnector {
	conns := make([]boardConnector, 0, len(raw))
	for _, m := range raw {
		conns = append(conns, parseBoardConnector(m))
	}
	return conns
}

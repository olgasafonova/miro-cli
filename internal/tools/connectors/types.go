// Package connectors holds the hand-authored Cobra subcommands for the
// /v2/boards/{board_id}/connectors/* family of Miro REST endpoints. Phase
// 3b ships create/get/update/delete on the same pattern as
// internal/tools/embeds/, with the connector-specific shape: no position
// or parent envelope, instead startItem/endItem references with optional
// snapTo or relative position, an optional style block, and an optional
// captions array.
package connectors

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// createRequest is the POST /v2/boards/{board_id}/connectors body. The
// API requires startItem and endItem; everything else is optional. We
// mirror Miro's typed-envelope shape so the JSON encoder emits the exact
// payload the API expects.
type createRequest struct {
	StartItem *itemEndpoint   `json:"startItem,omitempty"`
	EndItem   *itemEndpoint   `json:"endItem,omitempty"`
	Shape     string          `json:"shape,omitempty"`
	Captions  []captionData   `json:"captions,omitempty"`
	Style     *connectorStyle `json:"style,omitempty"`
}

// updateRequest is the PATCH body. All sections are pointers because the
// API treats absent sections as "leave alone". Captions slice is replaced
// wholesale when present — Miro PATCH does not support per-caption diffs.
type updateRequest struct {
	StartItem *itemEndpoint   `json:"startItem,omitempty"`
	EndItem   *itemEndpoint   `json:"endItem,omitempty"`
	Shape     string          `json:"shape,omitempty"`
	Captions  []captionData   `json:"captions,omitempty"`
	Style     *connectorStyle `json:"style,omitempty"`

	// captionsSet flags an explicit empty-array intent ("clear all
	// captions") so the encoder can emit "captions": []. Without this
	// guard, an empty slice would be omitted by omitempty and the API
	// would leave existing captions alone.
	captionsSet bool
}

// MarshalJSON honors captionsSet by emitting an explicit empty array
// when the user passed --clear-captions but no --caption entries.
func (u updateRequest) MarshalJSON() ([]byte, error) {
	// Use an inline alias to avoid recursion through MarshalJSON.
	type alias struct {
		StartItem *itemEndpoint   `json:"startItem,omitempty"`
		EndItem   *itemEndpoint   `json:"endItem,omitempty"`
		Shape     string          `json:"shape,omitempty"`
		Captions  *[]captionData  `json:"captions,omitempty"`
		Style     *connectorStyle `json:"style,omitempty"`
	}
	a := alias{
		StartItem: u.StartItem,
		EndItem:   u.EndItem,
		Shape:     u.Shape,
		Style:     u.Style,
	}
	if u.captionsSet {
		// Always emit when captionsSet — even an empty slice means
		// "clear all captions".
		cap := u.Captions
		if cap == nil {
			cap = []captionData{}
		}
		a.Captions = &cap
	}
	return json.Marshal(a)
}

// itemEndpoint mirrors ItemConnectionCreationData / ItemConnectionChangesData.
// id is the target item; snapTo and position are mutually exclusive per
// the Miro docs (we don't enforce that client-side — let the API surface
// the 400 with its own message).
type itemEndpoint struct {
	ID       string          `json:"id,omitempty"`
	SnapTo   string          `json:"snapTo,omitempty"`
	Position *relativeOffset `json:"position,omitempty"`
}

// relativeOffset is the {x, y} percentage envelope. Both axes are
// strings like "50%" per the spec, not floats. Empty means caller did
// not set that axis.
type relativeOffset struct {
	X string `json:"x,omitempty"`
	Y string `json:"y,omitempty"`
}

// captionData mirrors the Caption schema. content is required; position
// is a percentage string ("50%"); textAlignVertical is one of
// top|middle|bottom. We accept content+position pairs from a repeatable
// --caption flag and let the API default position to 50% if unset.
type captionData struct {
	Content           string `json:"content"`
	Position          string `json:"position,omitempty"`
	TextAlignVertical string `json:"textAlignVertical,omitempty"`
}

// connectorStyle mirrors ConnectorStyle / UpdateConnectorStyle. fontSize
// and strokeWidth are strings ("14", "2.0") matching the existing
// shapes/texts pattern of fontSize-as-string. color is the caption text
// color; strokeColor is the line color.
type connectorStyle struct {
	Color           string `json:"color,omitempty"`
	StrokeColor     string `json:"strokeColor,omitempty"`
	StrokeWidth     string `json:"strokeWidth,omitempty"`
	StrokeStyle     string `json:"strokeStyle,omitempty"`
	StartStrokeCap  string `json:"startStrokeCap,omitempty"`
	EndStrokeCap    string `json:"endStrokeCap,omitempty"`
	FontSize        string `json:"fontSize,omitempty"`
	TextOrientation string `json:"textOrientation,omitempty"`
}

// deleteResult is the synthesized JSON envelope emitted after a 204.
// Agents branch on `deleted` rather than inspecting exit codes.
type deleteResult struct {
	Deleted bool   `json:"deleted"`
	ID      string `json:"id"`
}

// validateShape rejects any --shape value that isn't one of the three
// strings Miro's API accepts. Empty passes (caller didn't set it).
func validateShape(s string) error {
	switch s {
	case "", "curved", "straight", "elbowed":
		return nil
	default:
		return fmt.Errorf("invalid --shape %q: must be curved, straight, or elbowed", s)
	}
}

// validateSnapTo rejects any snap-to value that isn't in the API enum.
func validateSnapTo(s, flagName string) error {
	switch s {
	case "", "auto", "top", "right", "bottom", "left":
		return nil
	default:
		return fmt.Errorf("invalid --%s %q: must be auto, top, right, bottom, or left", flagName, s)
	}
}

// validateStrokeStyle rejects any value that isn't normal|dotted|dashed.
func validateStrokeStyle(s string) error {
	switch s {
	case "", "normal", "dotted", "dashed":
		return nil
	default:
		return fmt.Errorf("invalid --stroke-style %q: must be normal, dotted, or dashed", s)
	}
}

// validateStrokeCap rejects any decoration-cap value not in the spec
// enum. The list is long but stable; matching it client-side gives a
// clearer error than the API's generic 400.
func validateStrokeCap(s, flagName string) error {
	switch s {
	case "",
		"none", "stealth", "rounded_stealth",
		"diamond", "filled_diamond",
		"oval", "filled_oval",
		"arrow", "triangle", "filled_triangle",
		"erd_one", "erd_many",
		"erd_only_one", "erd_zero_or_one",
		"erd_one_or_many", "erd_zero_or_many",
		"unknown":
		return nil
	default:
		return fmt.Errorf("invalid --%s %q: see API docs for allowed cap values", flagName, s)
	}
}

// validateTextOrientation rejects any value that isn't horizontal|aligned.
func validateTextOrientation(s string) error {
	switch s {
	case "", "horizontal", "aligned":
		return nil
	default:
		return fmt.Errorf("invalid --text-orientation %q: must be horizontal or aligned", s)
	}
}

// validateTextAlignVertical rejects any caption vertical-align value
// that isn't top|middle|bottom.
func validateTextAlignVertical(s string) error {
	switch s {
	case "", "top", "middle", "bottom":
		return nil
	default:
		return fmt.Errorf("invalid caption text-align-vertical %q: must be top, middle, or bottom", s)
	}
}

// parsePosition splits a "X%,Y%" pair into the two percentage strings.
// Empty input returns nil, nil (caller did not set the flag). Both axes
// are optional — "50%," sets only X, ",25%" sets only Y. Whitespace is
// trimmed. The function does not validate that the values are numeric
// percentages; the API returns a clear 400 for malformed offsets.
func parsePosition(s string) (*relativeOffset, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid position %q: must be X%%,Y%% (e.g. 50%%,25%%)", s)
	}
	off := &relativeOffset{
		X: strings.TrimSpace(parts[0]),
		Y: strings.TrimSpace(parts[1]),
	}
	if off.X == "" && off.Y == "" {
		return nil, fmt.Errorf("invalid position %q: at least one of X or Y must be set", s)
	}
	return off, nil
}

// parseCaption splits a "text@position" pair. The position suffix is
// optional; "Hello" alone is a valid caption with no explicit position.
// The position is whatever follows the last "@" so caption text may
// contain "@" symbols freely as long as the closing position string
// looks like a percentage.
func parseCaption(s string) (captionData, error) {
	if s == "" {
		return captionData{}, fmt.Errorf("--caption requires non-empty content")
	}
	idx := strings.LastIndex(s, "@")
	if idx < 0 {
		return captionData{Content: s}, nil
	}
	suffix := strings.TrimSpace(s[idx+1:])
	// Only treat @suffix as position if it looks like a percentage
	// (digits optionally followed by %). Anything else stays part of
	// the content.
	if !looksLikePercent(suffix) {
		return captionData{Content: s}, nil
	}
	content := strings.TrimSpace(s[:idx])
	if content == "" {
		return captionData{}, fmt.Errorf("--caption %q has empty content before @position", s)
	}
	return captionData{Content: content, Position: suffix}, nil
}

// looksLikePercent reports whether s is "<digits>%" or "<digits>". The
// trailing % is optional; "50" works as well as "50%".
func looksLikePercent(s string) bool {
	s = strings.TrimSuffix(s, "%")
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

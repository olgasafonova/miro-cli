package diagrams

import (
	"fmt"
	"strings"
)

// DiagramError represents a diagram parsing or layout error with helpful suggestions.
type DiagramError struct {
	Code       string // Error code for programmatic handling
	Message    string // User-friendly error message
	Suggestion string // Actionable suggestion to fix the error
	Line       int    // Line number where error occurred (0 if unknown)
	Input      string // Relevant input that caused the error
}

// Error implements the error interface.
func (e *DiagramError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)
	if e.Line > 0 {
		fmt.Fprintf(&sb, " (line %d)", e.Line)
	}
	if e.Suggestion != "" {
		sb.WriteString(". ")
		sb.WriteString(e.Suggestion)
	}
	return sb.String()
}

// Error codes for diagram parsing errors
const (
	ErrCodeNoNodes        = "NO_NODES"
	ErrCodeInvalidSyntax  = "INVALID_SYNTAX"
	ErrCodeMissingHeader  = "MISSING_HEADER"
	ErrCodeEmptyDiagram   = "EMPTY_DIAGRAM"
	ErrCodeInvalidShape   = "INVALID_SHAPE"
	ErrCodeCircularRef    = "CIRCULAR_REFERENCE"
	ErrCodeTooManyNodes   = "TOO_MANY_NODES"
	ErrCodeInvalidEdge    = "INVALID_EDGE"
	ErrCodeUnknownDiagram = "UNKNOWN_DIAGRAM_TYPE"
	ErrCodeInputTooLarge  = "INPUT_TOO_LARGE"
	ErrCodeTooManyLines   = "TOO_MANY_LINES"
	ErrCodeLineTooLong    = "LINE_TOO_LONG"
)

// Input size limits for ReDoS protection
const (
	// MaxDiagramInputSize is the maximum input size in bytes (50KB).
	MaxDiagramInputSize = 50 * 1024

	// MaxDiagramLines is the maximum number of lines in a diagram.
	MaxDiagramLines = 500

	// MaxLineLength is the maximum length of a single line.
	MaxLineLength = 2000
)

// NewDiagramError creates a new DiagramError with the given code and message.
func NewDiagramError(code, message string) *DiagramError {
	return &DiagramError{
		Code:    code,
		Message: message,
	}
}

// WithSuggestion adds a suggestion to the error.
func (e *DiagramError) WithSuggestion(suggestion string) *DiagramError {
	e.Suggestion = suggestion
	return e
}

// WithLine adds line number information to the error.
func (e *DiagramError) WithLine(line int) *DiagramError {
	e.Line = line
	return e
}

// WithInput adds the input that caused the error.
func (e *DiagramError) WithInput(input string) *DiagramError {
	// Truncate long inputs
	if len(input) > 50 {
		input = input[:47] + "..."
	}
	e.Input = input
	return e
}

// Common diagram errors with helpful suggestions

// ErrNoNodes is returned when no nodes are found in the diagram.
var ErrNoNodes = NewDiagramError(
	ErrCodeNoNodes,
	"no nodes found in diagram",
).WithSuggestion("Add node definitions like 'A[Label]' or edges like 'A --> B'. Example: flowchart TB\\n    A[Start] --> B[End]")

// ErrEmptyDiagram is returned when the diagram input is empty.
var ErrEmptyDiagram = NewDiagramError(
	ErrCodeEmptyDiagram,
	"diagram input is empty",
).WithSuggestion("Provide Mermaid diagram code starting with 'flowchart TB' or 'sequenceDiagram'")

// ErrMissingFlowchartHeader is returned when the flowchart header is missing.
var ErrMissingFlowchartHeader = NewDiagramError(
	ErrCodeMissingHeader,
	"missing diagram header",
).WithSuggestion("Start your diagram with 'flowchart TB' (top-bottom), 'flowchart LR' (left-right), or 'sequenceDiagram'")

// ErrMissingSequenceHeader is returned when the sequenceDiagram header is missing.
var ErrMissingSequenceHeader = NewDiagramError(
	ErrCodeMissingHeader,
	"not a sequence diagram: missing 'sequenceDiagram' header",
).WithSuggestion("Start your sequence diagram with 'sequenceDiagram' on the first line")

// ErrNoParticipants is returned when no participants are found in a sequence diagram.
var ErrNoParticipants = NewDiagramError(
	ErrCodeNoNodes,
	"no participants found in sequence diagram",
).WithSuggestion("Add participants using 'participant A' or messages like 'A->>B: Hello'")

// ErrTooManyNodes is returned when the diagram exceeds node limits.
func ErrTooManyNodes(count, limit int) *DiagramError {
	return NewDiagramError(
		ErrCodeTooManyNodes,
		fmt.Sprintf("diagram has %d nodes, exceeding limit of %d", count, limit),
	).WithSuggestion("Split the diagram into smaller subgraphs or reduce the number of nodes")
}

// ErrInputTooLarge is returned when the input exceeds size limits.
var ErrInputTooLarge = NewDiagramError(
	ErrCodeInputTooLarge,
	fmt.Sprintf("diagram input exceeds maximum size of %d bytes", MaxDiagramInputSize),
).WithSuggestion("Reduce diagram size or split into multiple smaller diagrams")

// ErrTooManyLinesError is returned when the diagram has too many lines.
func ErrTooManyLinesError(count int) *DiagramError {
	return NewDiagramError(
		ErrCodeTooManyLines,
		fmt.Sprintf("diagram has %d lines, exceeding limit of %d", count, MaxDiagramLines),
	).WithSuggestion("Reduce the number of lines or split into multiple diagrams")
}

// ErrLineTooLongError is returned when a line exceeds the maximum length.
func ErrLineTooLongError(lineNum, length int) *DiagramError {
	return NewDiagramError(
		ErrCodeLineTooLong,
		fmt.Sprintf("line %d has %d characters, exceeding limit of %d", lineNum, length, MaxLineLength),
	).WithLine(lineNum).WithSuggestion("Split long labels or node names into shorter segments")
}

// ErrInvalidNodeShape is returned when an unrecognized shape syntax is used.
func ErrInvalidNodeShape(shape string) *DiagramError {
	return NewDiagramError(
		ErrCodeInvalidShape,
		fmt.Sprintf("unrecognized node shape: %s", shape),
	).WithSuggestion("Use valid shapes: [text] for rectangle, (text) for rounded, {text} for diamond, ((text)) for circle, {{text}} for hexagon")
}

// ErrInvalidEdge is returned when an edge references a non-existent node.
func ErrInvalidEdge(fromID, toID string) *DiagramError {
	return NewDiagramError(
		ErrCodeInvalidEdge,
		fmt.Sprintf("edge references undefined node: %s or %s", fromID, toID),
	).WithSuggestion("Ensure all nodes in edges are defined. Define nodes before using them in edges")
}

// ParseDiagramSyntaxError creates an error for general syntax issues.
func ParseDiagramSyntaxError(line int, content string, reason string) *DiagramError {
	return NewDiagramError(
		ErrCodeInvalidSyntax,
		fmt.Sprintf("syntax error: %s", reason),
	).WithLine(line).WithInput(content).WithSuggestion("Check Mermaid syntax at https://mermaid.js.org/syntax/flowchart.html")
}

// DiagramTypeHint returns a helpful hint based on the input content.
func DiagramTypeHint(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))

	// Check for common mistakes
	switch {
	case usesSingleArrowSyntax(input):
		return singleArrowHint(input)
	case hasParticipantWithoutSequenceHeader(input):
		return "Sequence diagrams must start with 'sequenceDiagram'"
	case hasSubgraphWithoutFlowchartHeader(input):
		return "Flowcharts with subgraphs must start with 'flowchart TB' or 'graph TD'"
	default:
		return ""
	}
}

// usesSingleArrowSyntax reports whether the input uses '->' arrows without any '-->'.
func usesSingleArrowSyntax(input string) bool {
	return strings.Contains(input, "->") && !strings.Contains(input, "-->")
}

// singleArrowHint suggests the correct arrow syntax for the diagram type.
func singleArrowHint(input string) string {
	if strings.Contains(input, "sequencediagram") {
		return "Sequence diagrams use '->>': A->>B: message"
	}
	return "Flowcharts use '-->': A --> B"
}

// hasParticipantWithoutSequenceHeader reports whether participants appear
// without the required 'sequenceDiagram' header.
func hasParticipantWithoutSequenceHeader(input string) bool {
	return strings.Contains(input, "participant") && !strings.HasPrefix(input, "sequencediagram")
}

// hasSubgraphWithoutFlowchartHeader reports whether subgraphs appear without
// a 'flowchart' or 'graph' header.
func hasSubgraphWithoutFlowchartHeader(input string) bool {
	return strings.Contains(input, "subgraph") &&
		!strings.HasPrefix(input, "flowchart") &&
		!strings.HasPrefix(input, "graph")
}

// ValidateDiagramInput performs validation on diagram input including ReDoS protection.
func ValidateDiagramInput(input string) error {
	// Check total input size (ReDoS protection)
	if len(input) > MaxDiagramInputSize {
		return ErrInputTooLarge
	}

	input = strings.TrimSpace(input)

	if input == "" {
		return ErrEmptyDiagram
	}

	lines := strings.Split(input, "\n")
	if err := checkLineLimits(lines); err != nil {
		return err
	}

	return checkDiagramHeader(input, lines)
}

// checkLineLimits enforces the line-count and per-line length limits (ReDoS protection).
func checkLineLimits(lines []string) error {
	if len(lines) > MaxDiagramLines {
		return ErrTooManyLinesError(len(lines))
	}

	for i, line := range lines {
		if len(line) > MaxLineLength {
			return ErrLineTooLongError(i+1, len(line))
		}
	}

	return nil
}

// checkDiagramHeader verifies that the first meaningful line is a valid diagram header.
func checkDiagramHeader(input string, lines []string) error {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if isSkippableLine(line) {
			continue
		}

		if isValidDiagramHeader(line) {
			return nil
		}

		return missingHeaderError(input)
	}

	return nil
}

// isSkippableLine reports whether the line is empty or a Mermaid comment.
func isSkippableLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "%%")
}

// isValidDiagramHeader reports whether the line is a recognized diagram header.
func isValidDiagramHeader(line string) bool {
	lineLower := strings.ToLower(line)
	return strings.HasPrefix(lineLower, "flowchart") ||
		strings.HasPrefix(lineLower, "graph") ||
		lineLower == "sequencediagram"
}

// missingHeaderError builds the missing-header error, enriched with a type hint.
func missingHeaderError(input string) *DiagramError {
	err := NewDiagramError(
		ErrCodeMissingHeader,
		"diagram must start with a valid header",
	).WithSuggestion("Use 'flowchart TB', 'flowchart LR', 'graph TD', or 'sequenceDiagram'")

	if hint := DiagramTypeHint(input); hint != "" {
		err.Suggestion += ". " + hint
	}

	return err
}

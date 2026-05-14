package miro

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// EnvDebug enables verbose panic diagnostics on stderr. When unset, a
// panic surfaces as a structured error with a correlation ID only; the
// panic value and stack are suppressed so they don't leak into transcripts
// shared with third parties.
const EnvDebug = "MIRO_DEBUG"

// PanicError is the error returned from RunWithRecover when the wrapped
// function panicked. CorrelationID lets the operator grep their debug log
// for the matching stack trace; Value is the recovered panic value and is
// intentionally not included in Error() — keep it out of stderr by default.
type PanicError struct {
	CorrelationID string
	Value         any
}

func (e *PanicError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("miro: internal error (id: %s)", e.CorrelationID)
}

// RunWithRecover invokes fn and converts a panic into a *PanicError.
// On panic, full diagnostics (panic value + stack trace) are written to
// debugW only when EnvDebug is truthy; the returned error never carries
// the panic value in its string form. This is the CLI counterpart to the
// MCP-server "wrap every handler call in defer recover()" rule (HG-1).
func RunWithRecover(debugW io.Writer, fn func() error) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		id := newCorrelationID()
		if debugWantsDiagnostics() && debugW != nil {
			_, _ = fmt.Fprintf(debugW, "miro: panic (id: %s): %v\n%s\n", id, r, debug.Stack())
		}
		err = &PanicError{CorrelationID: id, Value: r}
	}()
	return fn()
}

func debugWantsDiagnostics() bool {
	v := os.Getenv(EnvDebug)
	return v != "" && v != "0" && v != "false"
}

// newCorrelationID returns 8 hex characters from crypto/rand. Short enough
// to read aloud or paste into a bug report, long enough to disambiguate
// concurrent panics in a single session.
func newCorrelationID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on a desktop is extraordinary; fall back to
		// a fixed sentinel so callers still get *something* unique-ish via
		// the panic value in debug mode.
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}

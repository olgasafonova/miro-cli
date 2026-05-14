package miro

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// ExitInternalError is the exit code reported when miro-cli aborts due
// to an internal panic. Chosen to match BSD sysexits.h EX_SOFTWARE so
// shell wrappers can distinguish "internal bug" from API failures.
const ExitInternalError = 70

// CrashDumpEnv is the environment variable callers can set to a file
// path; when present, crash dumps (panic value + stack trace) are
// appended to that file instead of being silently dropped.
const CrashDumpEnv = "MIRO_DEBUG_DUMP"

// CorrelationID returns a 16-character hex token suitable for tagging a
// crash report. The user is asked to quote this ID when filing an issue,
// and the same ID appears in the dump file so support can match them.
func CorrelationID() string {
	var b [8]byte
	// crypto/rand.Read never returns an error on supported platforms;
	// if it ever does, falling back to a timestamp is still better than
	// panicking inside the panic handler.
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("ts%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// WriteCrashDump appends a structured crash record to w. The panic
// value and stack trace are kept inside the dump file — they are NOT
// returned to the caller's stderr, because either can carry token
// fragments, request bodies, or other sensitive values that happened
// to be on the stack at the moment of panic.
func WriteCrashDump(w io.Writer, id string, recovered any, stack []byte) error {
	_, err := fmt.Fprintf(w,
		"time=%s correlation_id=%s panic=%v\n%s\n",
		time.Now().UTC().Format(time.RFC3339Nano), id, recovered, stack)
	return err
}

// OpenCrashDump opens the path named by CrashDumpEnv for append-only
// writing. Returns (nil, nil) if the env var is unset — callers must
// handle that case as "no dump location configured." The path comes
// from a deliberately-set env var, not from user input, so gosec's
// G304 file-inclusion warning is suppressed.
func OpenCrashDump() (*os.File, error) {
	path := os.Getenv(CrashDumpEnv)
	if path == "" {
		return nil, nil
	}
	// #nosec G304 G703 -- path comes from MIRO_DEBUG_DUMP, set deliberately by the operator to capture crash dumps; not user-supplied per-request input.
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}

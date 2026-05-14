package miro

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCorrelationIDFormat(t *testing.T) {
	id := CorrelationID()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("CorrelationID = %q, want 16 lowercase hex chars", id)
	}
}

func TestCorrelationIDUnique(t *testing.T) {
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := CorrelationID()
		if _, dup := seen[id]; dup {
			t.Fatalf("CorrelationID collision at iteration %d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestWriteCrashDumpFields(t *testing.T) {
	var buf bytes.Buffer
	err := WriteCrashDump(&buf, "abc123", "boom", []byte("goroutine 1 [running]:\n"))
	if err != nil {
		t.Fatalf("WriteCrashDump: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"correlation_id=abc123", "panic=boom", "goroutine 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("dump missing %q\n%s", want, got)
		}
	}
}

func TestOpenCrashDumpUnsetReturnsNil(t *testing.T) {
	t.Setenv(CrashDumpEnv, "")
	f, err := OpenCrashDump()
	if err != nil {
		t.Fatalf("OpenCrashDump with unset env: %v", err)
	}
	if f != nil {
		_ = f.Close()
		t.Errorf("OpenCrashDump with unset env returned non-nil file")
	}
}

func TestOpenCrashDumpAppendOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crash.log")
	t.Setenv(CrashDumpEnv, path)

	// First open + write.
	f, err := OpenCrashDump()
	if err != nil {
		t.Fatalf("first OpenCrashDump: %v", err)
	}
	if f == nil {
		t.Fatal("OpenCrashDump returned nil with env set")
	}
	if err := WriteCrashDump(f, "id1", "first", []byte("stack-A")); err != nil {
		t.Fatalf("first WriteCrashDump: %v", err)
	}
	_ = f.Close()

	// Second open must append, not truncate.
	f2, err := OpenCrashDump()
	if err != nil {
		t.Fatalf("second OpenCrashDump: %v", err)
	}
	if err := WriteCrashDump(f2, "id2", "second", []byte("stack-B")); err != nil {
		t.Fatalf("second WriteCrashDump: %v", err)
	}
	_ = f2.Close()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump file: %v", err)
	}
	for _, want := range []string{"id1", "id2", "first", "second", "stack-A", "stack-B"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("dump file missing %q\n%s", want, got)
		}
	}
}

func TestOpenCrashDumpMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crash.log")
	t.Setenv(CrashDumpEnv, path)

	f, err := OpenCrashDump()
	if err != nil {
		t.Fatalf("OpenCrashDump: %v", err)
	}
	_ = f.Close()

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// 0o600: owner read/write only. The dump may contain stack frames
	// referencing sensitive values; world-readable would be a leak.
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("crash dump file mode = %o, want 0600", perm)
	}
}

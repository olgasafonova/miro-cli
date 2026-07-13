package clictx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileOrStdinReadsFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "payload.json")
	want := `["a","b"]`
	if err := os.WriteFile(p, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFileOrStdin(p)
	if err != nil {
		t.Fatalf("ReadFileOrStdin(file): %v", err)
	}
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadFileOrStdinReadsStdinOnDash(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	want := `["x","y","z"]`
	go func() {
		_, _ = w.WriteString(want)
		_ = w.Close()
	}()

	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	got, err := ReadFileOrStdin("-")
	if err != nil {
		t.Fatalf("ReadFileOrStdin(-): %v", err)
	}
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

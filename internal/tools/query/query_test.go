package query

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olgasafonova/miro-cli/internal/store"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

func TestValidateSelect(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{"plain select", "SELECT * FROM boards", false},
		{"lowercase select", "select id from boards", false},
		{"leading whitespace", "   SELECT 1", false},
		{"with cte", "WITH t AS (SELECT 1) SELECT * FROM t", false},
		{"leading comment then select", "-- name: get boards\nSELECT * FROM boards", false},
		{"trailing semicolon ok", "SELECT 1;", false},
		{"empty", "", true},
		{"insert", "INSERT INTO boards VALUES (1)", true},
		{"update", "UPDATE boards SET name = 'x'", true},
		{"delete", "DELETE FROM boards", true},
		{"drop", "DROP TABLE boards", true},
		{"pragma", "PRAGMA user_version", true},
		{"attach", "ATTACH DATABASE 'x' AS y", true},
		{"two statements", "SELECT 1; DROP TABLE boards", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSelect(tc.sql)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateSelect(%q) err = %v, wantErr = %v", tc.sql, err, tc.wantErr)
			}
		})
	}
}

// seedStore creates a populated store at a temp path and returns the path.
func seedStore(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := store.Open(ctx, path)
	if err != nil {
		t.Fatalf("seed store: %v", err)
	}
	defer func() { _ = s.Close() }()

	if err := s.UpsertBoards(ctx, []store.Board{
		{ID: "b1", Name: "Roadmap", RawJSON: []byte(`{}`)},
		{ID: "b2", Name: "Retro", RawJSON: []byte(`{}`)},
	}); err != nil {
		t.Fatalf("seed boards: %v", err)
	}
	if err := s.UpsertItems(ctx, []store.Item{
		{ID: "i1", BoardID: "b1", Type: "sticky_note", RawJSON: []byte(`{}`)},
		{ID: "i2", BoardID: "b1", Type: "shape", RawJSON: []byte(`{}`)},
		{ID: "i3", BoardID: "b2", Type: "sticky_note", RawJSON: []byte(`{}`)},
	}); err != nil {
		t.Fatalf("seed items: %v", err)
	}
	return path
}

func TestRunEmitsJSONResults(t *testing.T) {
	path := seedStore(t)
	var stdout bytes.Buffer
	g := &clictx.Globals{
		StorePath: path,
		JSON:      true, // force JSON regardless of tty detection
		Stdout:    &stdout,
		Stderr:    io.Discard,
	}
	if err := run(context.Background(), g, `SELECT id, name FROM boards ORDER BY id`, DefaultRowLimit); err != nil {
		t.Fatalf("run: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, stdout.String())
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0]["id"] != "b1" || rows[0]["name"] != "Roadmap" {
		t.Errorf("first row = %+v, want {id:b1, name:Roadmap}", rows[0])
	}
}

func TestRunRespectsLimit(t *testing.T) {
	path := seedStore(t)
	var stdout bytes.Buffer
	g := &clictx.Globals{
		StorePath: path,
		JSON:      true,
		Stdout:    &stdout,
		Stderr:    io.Discard,
	}
	if err := run(context.Background(), g, `SELECT id FROM items`, 2); err != nil {
		t.Fatalf("run: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("limit=2 produced %d rows, want 2", len(rows))
	}
}

func TestRunRefusesNonSelect(t *testing.T) {
	path := seedStore(t)
	g := &clictx.Globals{
		StorePath: path,
		JSON:      true,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	}
	err := run(context.Background(), g, `DELETE FROM boards`, DefaultRowLimit)
	if err == nil {
		t.Fatal("DELETE accepted; want error")
	}
	if !strings.Contains(err.Error(), "SELECT") {
		t.Errorf("error did not mention SELECT requirement: %v", err)
	}
}

func TestRunRefusesWriteThatBypassesPrecheck(t *testing.T) {
	// PRAGMA query_only is the second line of defense. A WITH-prefixed
	// statement that mutates inside the CTE passes the leading-keyword
	// check (starts with "with") but must still be rejected at the
	// driver level. SQLite accepts INSERT/UPDATE/DELETE inside a CTE,
	// so this is a real attack surface the regex alone wouldn't catch.
	path := seedStore(t)
	g := &clictx.Globals{
		StorePath: path,
		JSON:      true,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	}
	err := run(context.Background(), g,
		`WITH deleted AS (DELETE FROM boards RETURNING id) SELECT * FROM deleted`,
		DefaultRowLimit)
	if err == nil {
		t.Error("write-via-CTE accepted by read-only handle; query_only/mode=ro failed")
	}
}

func TestRunMissingStore(t *testing.T) {
	g := &clictx.Globals{
		StorePath: filepath.Join(t.TempDir(), "absent.db"),
		JSON:      true,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	}
	err := run(context.Background(), g, `SELECT 1`, DefaultRowLimit)
	if err == nil {
		t.Error("expected error opening nonexistent store")
	}
}

func TestRunEmitsTableWhenNotJSON(t *testing.T) {
	path := seedStore(t)
	var stdout bytes.Buffer
	// JSON unset and Stdout not a *os.File → isTerminal returns false →
	// JSON fallback. To exercise the table path we call emitTable directly,
	// which mirrors what run() does for terminal stdouts.
	rows := []row{
		{"id": "b1", "name": "Roadmap"},
		{"id": "b2", "name": "Retro"},
	}
	if err := emitTable(&stdout, []string{"id", "name"}, rows); err != nil {
		t.Fatalf("emitTable: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "id\tname") {
		t.Errorf("table header missing: %q", got)
	}
	if !strings.Contains(got, "b1\tRoadmap") {
		t.Errorf("table row missing: %q", got)
	}
	_ = path
}

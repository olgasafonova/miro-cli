// Package query implements `miro query <sql>` — a read-only SQL passthrough
// against the local store populated by `miro sync`. SELECT (and CTE-prefixed
// SELECT) statements only; the underlying connection is opened read-only at
// the OS level AND has PRAGMA query_only=ON, so any path that bypasses the
// regex pre-check still fails at the driver. Output is JSON by default and
// a tab-separated table when stdout is a terminal.
package query

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/olgasafonova/miro-cli/internal/store"
	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// DefaultRowLimit caps the number of rows returned by a single query unless
// the user overrides it with --limit. The cap protects against an accidental
// `SELECT * FROM items` exhausting memory on a fully-synced store.
const DefaultRowLimit = 1000

// NewCmd returns the `query` command. The single positional argument is the
// SQL statement; we deliberately do not accept SQL on stdin (you can wrap
// the call in `miro query "$(cat my-query.sql)"` if you want to). Keeping
// the surface narrow makes the SELECT-only guard easier to reason about.
func NewCmd(g *clictx.Globals) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "query <sql>",
		Short: "Run a read-only SQL query against the local store",
		Long: "Opens the local SQLite store (populated by `miro sync`) in\n" +
			"read-only mode and runs the supplied SELECT statement.\n\n" +
			"Refuses non-SELECT input. The connection is opened with mode=ro\n" +
			"and PRAGMA query_only=ON, so even if the syntax pre-check is\n" +
			"bypassed the driver rejects writes.\n\n" +
			"Output is JSON by default. When stdout is a terminal the rows\n" +
			"are rendered as a tab-separated table; pipe through `column -t`\n" +
			"for aligned columns. Use --json to force JSON in either case.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), g, args[0], limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", DefaultRowLimit, "Maximum rows returned (0 disables the cap)")
	return cmd
}

func run(ctx context.Context, g *clictx.Globals, sqlText string, limit int) error {
	if err := validateSelect(sqlText); err != nil {
		return err
	}
	path := g.StorePath
	if path == "" {
		var err error
		path, err = store.DefaultPath()
		if err != nil {
			return err
		}
	}

	s, err := store.OpenReadOnly(ctx, path)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer func() { _ = s.Close() }()

	rows, err := s.DB().QueryContext(ctx, sqlText)
	if err != nil {
		return fmt.Errorf("query: execute: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("query: columns: %w", err)
	}

	results, err := collect(rows, cols, limit)
	if err != nil {
		return err
	}

	if g.JSON || !isTerminal(g.Stdout) {
		return g.EmitJSON(results)
	}
	return emitTable(g.Stdout, cols, results)
}

// validateSelect refuses anything other than a SELECT (or a CTE that wraps
// a SELECT). PRAGMA query_only is the actual enforcement; this check is
// for friendlier error messages and to refuse multi-statement input early.
func validateSelect(sqlText string) error {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return errors.New("query: empty SQL")
	}
	// Strip a single leading line comment so `-- name: foo\nselect ...`
	// patterns still pass. Anything more elaborate is the user's problem.
	if strings.HasPrefix(trimmed, "--") {
		if nl := strings.IndexByte(trimmed, '\n'); nl > 0 {
			trimmed = strings.TrimSpace(trimmed[nl+1:])
		}
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("query: only SELECT statements are allowed (got %q)", firstWord(trimmed))
	}
	// Refuse trailing-semicolon-then-statement combos. One trailing
	// semicolon is fine; anything after it is rejected.
	if idx := strings.IndexByte(trimmed, ';'); idx >= 0 {
		tail := strings.TrimSpace(trimmed[idx+1:])
		if tail != "" {
			return errors.New("query: multiple statements not allowed")
		}
	}
	return nil
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			return s[:i]
		}
	}
	return s
}

// row is the map shape used for both JSON and table output. Column ordering
// is preserved by the parallel cols slice; map iteration order doesn't
// matter for JSON because emitTable uses cols anyway.
type row map[string]any

func collect(rows *sql.Rows, cols []string, limit int) ([]row, error) {
	var out []row
	for rows.Next() {
		if limit > 0 && len(out) >= limit {
			// Drain the cursor so the driver releases its resources
			// before we close, but stop appending.
			break
		}
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("query: scan: %w", err)
		}
		r := make(row, len(cols))
		for i, c := range cols {
			r[c] = normaliseValue(values[i])
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query: iterate: %w", err)
	}
	return out, nil
}

// normaliseValue turns driver-specific scan types into JSON-friendly ones.
// []byte for TEXT columns is the main case worth catching; everything else
// (int64, float64, bool, nil, time.Time) marshals cleanly already.
func normaliseValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return t
	}
}

func emitTable(w io.Writer, cols []string, rows []row) error {
	if _, err := fmt.Fprintln(w, strings.Join(cols, "\t")); err != nil {
		return err
	}
	for _, r := range rows {
		fields := make([]string, len(cols))
		for i, c := range cols {
			fields[i] = formatCell(r[c])
		}
		if _, err := fmt.Fprintln(w, strings.Join(fields, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func formatCell(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// isTerminal reports whether w is a terminal stdout. The only writer we
// expect from production is *os.File; tests pass *bytes.Buffer or io.Discard
// where the answer is unambiguously "not a terminal", letting the default
// branch handle them.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

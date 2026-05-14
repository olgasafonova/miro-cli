// Package store provides local SQLite persistence for miro-cli. Sibling
// beads will layer a sync command (downloads from the Miro API into the
// store) and a query command (read-only SQL over the store) on top of
// this skeleton. FTS is a separate bead that adds a virtual table over
// the items text fields once both sync and query are in place.
//
// Driver: modernc.org/sqlite — pure Go, no CGO, so the CLI binary stays
// cross-compilable without a C toolchain.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the on-disk schema version this binary writes. Open
// stamps it into PRAGMA user_version on fresh databases and refuses to
// open a database whose stamped version is higher (an older binary
// against a newer schema would silently misread the data).
const SchemaVersion = 1

// ErrSchemaTooNew is returned when the on-disk database was written by a
// newer binary that bumped SchemaVersion. The caller should refuse to
// proceed; a migration path can be added when the second version ships.
var ErrSchemaTooNew = errors.New("store: database schema is newer than this binary supports")

// Board is the lightweight projection of a Miro board into the local
// store. RawJSON is the verbatim API response — every other column is a
// denormalisation of fields useful for indexing. Sync writers populate
// both; readers either decode RawJSON or use the denormalised columns,
// depending on the query.
type Board struct {
	ID         string
	Name       string
	OwnerID    string
	ModifiedAt string // RFC3339; preserved verbatim from the API for round-tripping
	RawJSON    []byte
}

// Item is the lightweight projection of a board item. Position columns
// are denormalised from RawJSON so spatial queries (overlap, hit-test)
// don't need to parse every row.
type Item struct {
	ID         string
	BoardID    string
	Type       string
	PositionX  float64
	PositionY  float64
	ModifiedAt string
	RawJSON    []byte
}

// Store is a handle on an open SQLite database. Construct with Open;
// Close when done. Safe for concurrent use: reads run against WAL with
// no extra coordination, writes are serialised by writeMu so retried
// "database is locked" errors don't bubble up to callers.
type Store struct {
	db   *sql.DB
	path string

	writeMu sync.Mutex
}

// Open opens or creates the SQLite store at path and applies migrations.
// The directory is created with 0o755 if missing. Returns ErrSchemaTooNew
// if the on-disk schema version exceeds SchemaVersion — callers should
// surface that as a "please upgrade miro-cli" error rather than try to
// migrate downward.
//
// WAL mode + busy_timeout=5s gives readers and writers room to interleave
// without producing "database is locked" errors under normal use.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create db directory: %w", err)
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// One open conn is plenty for the CLI use case; the WAL still lets
	// the read-only handle (a separate sql.DB) run concurrent SELECTs.
	db.SetMaxOpenConns(1)
	db.SetConnMaxIdleTime(5 * time.Minute)

	s := &Store{db: db, path: path}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the on-disk path of the backing SQLite file.
func (s *Store) Path() string {
	return s.path
}

// DB returns the underlying *sql.DB. Callers must not call Close on it.
// Intended for ad-hoc read queries (the upcoming `miro query` command)
// that don't fit the typed Upsert/Get/List helpers; production write
// paths should go through the helpers so the writeMu serialisation
// stays honest.
func (s *Store) DB() *sql.DB {
	return s.db
}

// SchemaVersion reads PRAGMA user_version. A zero value means the
// database predates the version stamp; Open's migrate has stamped it
// since version 1, so any non-zero result reflects the schema the data
// was written under.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("store: read user_version: %w", err)
	}
	return v, nil
}

// DefaultPath returns the conventional on-disk path for the store. Prefers
// $XDG_DATA_HOME/miro-cli/store.db when XDG_DATA_HOME is set; otherwise
// $HOME/.local/share/miro-cli/store.db on Linux conventions, and falls
// back to ~/.miro/store.db when $HOME is unset (rare; CI sandboxes
// occasionally clear it). Callers can override this with --store-path
// once the sync command lands.
func DefaultPath() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "miro-cli", "store.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// UserHomeDir resolves XDG_CONFIG_HOME, HOME, USERPROFILE etc.
		// If all of those are empty we have nothing sensible to return.
		return "", fmt.Errorf("store: cannot resolve default store path: %w", err)
	}
	return filepath.Join(home, ".local", "share", "miro-cli", "store.db"), nil
}

// migrate applies migrations up to SchemaVersion. The migrations table
// pattern is overkill for v1; PRAGMA user_version + a switch is enough
// until a second migration earns the bookkeeping.
func (s *Store) migrate(ctx context.Context) error {
	var current int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("store: read user_version: %w", err)
	}
	if current > SchemaVersion {
		return fmt.Errorf("%w: on-disk version %d, binary supports %d", ErrSchemaTooNew, current, SchemaVersion)
	}
	if current == SchemaVersion {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range schemaV1 {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: migrate v1: %w", err)
		}
	}
	// PRAGMA user_version doesn't accept parameters and must be set as a
	// literal. SchemaVersion is a compile-time constant so the formatted
	// statement is safe.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
		return fmt.Errorf("store: stamp user_version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit migration: %w", err)
	}
	return nil
}

var schemaV1 = []string{
	`CREATE TABLE boards (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		owner_id TEXT NOT NULL DEFAULT '',
		modified_at TEXT NOT NULL DEFAULT '',
		raw_json TEXT NOT NULL,
		synced_at TEXT NOT NULL
	)`,
	`CREATE TABLE items (
		id TEXT PRIMARY KEY,
		board_id TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT '',
		position_x REAL NOT NULL DEFAULT 0,
		position_y REAL NOT NULL DEFAULT 0,
		modified_at TEXT NOT NULL DEFAULT '',
		raw_json TEXT NOT NULL,
		synced_at TEXT NOT NULL,
		FOREIGN KEY (board_id) REFERENCES boards(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX idx_items_board_id ON items(board_id)`,
	`CREATE INDEX idx_items_type ON items(type)`,
	`CREATE TABLE sync_metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
}

// UpsertBoard writes or replaces a single board row. RawJSON is required;
// the other fields can be empty (the schema defaults them).
func (s *Store) UpsertBoard(ctx context.Context, b Board) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return upsertBoard(ctx, s.db, b)
}

// UpsertBoards writes a batch of boards in a single transaction.
func (s *Store) UpsertBoards(ctx context.Context, boards []Board) error {
	if len(boards) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin upsert boards: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, b := range boards {
		if err := upsertBoard(ctx, tx, b); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit upsert boards: %w", err)
	}
	return nil
}

// execer is the subset of *sql.DB / *sql.Tx that upsert helpers depend
// on, so the same code services both the single-row and batched calls.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func upsertBoard(ctx context.Context, e execer, b Board) error {
	if b.ID == "" {
		return errors.New("store: board id is required")
	}
	if len(b.RawJSON) == 0 {
		return errors.New("store: board raw_json is required")
	}
	const q = `INSERT INTO boards (id, name, owner_id, modified_at, raw_json, synced_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			owner_id = excluded.owner_id,
			modified_at = excluded.modified_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at`
	if _, err := e.ExecContext(ctx, q,
		b.ID, b.Name, b.OwnerID, b.ModifiedAt, string(b.RawJSON), time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("store: upsert board %s: %w", b.ID, err)
	}
	return nil
}

// UpsertItem writes or replaces a single item row.
func (s *Store) UpsertItem(ctx context.Context, it Item) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return upsertItem(ctx, s.db, it)
}

// UpsertItems writes a batch of items in a single transaction.
func (s *Store) UpsertItems(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin upsert items: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, it := range items {
		if err := upsertItem(ctx, tx, it); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit upsert items: %w", err)
	}
	return nil
}

func upsertItem(ctx context.Context, e execer, it Item) error {
	if it.ID == "" {
		return errors.New("store: item id is required")
	}
	if it.BoardID == "" {
		return errors.New("store: item board_id is required")
	}
	if len(it.RawJSON) == 0 {
		return errors.New("store: item raw_json is required")
	}
	const q = `INSERT INTO items (id, board_id, type, position_x, position_y, modified_at, raw_json, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			board_id = excluded.board_id,
			type = excluded.type,
			position_x = excluded.position_x,
			position_y = excluded.position_y,
			modified_at = excluded.modified_at,
			raw_json = excluded.raw_json,
			synced_at = excluded.synced_at`
	if _, err := e.ExecContext(ctx, q,
		it.ID, it.BoardID, it.Type, it.PositionX, it.PositionY, it.ModifiedAt, string(it.RawJSON), time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("store: upsert item %s: %w", it.ID, err)
	}
	return nil
}

// GetBoard returns a board by id. Returns sql.ErrNoRows when the id is
// not present; callers can check with errors.Is(err, sql.ErrNoRows) and
// treat that as "not yet synced".
func (s *Store) GetBoard(ctx context.Context, id string) (Board, error) {
	const q = `SELECT id, name, owner_id, modified_at, raw_json FROM boards WHERE id = ?`
	var b Board
	var raw string
	err := s.db.QueryRowContext(ctx, q, id).Scan(&b.ID, &b.Name, &b.OwnerID, &b.ModifiedAt, &raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Board{}, err
		}
		return Board{}, fmt.Errorf("store: get board %s: %w", id, err)
	}
	b.RawJSON = []byte(raw)
	return b, nil
}

// ListBoards returns every board, ordered by id for stable test output.
func (s *Store) ListBoards(ctx context.Context) ([]Board, error) {
	const q = `SELECT id, name, owner_id, modified_at, raw_json FROM boards ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: list boards: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Board
	for rows.Next() {
		var b Board
		var raw string
		if err := rows.Scan(&b.ID, &b.Name, &b.OwnerID, &b.ModifiedAt, &raw); err != nil {
			return nil, fmt.Errorf("store: scan board row: %w", err)
		}
		b.RawJSON = []byte(raw)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate boards: %w", err)
	}
	return out, nil
}

// GetItem returns an item by id. Returns sql.ErrNoRows when missing.
func (s *Store) GetItem(ctx context.Context, id string) (Item, error) {
	const q = `SELECT id, board_id, type, position_x, position_y, modified_at, raw_json FROM items WHERE id = ?`
	var it Item
	var raw string
	err := s.db.QueryRowContext(ctx, q, id).Scan(&it.ID, &it.BoardID, &it.Type, &it.PositionX, &it.PositionY, &it.ModifiedAt, &raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, err
		}
		return Item{}, fmt.Errorf("store: get item %s: %w", id, err)
	}
	it.RawJSON = []byte(raw)
	return it, nil
}

// ListItemsByBoard returns every item on a board, ordered by id.
func (s *Store) ListItemsByBoard(ctx context.Context, boardID string) ([]Item, error) {
	const q = `SELECT id, board_id, type, position_x, position_y, modified_at, raw_json
		FROM items WHERE board_id = ? ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q, boardID)
	if err != nil {
		return nil, fmt.Errorf("store: list items by board %s: %w", boardID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []Item
	for rows.Next() {
		var it Item
		var raw string
		if err := rows.Scan(&it.ID, &it.BoardID, &it.Type, &it.PositionX, &it.PositionY, &it.ModifiedAt, &raw); err != nil {
			return nil, fmt.Errorf("store: scan item row: %w", err)
		}
		it.RawJSON = []byte(raw)
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate items: %w", err)
	}
	return out, nil
}

// SetSyncMetadata writes or replaces a key/value pair. The sync command
// uses this for per-resource cursors ("boards.last_sync") and any other
// out-of-band bookkeeping the schema doesn't model as a column.
func (s *Store) SetSyncMetadata(ctx context.Context, key, value string) error {
	if key == "" {
		return errors.New("store: sync metadata key is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	const q = `INSERT INTO sync_metadata (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	if _, err := s.db.ExecContext(ctx, q, key, value, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("store: set sync metadata %s: %w", key, err)
	}
	return nil
}

// GetSyncMetadata returns the value for key, or sql.ErrNoRows if unset.
func (s *Store) GetSyncMetadata(ctx context.Context, key string) (string, error) {
	const q = `SELECT value FROM sync_metadata WHERE key = ?`
	var v string
	err := s.db.QueryRowContext(ctx, q, key).Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		return "", fmt.Errorf("store: get sync metadata %s: %w", key, err)
	}
	return v, nil
}

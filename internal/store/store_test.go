package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// openTestDB returns a database backed by a real file in a temporary
// directory. Tests never mock SQLite: integration behaviour is the point.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Errorf("closing test database: %v", err)
		}
	})
	return st.DB()
}

func TestOpen_AppliesPragmas(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ctx := context.Background()

	var foreignKeys int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("reading foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("reading journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var busyTimeout int
	if err := db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("reading busy_timeout: %v", err)
	}
	if busyTimeout != busyTimeoutMillis {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, busyTimeoutMillis)
	}
}

func TestOpen_CreatesPrivateFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "test.sqlite")
	// The caller is expected to create the directory; mirror that here.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("database permissions = %04o, want 0600", perm)
	}
}

func TestOpen_PathWithSpaces(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "data dir with spaces")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	st, err := Open(context.Background(), filepath.Join(dir, "test.sqlite"))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.DB().Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY) STRICT`); err != nil {
		t.Fatalf("creating table: %v", err)
	}
}

func TestOpen_EnforcesForeignKeys(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		CREATE TABLE parents (id INTEGER PRIMARY KEY) STRICT;
		CREATE TABLE children (
			id        INTEGER PRIMARY KEY,
			parent_id INTEGER NOT NULL REFERENCES parents (id)
		) STRICT;
	`)
	if err != nil {
		t.Fatalf("creating tables: %v", err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO children (id, parent_id) VALUES (1, 42)`)
	if err == nil {
		t.Fatal("inserting an orphan row succeeded, want a foreign key violation")
	}
}

func TestOpen_RejectsEmptyPath(t *testing.T) {
	t.Parallel()

	_, err := Open(context.Background(), "   ")
	if err == nil {
		t.Fatal("Open(empty) error = nil, want an error")
	}
}

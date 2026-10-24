// Package store owns the SQLite database: connection setup, the pragmas the
// project relies on, and the forward-only migration runner.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// busyTimeoutMillis is how long SQLite waits for a lock before returning
// SQLITE_BUSY. One user means contention is rare; this only smooths bursts.
const busyTimeoutMillis = 5000

// Open opens (creating if needed) the SQLite database at path and verifies
// that it is reachable. The file is created with mode 0600 (data directory
// permissions are the caller's responsibility). The returned *sql.DB is safe
// for concurrent use; its pool is capped at one connection on purpose:
// SQLite serializes writes anyway, and a single connection removes lock
// contention inside the process.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("store: empty database path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving database path: %w", err)
	}
	// Create the file ourselves so it is never world-readable. The path comes
	// from validated configuration, never from a request.
	// #nosec G304 -- opening the configured database path is the point.
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("creating database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("closing database file: %w", err)
	}

	db, err := sql.Open("sqlite", dsn(abs))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if pingErr := db.PingContext(ctx); pingErr != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("pinging database: %w (and closing it: %w)", pingErr, closeErr)
		}
		return nil, fmt.Errorf("pinging database: %w", pingErr)
	}
	return db, nil
}

// dsn builds a SQLite connection string that applies the project's pragmas
// on every new connection. They cannot be set once globally because
// foreign_keys and busy_timeout are per-connection settings.
func dsn(absPath string) string {
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	u := url.URL{Scheme: "file", Path: absPath, RawQuery: q.Encode()}
	return u.String()
}

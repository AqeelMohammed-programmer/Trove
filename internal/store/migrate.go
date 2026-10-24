package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// migrationFilePattern names the only accepted migration file form:
// four digits, an underscore, a snake_case description, and .sql.
var migrationFilePattern = regexp.MustCompile(`^(\d{4})_([a-z0-9]+(?:_[a-z0-9]+)*)\.sql$`)

// Migrator applies forward-only SQL migrations embedded in a filesystem.
type Migrator struct {
	// DB is the database to migrate. Required.
	DB *sql.DB
	// FS holds the migration files. Required.
	FS fs.FS
	// Dir is the directory inside FS that holds the files. Use "." for the
	// root of FS; an empty Dir means the same.
	Dir string
	// Now supplies the recorded application time. Nil means time.Now.
	// Tests inject a fixed clock for deterministic rows.
	Now func() time.Time
}

type migration struct {
	version int
	name    string
	file    string
}

// Up applies every migration that has not been applied yet, in ascending
// version order. Each migration and its bookkeeping row commit together, so
// a failure leaves the database at the last fully applied version. Up is
// idempotent: running it twice applies nothing the second time.
func (m *Migrator) Up(ctx context.Context) error {
	if m.DB == nil {
		return errors.New("store: migrator has no database")
	}
	if m.FS == nil {
		return errors.New("store: migrator has no filesystem")
	}
	dir := m.Dir
	if dir == "" {
		dir = "."
	}
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	migrations, err := readMigrations(m.FS, dir)
	if err != nil {
		return err
	}
	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return err
	}

	for _, mig := range migrations {
		if _, ok := applied[mig.version]; ok {
			continue
		}
		if err := m.apply(ctx, mig); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	const q = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT    NOT NULL,
		applied_at TEXT    NOT NULL
	) STRICT`
	if _, err := m.DB.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}
	return nil
}

func (m *Migrator) appliedVersions(ctx context.Context) (map[int]struct{}, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query; close error adds nothing

	applied := make(map[int]struct{})
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scanning applied migration: %w", err)
		}
		applied[v] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating applied migrations: %w", err)
	}
	return applied, nil
}

func (m *Migrator) apply(ctx context.Context, mig migration) error {
	body, err := fs.ReadFile(m.FS, path.Join(m.Dir, mig.file))
	if err != nil {
		return fmt.Errorf("reading migration %s: %w", mig.file, err)
	}
	now := time.Now
	if m.Now != nil {
		now = m.Now
	}

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting migration %s: %w", mig.file, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after a successful commit

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("applying migration %s: %w", mig.file, err)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		mig.version, mig.name, now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("recording migration %s: %w", mig.file, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %s: %w", mig.file, err)
	}
	return nil
}

// readMigrations lists, validates and sorts the migration files in dir.
func readMigrations(efs fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(efs, dir)
	if err != nil {
		return nil, fmt.Errorf("listing migrations in %q: %w", dir, err)
	}

	migrations := make([]migration, 0, len(entries))
	seen := make(map[int]string, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		match := migrationFilePattern.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf("migration %q does not match NNNN_snake_case_name.sql", name)
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			// Unreachable: the pattern only matches digits.
			return nil, fmt.Errorf("parsing version of %q: %w", name, err)
		}
		if other, ok := seen[version]; ok {
			return nil, fmt.Errorf("migrations %q and %q share version %04d", other, name, version)
		}
		seen[version] = name
		migrations = append(migrations, migration{
			version: version,
			name:    match[2],
			file:    name,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

package store

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"
	"time"
)

func mapFS(files map[string]string) fstest.MapFS {
	fsys := make(fstest.MapFS, len(files))
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

func TestMigrator_AppliesInOrderAndIsIdempotent(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	fixed := time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)
	m := &Migrator{
		DB: db,
		FS: mapFS(map[string]string{
			// Two statements in one file: proves multi-statement scripts run
			// as a whole.
			"m/0002_add_index.sql": `
				CREATE INDEX widgets_name ON widgets (name);
				INSERT INTO widgets (name) VALUES ('second');
			`,
			"m/0001_create_widgets.sql": `
				CREATE TABLE widgets (
					id   INTEGER PRIMARY KEY,
					name TEXT NOT NULL
				) STRICT;
				INSERT INTO widgets (name) VALUES ('first');
			`,
			"m/README.md": "ignored",
		}),
		Dir: "m",
		Now: func() time.Time { return fixed },
	}

	ctx := context.Background()
	if err := m.Up(ctx); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("counting widgets: %v", err)
	}
	if count != 2 {
		t.Errorf("widgets = %d, want 2 (both migration files must run)", count)
	}

	rows, err := db.QueryContext(ctx, `SELECT version, name, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("reading schema_migrations: %v", err)
	}
	t.Cleanup(func() { _ = rows.Close() })

	var versions []int
	for rows.Next() {
		var (
			version   int
			name      string
			appliedAt string
		)
		if err := rows.Scan(&version, &name, &appliedAt); err != nil {
			t.Fatalf("scanning schema_migrations: %v", err)
		}
		if appliedAt != "2026-10-24T09:30:00Z" {
			t.Errorf("version %d applied_at = %q, want the injected clock value", version, appliedAt)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating schema_migrations: %v", err)
	}
	if len(versions) != 2 || versions[0] != 1 || versions[1] != 2 {
		t.Errorf("applied versions = %v, want [1 2]", versions)
	}

	// Running again must not re-apply or fail.
	if err := m.Up(ctx); err != nil {
		t.Fatalf("second Up(): %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("counting widgets after second Up: %v", err)
	}
	if count != 2 {
		t.Errorf("widgets after second Up = %d, want 2", count)
	}
}

func TestMigrator_RollsBackFailedMigration(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	m := &Migrator{
		DB: db,
		FS: mapFS(map[string]string{
			"m/0001_widgets.sql": `CREATE TABLE widgets (id INTEGER PRIMARY KEY) STRICT;`,
			"m/0002_broken.sql": `
				CREATE TABLE gadgets (id INTEGER PRIMARY KEY) STRICT;
				INSERT INTO missing_table (id) VALUES (1);
			`,
		}),
		Dir: "m",
	}

	ctx := context.Background()
	if err := m.Up(ctx); err == nil {
		t.Fatal("Up() error = nil, want the broken migration to fail")
	}

	var applied int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("counting schema_migrations: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied migrations = %d, want 1", applied)
	}

	// The partial work of the failed migration must have been rolled back.
	var gadgets sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'gadgets'`).Scan(&gadgets)
	if err == nil {
		t.Fatal("table gadgets survived the rolled-back migration")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("looking up gadgets: %v", err)
	}
}

func TestMigrator_RejectsMalformedNames(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"too short":    "m/1_oops.sql",
		"camel case":   "m/0001_CreateWidgets.sql",
		"missing desc": "m/0001.sql",
		"hyphen":       "m/0001-create-widgets.sql",
		"duplicate":    "m/0001_other.sql",
	}
	for name, file := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			files := map[string]string{"m/0001_widgets.sql": "SELECT 1;"}
			files[file] = "SELECT 1;"
			m := &Migrator{DB: openTestDB(t), FS: mapFS(files), Dir: "m"}
			if err := m.Up(context.Background()); err == nil {
				t.Fatalf("Up() error = nil for %q, want a validation error", file)
			}
		})
	}
}

func TestMigrator_RequiresDatabaseAndFilesystem(t *testing.T) {
	t.Parallel()

	if err := (&Migrator{FS: mapFS(nil)}).Up(context.Background()); err == nil {
		t.Error("Up() without DB error = nil, want an error")
	}
	if err := (&Migrator{DB: openTestDB(t)}).Up(context.Background()); err == nil {
		t.Error("Up() without FS error = nil, want an error")
	}
}

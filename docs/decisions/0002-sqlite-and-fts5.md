# 0002. SQLite with FTS5, using the pure-Go driver

**Status:** Accepted
**Date:** 2026-10-24

## Context

Trove stores one user's collection for years: items, tags, attachments and
full-text search over title, description, note and tags (`S-1`). The
implementation must ship as one binary without a C compiler, and the search
requirement (`S-2`, `S-3`, `S-6`) is best served by SQLite's FTS5 extension.

The project's own rule (`AGENTS.md` section 3) requires proving FTS5 support
before the driver choice is final.

## Decision

Use **SQLite** through [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)
**v1.61.0**, a pure-Go translation of SQLite, with `database/sql`.

Connection settings, applied in the DSN so every pooled connection gets them:

- `foreign_keys(1)`
- `journal_mode(WAL)`
- `busy_timeout(5000)`

The connection pool is capped at one connection. SQLite serializes writes
anyway, and a single connection removes in-process lock contention; one user
does not need concurrent database readers.

### FTS5 proof

`TestFTS5_AvailableAndUsable` (in `internal/store/fts5_test.go`) creates an
FTS5 table with the `unicode61` tokenizer and verifies, against a real
temporary database:

- prefix matching (`"own"*` matches "Ownership"),
- case-insensitive matching (`"BORROW"*` matches "borrow"),
- multi-word queries requiring every token,
- that a bare token does not match a longer word (prefix must be explicit),
- `bm25` ranking with per-column weights putting a title match first.

The test passes on Go 1.26.8 / linux-amd64 as of 2026-10-24. FTS5 is therefore
supported, and no fallback driver is needed.

## Consequences

- No CGO: `go build` works with the default toolchain and cross-compiles
  easily.
- The database is a single file, so backup is a file copy (see
  [operations](../guides/operations.md)).
- FTS5 is used directly; the query builder must quote user tokens (see
  [search](../architecture/search.md)).
- `modernc.org/sqlite` is a large dependency (BSD-3-Clause) with a sizeable
  indirect tree; this is accepted because it removes the C toolchain.
- SQLite's single-writer model is fine for one user but must be respected:
  long transactions are avoided, especially during file streaming.

## Alternatives considered

- **`github.com/mattn/go-sqlite3`**: mature, but requires CGO and a C compiler,
  which breaks the single-binary, cross-compile-friendly build.
- **PostgreSQL**: stronger concurrency and indexing, but adds a server to
  install, configure and back up for a one-user application; the spec's scale
  does not justify it.
- **A pure-Go embedded key-value store plus a hand-written index**: more code,
  no FTS5, and a much higher chance of subtle search bugs.

# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Adopted the behaviour specification as `docs/spec.md` with status **Accepted**, and
  resolved the product name as **Trove**.
- `AGENTS.md`, the guide for humans and AI agents working on the project.
- M0 skeleton: one Go binary that loads validated configuration, creates a
  private data directory, opens SQLite with foreign keys and WAL, applies
  embedded migrations in transactions, logs structured JSON, and serves
  `GET /healthz` with strict security headers and graceful shutdown.
- FTS5 proof test for the pure-Go `modernc.org/sqlite` driver, covering prefix
  matching, case folding and bm25 ranking.
- Test suite for configuration, SQLite pragmas and permissions, the migration
  runner, and the HTTP surface, all run with `-race`.
- `Makefile` targets (`run`, `fmt`, `lint`, `test`, `build`, `check`, `bench`,
  `tools`) with pinned `goimports` and `golangci-lint` versions.
- `golangci.yml`, the documentation link checker, and the initial wiki pages
  (architecture, guides, traceability, open questions).
- ADRs 0001–0005: Go, SQLite/FTS5, server-rendered HTML with HTMX, password
  reset as a CLI subcommand, and the export/import archive.

### Added

- Authentication (M1): first-run password setup, login, logout, change
  password, Argon2id hashing with stored parameters, server-side sessions
  (hashed tokens, `HttpOnly`, `SameSite=Lax`, browser-session cookie), CSRF
  tokens plus Origin checks, login throttling, and `reset-password` as a local
  CLI subcommand.
- Authentication tests, including a route-enumeration test proving every
  non-public route rejects unauthenticated requests, and a real-SQLite
  integration suite for setup, login, throttling, logout and password change.

### Changed

- Specification section 8.1: removed the resolved "Product name" decision and
  renumbered the remaining open decisions.
- `AGENTS.md` no longer assumes a specific hosting or CI service; `make check`
  is the single, portable gate.

# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Redesigned the interface around a role-based design system: neutral paper,
  one accent (teal) used only for links and the primary action, a fixed type
  and spacing scale, serif headings and note prose, hairline separators instead
  of boxes, and inline SVG icons instead of emoji. All colour pairs are
  verified against WCAG 2.2 AA and the choices are recorded in
  [`docs/architecture/ui.md`](docs/architecture/ui.md).

### Added

- A light/dark/system theme switch in the top bar, remembered in the browser.
  It is a UI preference outside the specification; `theme.js` applies it before
  paint to avoid a flash, and both scripts stay external to respect the CSP.

### Fixed

- Tag autocomplete on the add/edit form now suggests the tag being typed, not
  only the first one. The previous `<datalist>` matched the whole
  comma-separated field, so suggestions stopped as soon as a comma appeared and
  choosing one replaced every tag already entered. The field is now enhanced by
  a small accessible combobox (`role="combobox"` with a `listbox` popup) that
  asks `GET /tags/suggest` for active tags matching the current token (`T-2`),
  and inserts the chosen name without touching the others. The datalist stays as
  a no-JavaScript fallback.

## [1.0.0] - 2026-10-24

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

- Authentication (M1): first-run password setup, login, logout, change
  password, Argon2id hashing with stored parameters, server-side sessions
  (hashed tokens, `HttpOnly`, `SameSite=Lax`, browser-session cookie), CSRF
  tokens plus Origin checks, login throttling, and `reset-password` as a local
  CLI subcommand.
- Authentication tests, including a route-enumeration test proving every
  non-public route rejects unauthenticated requests, and a real-SQLite
  integration suite for setup, login, throttling, logout and password change.

- Items and tags (M2): a single add/edit form (`I-1`, `I-6`), title defaulting
  to the first attachment (`I-2`), tag parsing, case-insensitive reuse and
  automatic removal of unused tags (`T-1`–`T-6`), favorites (`F-1`–`F-3`),
  browsing with sort orders, filters, tag chips and pagination (`B-1`–`B-7`),
  and an item detail view.
- Markdown notes rendered with goldmark and sanitized with bluemonday: raw HTML
  is never rendered (`V-2`), image syntax becomes a link (`V-3`), and links open
  in a new tab (`V-4`).
- Vendored HTMX 2.0.4 (MIT) for partial updates, configured for the strict
  Content-Security-Policy; every action still works without JavaScript.

- Attachments (M3): link attachments with `http`/`https` validation (`A-2`),
  and file uploads streamed to disk while a SHA-256 is computed, stored under
  the attachment id and never by file name (`A-3`, `A-4`, `A-6`, `A-8`).
  Several files can be added at once, and kinds mix on one item (`A-1`, `A-5`).
- Attachment display and actions: links open in a new tab and can be copied;
  files download under their original name with `Content-Disposition:
  attachment` and `nosniff` (`A-7`).
- A streamed multipart add/edit form and the `/attachments/{id}/download`
  endpoint.

- Search (M4): an FTS5 index over title, description, note and tags (`S-1`),
  word-prefix matching with all words required and case-insensitive comparison
  (`S-2`), relevance ranking with title matches first (`S-3`), a search box,
  and combination with every existing filter (`S-5`). The index is written in
  the same transaction as the item change (`S-6`), and user input is always
  treated as literal text, never as FTS syntax.

- Trash (M5): deleting an item or removing one attachment moves it to the
  trash without confirmation (`I-7`, `A-9`, `X-1`, `X-2`, `X-13`). The trash
  view lists trashed items with all their attachments and individually trashed
  attachments with their owning item (`X-3`). Items and attachments restore
  with the right attachment states (`X-4`, `X-5`). Permanent deletion of an
  item, an attachment, or the whole trash requires confirmation and removes
  stored file content only then (`X-6`, `X-7`, `X-8`, `X-9`). Identical-content
  attachments are independent (`X-10`), trashed items keep their tags (`X-11`),
  and orphaned content is cleaned up at startup (`X-12`).

- Duplicate handling (M6): the item detail view warns when an attachment matches
  another by exact URL or the same SHA-256, including matches inside the same
  item and trashed ones, and offers to open the matching item or restore a
  trashed one (`D-1`..`D-5`). A read-only **Duplicates** screen groups matching
  attachments by normalized URL (fragment and one trailing slash removed) and by
  SHA-256, marks trashed entries and links each to its item (`R-1`..`R-6`).

- Tag management (M7): a Tags screen lists tags shown under `T-6` with active
  item counts, alphabetical by default and sortable by count (`M-1`). Rename
  changes the spelling everywhere and refuses a clash with another tag,
  suggesting a merge, while changing only capitalization is allowed (`M-2`).
  Merge moves every item's source tag to the target without duplicates and
  removes the source (`M-3`). Delete removes a tag from every item after showing
  how many are affected (`M-4`). None of these change `updated_at` (`M-5`); the
  search index is refreshed so renamed or merged tags stay searchable.

- Export and import (M8): the whole repository exports as one ZIP archive
  holding `data.json` and a `files/` directory, with no password, hash or
  session data (`E-1`..`E-4`). Import is restore-only into an empty repository
  (`E-5`) and validates the entire archive first: recognised format and
  version, well-formed metadata, every referenced file present, and matching
  SHA-256 for each file, with zip-slip protection (`E-6`). It is all-or-nothing
  (`E-7`), preserves identifiers, timestamps, trash states, favorites, tags,
  labels and content (`E-8`), leaves the password alone (`E-9`), and a full
  export-then-import round trip reproduces the repository (`E-10`).

- Hardening (M9): a 100,000-item benchmark suite (`make bench`) measuring list,
  deep paging, tag filtering and single/multi-word search, all well under the
  one-second target (`N-2`), with the numbers and the documented limits (`N-1`)
  recorded in the performance page.
- A multi-stage `Dockerfile` (static binary on a distroless, non-root image) and
  a `docker-compose.yml` example; both verified by building and running the
  image.

### Changed

- Specification section 8.1: removed the resolved "Product name" decision and
  renumbered the remaining open decisions.
- `AGENTS.md` no longer assumes a specific hosting or CI service; `make check`
  is the single, portable gate.

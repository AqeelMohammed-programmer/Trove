# Data model

This page documents the database schema, its indexes and its migration
history. The fields themselves come from [`../spec.md`](../spec.md) section 2;
this page explains how they are stored.

## Current schema (through M2)

Applied by numbered, forward-only migrations embedded in the binary. The
milestone that first needs a table introduces it in its own migration.

### `schema_migrations`

Created by `internal/store.Migrator` before the first migration is applied.

| Column | Type | Notes |
|---|---|---|
| `version` | `INTEGER PRIMARY KEY` | Four-digit number from the file name, for example `1` for `0001_...sql`. |
| `name` | `TEXT NOT NULL` | The snake_case part of the file name, without the number and extension. |
| `applied_at` | `TEXT NOT NULL` | UTC ISO 8601 timestamp, from the injected clock. |

The table is `STRICT`. Each row is written in the same transaction as the
migration it records, so a failure never leaves a half-applied version.

### `settings`, `sessions`, `login_throttle` (migration `0001_auth`)

| Table | Purpose |
|---|---|
| `settings` | Key/value state. The only product key is `password_hash`, an Argon2id PHC string (`U-8`). |
| `sessions` | Server-side sessions: `token_hash` (primary key, SHA-256 of the cookie token), `csrf_token`, `created_at`. The raw token is never stored (`U-5`). |
| `login_throttle` | Per-client failure count and `blocked_until` (`U-9`). |

### `items`, `tags`, `item_tags`, `attachments` (migration `0002_items`)

| Table | Purpose |
|---|---|
| `items` | Item fields from spec 2.1, including `favorite` and `trashed_at`. |
| `tags` | `name` keeps the first-used spelling; `name_folded` (lower-case) is `UNIQUE`, enforcing case-insensitive identity (`T-4`). |
| `item_tags` | Join table; primary key `(item_id, tag_id)` enforces the set rule (`T-5`). Both sides `ON DELETE CASCADE`. |
| `attachments` | Common fields plus link/file columns and `state`. Created here so the list's kind filters have a stable schema, even though the attachment UI arrives in M3. A `CHECK` enforces the per-kind required columns. |

`attachments` links to `items(id)` with `ON DELETE CASCADE`; permanent deletion
removes rows, while trashing only changes `state` (`X-9`).

### `items_fts` (migration `0003_search`)

A standalone FTS5 table over `title`, `description`, `note` and `tags`,
sharing the `items` rowid. It is written in the same transaction as the item
change (`S-6`). See the [search page](search.md).

Design rules that apply to every table:

- `STRICT` tables; timestamps stored as UTC ISO 8601 text.
- Identifiers are 128-bit random values, hex-encoded, and never reused.
- Foreign keys are on and declared with explicit `ON DELETE` behaviour.
- Writes that touch more than one row happen in a single transaction, together
  with the FTS index update (`S-6`).
- The normalized URL used by the duplicate report (`R-2`) is computed at write
  time and stored in a column so the report stays fast.

## Indexes

Indexes are added in the migration that needs them, not speculatively. Present
now:

| Index | Query it serves |
|---|---|
| `items_active_created (trashed_at, created_at DESC)` | Default list (`B-1`, `B-6`). |
| `items_active_updated (trashed_at, updated_at DESC)` | Recently edited sort (`B-2`). |
| `items_active_title (trashed_at, title COLLATE NOCASE)` | Title sort (`B-2`). |
| `item_tags(tag_id, item_id)` | Tag filter and tag counts (`B-4`, `T-6`). |
| `attachments(item_id, created_at)` | Attachment lists ordered by time added (`A-1`). |
| `attachments(sha256)` | Duplicate lookups (`D-2`, `R-2`). |
| `attachments(normalized_url)` | Duplicate lookups (`D-1`, `R-2`). |
| `attachments(state, trashed_at)` | Trash listing (`X-3`). |

A favorites index is deliberately not added yet: one user with a personal
collection does not need it, and the performance check at M9 will decide. A
trash index for the attachment-kind filters is likewise deferred to M3.

These will be validated with the performance check at M9; `make bench` exists
for that from M0 onwards.

## Migration history

| Version | File | Change | Date |
|---|---|---|---|
| 1 | `0001_auth.sql` | `settings`, `sessions`, `login_throttle` (authentication). | 2026-10-24 |
| 2 | `0002_items.sql` | `items`, `tags`, `item_tags`, `attachments` plus list/filter indexes. | 2026-10-24 |
| 3 | `0003_search.sql` | `items_fts` FTS5 index over title, description, note and tags. | 2026-10-24 |

## Related pages

- [Storage](storage.md) — where file content lives.
- [Search](search.md) — FTS5 index design.
- [Overview](overview.md) — package responsibilities.

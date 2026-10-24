# Data model

This page documents the database schema, its indexes and its migration
history. The fields themselves come from [`../spec.md`](../spec.md) section 2;
this page explains how they are stored.

## Current schema (M0)

Only the migration bookkeeping table exists. Domain tables are introduced by
the milestone that first needs them, each in its own numbered migration.

### `schema_migrations`

Created by `internal/store.Migrator` before the first migration is applied.

| Column | Type | Notes |
|---|---|---|
| `version` | `INTEGER PRIMARY KEY` | Four-digit number from the file name, for example `1` for `0001_...sql`. |
| `name` | `TEXT NOT NULL` | The snake_case part of the file name, without the number and extension. |
| `applied_at` | `TEXT NOT NULL` | UTC ISO 8601 timestamp, from the injected clock. |

The table is `STRICT`. Each row is written in the same transaction as the
migration it records, so a failure never leaves a half-applied version.

## Planned tables

These are sketches, not commitments: the milestone that implements them may
adjust columns as long as it documents the change here. Anything listed under
"Out of scope" in the spec never gets a table.

| Table | Purpose | Introduced |
|---|---|---|
| `settings` | First-run state and the Argon2id password hash with its parameters. | M1 |
| `sessions` | Server-side sessions with hashed tokens and an optional end time. | M1 |
| `login_attempts` | Consecutive failure count and block expiry for throttling (`U-9`). | M1 |
| `items` | Item fields from spec 2.1, including `trashed_at`. | M2 |
| `tags` | Tag name with its first-used spelling (`T-4`). | M2 |
| `item_tags` | Join table; primary key on `(item_id, tag_id)` enforces `T-5`. | M2 |
| `attachments` | Common fields plus link/file columns and `state` from spec 2.2. | M3 |
| `attachments_fts` | FTS5 index over title, description, note and tags (`S-1`). | M4 |

Design rules that apply to every table:

- `STRICT` tables; timestamps stored as UTC ISO 8601 text.
- Identifiers are 128-bit random values, hex-encoded, and never reused.
- Foreign keys are on and declared with explicit `ON DELETE` behaviour.
- Writes that touch more than one row happen in a single transaction, together
  with the FTS index update (`S-6`).
- The normalized URL used by the duplicate report (`R-2`) is computed at write
  time and stored in a column so the report stays fast.

## Indexes

Indexes are added in the migration that needs them, not speculatively. The
planned minimum:

| Index | Query it serves |
|---|---|
| `items(trashed_at, created_at DESC)` | Default list (`B-1`, `B-6`). |
| `items(trashed_at, updated_at DESC)` | Recently edited sort (`B-2`). |
| `items(trashed_at, favorite, created_at DESC)` | Favorites view (`F-2`). |
| `item_tags(tag_id)` | Tag filter and tag counts (`B-4`, `T-6`). |
| `attachments(item_id, state)` | Attachment lists and kind filters (`B-5`). |
| `attachments(sha256)` | Duplicate lookups (`D-2`, `R-2`). |
| `attachments(normalized_url)` | Duplicate lookups (`D-1`, `R-2`). |
| `attachments(state, trashed_at)` | Trash listing (`X-3`). |

These will be validated with the performance check at M9; `make bench` exists
for that from M0 onwards.

## Migration history

| Version | File | Change | Date |
|---|---|---|---|
| — | — | No numbered migration yet. The runner and `schema_migrations` exist; M1 adds the first migration. | 2026-10-24 |

## Related pages

- [Storage](storage.md) — where file content lives.
- [Search](search.md) — FTS5 index design.
- [Overview](overview.md) — package responsibilities.

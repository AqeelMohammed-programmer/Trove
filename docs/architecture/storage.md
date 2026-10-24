# Storage

This page describes the on-disk layout of a Trove instance: what lives in the
data directory, how permissions are set, and how copied files are written so a
crash cannot leave half a file behind.

## Layout

```text
<data-dir>/                    0700
├── repository.sqlite          0600  metadata (items, tags, attachments, FTS)
├── repository.sqlite-wal      0600  WAL file, present while running
├── repository.sqlite-shm      0600  shared-memory file, present while running
└── files/                     0700  copied file content (from M3)
    └── <attachment-id>        0600  exact bytes, no extension
```

- File names on disk are **attachment ids**, never user-supplied names. This
  removes path traversal as a class of bug (`AGENTS.md` section 6).
- The original file name is metadata only; it is used for display and for the
  `Content-Disposition` header on download.
- A file is removed only on permanent deletion (`X-9`), never when it is
  merely moved to the trash.

## Permissions

| Object | Mode | Set by |
|---|---|---|
| Data directory | `0700` | `cmd/server` at startup (`os.MkdirAll`) |
| Database and WAL files | `0600` | `internal/store.Open` creates the file; SQLite inherits it |
| Copied files | `0600` | The blob writer (M3) |
| Temporary upload files | `0600` | The blob writer, in the same directory as the final file |

`MkdirAll` does not tighten an existing directory. Operators should create the
data directory themselves with restrictive permissions if it already exists
with looser ones; the operations guide covers this.

## Writing a copied file (M3)

The contract, to be implemented in `internal/blob`:

1. Create a temporary file in `files/` (same filesystem, so the final rename is
   atomic).
2. Stream the request body into it through `io.MultiWriter` with a SHA-256
   hasher. The upload is never loaded fully into memory (spec `A-4`, `N-1`).
3. `fsync` the temporary file, close it, and rename it to `files/<id>`.
4. Commit the database row in the same request. If the transaction fails,
   remove the just-written file and log any removal failure.
5. On permanent deletion, commit the database change first, then delete the
   file. A failed delete is logged and retried by the orphan cleanup.

The cleanup routine (M5/M9) lists `files/`, compares the names with the
attachment ids still present in any state, and removes unreferenced entries
older than a safety window. It never touches anything outside the data
directory (`X-12`).

## Downloads

Downloads are served from the data directory after an authenticated request
(`U-3`), with:

- `Content-Disposition: attachment` and an encoded original file name.
- `X-Content-Type-Options: nosniff`.
- The stored media type when known, otherwise `application/octet-stream`.

This prevents an uploaded HTML or SVG file from ever executing in the
application's origin.

## Backups

Backing up the data directory while the server is stopped is always safe.
Online backups should use SQLite's backup API or `VACUUM INTO`, because copying
a live WAL database can produce a torn snapshot. The
[operations guide](../guides/operations.md) gives the commands.

## Current state (M0)

Only `repository.sqlite` is created. There are no copied files yet, so
`files/` and the cleanup routine do not exist.

## Related pages

- [Data model](data-model.md)
- [Security](security.md)
- [Operations guide](../guides/operations.md)

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
└── files/                     0700  copied file content
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
| Copied files | `0600` | `internal/blob` |
| Temporary upload files | `0600` | `internal/blob`, in the same directory as the final file |

`MkdirAll` does not tighten an existing directory. Operators should create the
data directory themselves with restrictive permissions if it already exists
with looser ones; the operations guide covers this.

## Writing a copied file

Implemented in `internal/blob`:

1. Create a temporary file in `files/` (same filesystem, so the final rename is
   atomic).
2. Stream the request body into it through `io.MultiWriter` with a SHA-256
   hasher. The upload is never loaded fully into memory (spec `A-4`, `N-1`).
3. Set `0600`, close it, and rename it to `files/<id>`. A missing or interrupted
   save leaves the destination absent rather than partial.
4. Commit the database row in the same request. If the transaction fails, the
   service removes the files it just wrote, so a rejected save leaves no
   orphaned content. A failed removal is logged and picked up by the cleanup
   routine.
5. On permanent deletion, commit the database change first, then delete the
   file (`X-9`; the delete is idempotent). A failed delete is logged and retried
   by the orphan cleanup.

`internal/blob` accepts only the 32-character lowercase hex ids the application
generates, so a user-supplied file name is never part of a path. Temporary
uploads are named `.upload-*` and are ignored by the listing used for cleanup.

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

## Current state (M3)

`repository.sqlite` and `files/` are both created at startup. File uploads are
streamed, hashed and stored under `files/<id>`; downloads serve them with safe
headers. The orphan cleanup routine is still to come (M5/M9).

## Related pages

- [Data model](data-model.md)
- [Security](security.md)
- [Operations guide](../guides/operations.md)

# 0005. Export and import as a validated ZIP archive

**Status:** Accepted
**Date:** 2026-10-24

## Context

The user must be able to export the whole repository as one archive and import
it into a fresh instance, reproducing everything visible (`E-1`–`E-10`). Import
is restore-only, must validate the entire archive before changing anything, and
must be all-or-nothing. The format must stay readable years from now and carry
trashed state, attachments and file content.

## Decision

Use a **ZIP** archive with this layout, exactly as specified:

```text
data.json                 metadata for all items, tags and attachments
files/<attachment-id>     exact bytes of every copied file, no extension
```

- `format` is `personal-repository-export`, `format_version` is `1`.
- Timestamps are UTC ISO 8601; identifiers and trash states are preserved
  (`E-8`).
- The password hash and sessions are never included (`E-3`).
- Export reads a consistent snapshot in one database transaction (`E-4`).
- Import validates before writing (`E-6`): recognized format and version,
  well-formed JSON, every referenced file present, SHA-256 of each file equal
  to the recorded value, and no unexpected paths.
- Import then writes into a staging directory and a single transaction; on any
  failure the transaction rolls back and the staging directory is removed, so
  the repository stays empty (`E-7`).
- Import refuses to run unless the repository is empty (`E-5`).
- Unknown JSON fields are ignored so a newer export can still be read
  (forward compatibility, as the spec suggests).

The serializer is Go's `encoding/json`; the archive writer is `archive/zip`.
Both are standard library.

## Consequences

- One file to copy to cold storage; file content is byte-exact.
- Validation is done twice in effect: once by the importer and once by the
  SHA-256 check, which catches truncation and tampering.
- The round-trip test (`E-10`) compares exported and re-imported data field by
  field, including trash state and tags.
- Large archives stream rather than load fully into memory; the import uses a
  temporary directory (documented in the storage architecture page).
- No encryption: the archive contains all data in the clear and must be stored
  accordingly. Adding encryption would be a new decision.

## Alternatives considered

- **Copying the SQLite file**: smaller and simpler, but exposes the schema as
  the public format, cannot be read without SQLite, and makes forward
  compatibility and validation harder.
- **A tar archive**: equivalent capability, but ZIP is more universally
  readable and `archive/zip` avoids a dependency.
- **JSON only, with base64 files**: bloats the metadata file, breaks streaming
  and makes partial validation awkward.
- **Merge import**: explicitly out of scope for this version (`E-5`); it needs
  conflict resolution rules the spec does not define.

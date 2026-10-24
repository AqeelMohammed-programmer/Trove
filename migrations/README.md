# Migrations

Forward-only SQL migrations, applied automatically at startup by
`internal/store.Migrator`. This folder is embedded in the binary.

## Rules

- One file per change, named `NNNN_snake_case_name.sql`, starting at `0001`.
- Never edit or rename a migration that has been merged. Add a new one.
- Each file runs in its own transaction together with its
  `schema_migrations` row, so a failure leaves the previous version intact.
- Every schema change must update `docs/architecture/data-model.md` in the
  same commit.

There are no migrations yet. M0 delivers the runner; the first real migration
comes with the authentication milestone.

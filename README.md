# Trove

Trove is a private, single-user repository for the things you want to find
again years later: notes, links and files, each stored as an **item** with a
title, description, Markdown note, tags and any number of attachments. It runs
as one self-contained web application on a trusted local network, protected by
one password.

The behaviour is defined by [`docs/spec.md`](docs/spec.md). The development
guide is [`AGENTS.md`](AGENTS.md).

**Version 1.0.0.**

## Status

All milestones (**M0–M9**) are complete. What works:

- One Go binary that loads configuration, opens SQLite, applies migrations and
  serves both the UI and `GET /healthz`.
- Single-password authentication: first-run setup, login, logout and change
  password, with Argon2id hashing, server-side sessions, login throttling and a
  local-only `reset-password` subcommand.
- Items and tags: add and edit one form, Markdown notes rendered safely, tag
  autocomplete and reuse, favorites, sorting, filters, tag chips and paged
  listing.
- Attachments: link attachments and file uploads (streamed and hashed), with
  open, copy and download actions.
- Search: FTS5 over title, description, note and tags, with prefix matching,
  relevance ranking and combination with every filter.
- Trash: soft delete, restore, permanent delete and empty trash, without ever
  losing data by accident.
- Duplicates: add-time warnings by exact URL or SHA-256, and a read-only
  duplicate report grouped by normalized URL and content hash.
- Tag management: rename, merge and delete tags across active and trashed items.
- Export and import: one ZIP archive for the whole repository, with validated,
  all-or-nothing restore into a fresh instance.
- Strict security headers, CSRF protection and a strict Content-Security-Policy
  on every response.

All milestones are complete. See the
[milestone table](docs/README.md#milestones).

## Docker

```sh
docker compose up -d --build
```

The image is a static binary on a distroless, non-root base and stores its data
in a volume. See the [operations guide](docs/guides/operations.md#docker).

## Quick start

Requirements: Go 1.26 or newer, GNU Make.

```sh
make tools   # install pinned goimports and golangci-lint into .tools/bin
make test    # unit, integration and HTTP tests with -race
make run     # start on 127.0.0.1:8080 with a throwaway data directory
```

In another terminal:

```sh
curl http://127.0.0.1:8080/healthz   # -> ok
```

`make check` runs formatting, lint, tests, the build and the documentation
link check in one go. It is the single gate before a commit.

## Documentation

Start at [`docs/README.md`](docs/README.md).

## License

No license has been chosen yet; that decision is tracked in
[`docs/open-questions.md`](docs/open-questions.md).

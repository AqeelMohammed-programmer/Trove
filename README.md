# Trove

Trove is a private, single-user repository for the things you want to find
again years later: notes, links and files, each stored as an **item** with a
title, description, Markdown note, tags and any number of attachments. It runs
as one self-contained web application on a trusted local network, protected by
one password.

The behaviour is defined by [`docs/spec.md`](docs/spec.md). The development
guide is [`AGENTS.md`](AGENTS.md).

## Status

Milestone **M0 (skeleton)** is complete. What works today:

- One Go binary that loads configuration, opens SQLite, applies migrations
  and serves `GET /healthz`.
- Strict security headers on every response.
- A migration runner, proven with tests.
- An FTS5 proof test for the chosen pure-Go SQLite driver.

There is no user interface yet: authentication arrives in M1, items in M2.
See the [milestone table](docs/README.md#milestones) for the full order.

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

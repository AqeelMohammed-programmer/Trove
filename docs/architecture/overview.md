# Architecture overview

This page shows how Trove is put together, which package owns what, and how a
request flows through the system. It is the starting point before reading the
[data model](data-model.md), [storage](storage.md), [search](search.md),
[security](security.md) and [performance](performance.md) pages.

## Goals

- One self-contained binary, one data directory, one user.
- Boring, standard-library-first Go. No framework, no ORM, no build step for
  front-end assets.
- The spec rules are the source of behaviour; this document only describes
  structure.

## Components

```mermaid
flowchart LR
    Browser["Browser<br/>(HTML + vendored HTMX)"]
    HTTP["internal/web<br/>routing, middleware, templates"]
    Service["internal/service<br/>use cases, spec rules"]
    Domain["internal/domain<br/>entities and pure rules"]
    Store["internal/store<br/>SQLite + migrations + FTS5"]
    Blob["internal/blob<br/>copied files on disk"]
    Auth["internal/auth<br/>password, sessions, throttling"]
    Exchange["internal/exchange<br/>export / import archive"]

    Browser -->|HTTP| HTTP
    HTTP --> Service
    Service --> Domain
    Service --> Store
    Service --> Blob
    Service --> Auth
    Service --> Exchange
```

Dependency rules (`AGENTS.md` section 4.2):

- `domain` imports nothing from this project.
- `service` declares the interfaces it needs; `store`, `blob`, `auth` and
  `exchange` implement them. `service` never imports `web` or `database/sql`.
- `web` handlers only parse input, call a service and render a template.
- Wiring happens in `cmd/server/main.go`; there is no global mutable state.

## Packages

| Package | Responsibility | State at M0 |
|---|---|---|
| `cmd/server` | Configuration, wiring, startup and graceful shutdown. | Implemented. |
| `internal/app` | Product name and build information. | Implemented. |
| `internal/config` | Flags and environment variables, validated at startup. | Implemented. |
| `internal/store` | SQLite connection, pragmas, migration runner, item/tag/attachment SQL and FTS building. | Implemented through M4. |
| `internal/web` | HTTP routes, middleware, templates, security headers. | Auth screens and `/healthz`; item UI arrives in M2. |
| `internal/domain` | Entities, validation and pure rules. | Implemented (items, tags, attachments, query types). |
| `internal/service` | Use cases implementing spec rules. | Item and tag use cases implemented. |
| `internal/blob` | Streaming, hashing and atomic storage of copied files. | Implemented. |
| `internal/auth` | Argon2id, sessions, login throttling. | Implemented. |
| `internal/exchange` | ZIP export and restore-only import. | Implemented. |

Empty packages are deliberately not created ahead of time: a milestone creates
them when it has code and tests to put in them.

## Startup sequence

```mermaid
sequenceDiagram
    participant Main as cmd/server
    participant Cfg as internal/config
    participant DB as internal/store
    participant Mig as migrations
    participant Web as internal/web

    Main->>Cfg: Load(args, env)
    Cfg-->>Main: validated Config
    Main->>Main: create data dir (0700)
    Main->>DB: Open(data/repository.sqlite)
    DB-->>Main: *sql.DB with pragmas
    Main->>Mig: Up() inside transactions
    Mig-->>Main: schema at latest version
    Main->>Web: New(logger)
    Main->>Main: ListenAndServe
    Note over Main: SIGINT/SIGTERM -> graceful shutdown
```

## Request flow (from M2)

```mermaid
sequenceDiagram
    participant B as Browser
    participant W as web
    participant S as service
    participant D as SQLite

    B->>W: GET /items?...
    W->>W: parse and validate query
    W->>S: ListItems(ctx, filter)
    S->>D: parameterised SQL
    D-->>S: rows
    S-->>W: items
    W->>W: render html/template
    W-->>B: HTML fragment or full page
```

HTMX requests return the same HTML fragments as normal navigation, so every
action also works with plain links and forms.

## Startup guarantees

- Configuration is validated before anything is created or opened.
- The data directory is created with mode `0700`; the database file is created
  with mode `0600`.
- Migrations run in transactions and never edit history.
- The HTTP server has no read/write timeouts because uploads stream with no
  size cap (spec `N-1`); only `ReadHeaderTimeout` and `IdleTimeout` are set.

## Related pages

- [Data model](data-model.md)
- [Storage](storage.md)
- [Search](search.md)
- [Security](security.md)
- [Performance](performance.md)
- [Development guide](../guides/development.md)

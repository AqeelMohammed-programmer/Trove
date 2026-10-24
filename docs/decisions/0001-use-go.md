# 0001. Use Go for the implementation

**Status:** Accepted
**Date:** 2026-10-24

## Context

Trove is a single-user web application that must ship as one self-contained
binary, run on modest hardware, and stay maintainable for years. The
development guide (`AGENTS.md` section 3) already fixes Go, the standard
library HTTP stack and server-rendered HTML. This ADR records why that fits.

## Decision

Implement Trove in **Go** (module `Trove`, Go 1.26.8), using:

- `net/http` with method-and-path patterns in `http.ServeMux`,
- `html/template` with templates embedded via `embed`,
- `database/sql` with hand-written SQL and no ORM,
- `log/slog` for structured logging,
- `crypto/rand` for identifiers.

The build produces one binary with version information injected through
`-ldflags`.

## Consequences

- Deployment is "copy one binary and a data directory"; no runtime, no server
  to install.
- The standard library covers HTTP, templates, logging and crypto, so the
  dependency list stays short and auditable.
- Compile-time typing and `go vet`/`golangci-lint` catch classes of mistakes
  early.
- Embedding makes templates, migrations and vendored assets part of the
  binary, which eliminates "missing file" deployments.
- The team must be comfortable with Go; there is no plugin ecosystem to lean
  on for UI work, which is deliberate (see ADR 0003).

## Alternatives considered

- **Rust** with a native toolkit such as Slint: fast and small, but the Markdown
  support of that toolkit is only a subset of what `V-1` requires, and
  server-rendered HTML is simpler for a browser-first application.
- **Node.js / TypeScript**: a large ecosystem, but it brings a build toolchain,
  more runtime dependencies and a bigger supply-chain surface than the project
  needs.
- **Python**: quick to write, but packaging a single binary and achieving the
  `N-2` target with 100,000 items is more work than with Go.

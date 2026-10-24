# Trove documentation

This folder is the project wiki. It explains how Trove is built and operated,
and it is versioned together with the code. Behaviour belongs to
[`spec.md`](spec.md); everything else here explains, records or checks it.

## How to read these pages

New to the project:

1. [`../README.md`](../README.md) for what Trove is and how to run it.
2. [`../AGENTS.md`](../AGENTS.md) for the working rules (humans and AI agents).
3. [`spec.md`](spec.md) for what the system must do. Rules have IDs such as
   `I-2`; the rest of the documentation links to those IDs instead of repeating
   them.
4. [`guides/getting-started.md`](guides/getting-started.md) to get a checkout
   running.

Then pick the page for your task.

## Index

### Specification and tracking

- [`spec.md`](spec.md) — the behaviour specification (owned by the user).
- [`traceability.md`](traceability.md) — which test covers which rule.
- [`open-questions.md`](open-questions.md) — unresolved questions.

### Architecture

- [`architecture/overview.md`](architecture/overview.md) — components, packages, request flow.
- [`architecture/data-model.md`](architecture/data-model.md) — tables, indexes, migrations.
- [`architecture/storage.md`](architecture/storage.md) — on-disk layout and file safety.
- [`architecture/search.md`](architecture/search.md) — FTS5 usage and query building.
- [`architecture/security.md`](architecture/security.md) — threats and controls.
- [`architecture/performance.md`](architecture/performance.md) — targets and measurements.

### Guides

- [`guides/getting-started.md`](guides/getting-started.md) — clone to running in five minutes.
- [`guides/development.md`](guides/development.md) — workflow, make targets, conventions.
- [`guides/testing.md`](guides/testing.md) — test layers and how to write tests.
- [`guides/operations.md`](guides/operations.md) — configuration, backup, upgrades, password reset.

### Decisions (ADRs)

- [`decisions/0000-template.md`](decisions/0000-template.md) — the template.
- [`decisions/0001-use-go.md`](decisions/0001-use-go.md) — Go as the implementation language.
- [`decisions/0002-sqlite-and-fts5.md`](decisions/0002-sqlite-and-fts5.md) — SQLite, driver choice, FTS5 proof.
- [`decisions/0003-server-rendered-html-with-htmx.md`](decisions/0003-server-rendered-html-with-htmx.md) — pages and interactivity.
- [`decisions/0004-password-reset-as-cli-command.md`](decisions/0004-password-reset-as-cli-command.md) — reset without HTTP.
- [`decisions/0005-export-import-archive.md`](decisions/0005-export-import-archive.md) — the ZIP archive format.
- [`decisions/0006-markdown-and-sanitizing.md`](decisions/0006-markdown-and-sanitizing.md) — goldmark plus bluemonday for safe notes.

## Milestones

Build order from `AGENTS.md` section 10.1. A milestone is done only when every
rule it covers has a test and its pages are updated.

| # | Milestone | Status |
|---|---|---|
| M0 | Skeleton: repo, make targets, config, migrations, health, FTS5 proof, ADRs, docs | done |
| M1 | Authentication | done |
| M2 | Items and tags | done |
| M3 | Attachments | done |
| M4 | Search | not started |
| M5 | Trash | not started |
| M6 | Duplicates | not started |
| M7 | Tag management | not started |
| M8 | Export and import | not started |
| M9 | Hardening | not started |

## Conventions

- Rule IDs (`I-2`, `S-6`, …) come from the spec and are referenced, not
  restated.
- One ADR per significant decision, numbered, never rewritten; supersede
  instead.
- Every page starts with a purpose paragraph and links to related pages.
- Every relative link is checked by `make docs-check`.

# Development guide

This guide is for anyone changing Trove: the workflow, the commands, and the
conventions that keep changes reviewable. The binding rules are in
[`../../AGENTS.md`](../../AGENTS.md); this page is the practical summary.

## Before you change anything

1. Read the relevant rules in [`../spec.md`](../spec.md), plus section 1.3
   (out of scope).
2. Read the page for the area: [architecture](../architecture/overview.md),
   [data model](../architecture/data-model.md), [storage](../architecture/storage.md),
   [search](../architecture/search.md), [security](../architecture/security.md).
3. Run `make check`. It must be green before you start.

If the spec is ambiguous or silent, stop and ask. Record the question in
[open questions](../open-questions.md) rather than guessing.

## Branches and commits

- Work on a branch per feature or milestone; never commit directly to `main`.
- Use Conventional Commit messages: `feat:`, `fix:`, `docs:`, `test:`,
  `refactor:`, `chore:`.
- Mention rule IDs in the body when the change implements them, for example
  `feat: reject non-http link URLs (A-2)`.
- Keep commits small; each one must build and pass tests.

## Make targets

| Command | What it does |
|---|---|
| `make run` | Run with a throwaway data directory on `127.0.0.1:8080`. |
| `make fmt` | Format with `gofmt` and `goimports`. |
| `make fmt-check` | Fail if formatting is needed. |
| `make lint` | `go vet` plus `golangci-lint`. |
| `make test` | All tests with `-race`. |
| `make build` | Build `bin/trove` with version information. |
| `make check` | Everything above plus the documentation check. |
| `make bench` | Performance benchmarks (meaningful from M2). |
| `make tools` | Install pinned tools into `.tools/bin`. |
| `make clean` | Remove build output and tools. |

`make check` is the only gate. There is no required hosting or CI service; if
your environment has a runner, configure it to run exactly `make check`.

## Code conventions

The full list is in `AGENTS.md` section 5. The ones that come up daily:

- Standard library first. Every new dependency needs an ADR and approval.
- `context.Context` first for anything doing I/O; wrap errors with `%w`.
- Return errors in request paths; do not panic.
- Small interfaces declared by the consumer, not the implementation.
- Handlers: parse, call a service, render. No SQL and no business rules.
- Every exported identifier and package has a doc comment explaining *why*.
- Keep files under about 400 lines.
- Use an injected clock for anything time-dependent so tests are deterministic.
- Tests are part of the change, not a follow-up.

## Adding a feature

1. Identify the spec rule IDs and re-read them.
2. Write a short plan. If it touches the schema, security or file format, write
   or update an ADR first.
3. Write tests first or alongside the code, with the rule ID in the test name.
4. Implement the smallest change that satisfies the rules.
5. Run `make check` and fix everything.
6. Update the docs: the architecture page you touched, the
   [traceability table](../traceability.md), and [`CHANGELOG.md`](../../CHANGELOG.md).
7. Commit with a Conventional Commit message.

## Database changes

- Add a new numbered file in [`migrations/`](../../migrations/README.md); never
  edit one that has been merged.
- Use `STRICT` tables and the conventions in the
  [data model](../architecture/data-model.md).
- Update `docs/architecture/data-model.md` in the same commit.
- Test the migration on an empty database and on the previous schema with data.

## Dependencies

Adding a module requires an entry in [`decisions/`](../decisions/0000-template.md)
covering: what it is for, why the standard library is not enough, maintenance
and license. Then run `go mod tidy` and `go mod verify`, and commit `go.sum`.
Prefer the standard library and boring solutions.

## Front end

Pages are server-rendered with `html/template`; HTMX is vendored into the
repository. There is no build step and no CDN. Every action must work as a
plain link or form before HTMX improves it. Asset requirements are in
`AGENTS.md` sections 3 and 11.

## Documentation

Documentation is code: a change that alters behaviour, structure,
configuration or operation is not done until the matching page is updated.
`make docs-check` fails on broken relative links or a page missing from the
[documentation index](../README.md).

## Related pages

- [Getting started](getting-started.md)
- [Testing](testing.md)
- [Architecture overview](../architecture/overview.md)
- [Open questions](../open-questions.md)

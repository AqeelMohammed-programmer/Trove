# Testing guide

This page explains the test layers, how to run them and how to write a test that
counts as covering a spec rule. Tests are how the specification is enforced.

## Commands

```sh
make test          # everything, with -race, no cache
make bench         # benchmarks only (performance check, M9)
go test ./internal/store -run FTS5 -v   # one package, one test
```

Always use the race detector for the full suite; it is part of `make test`.

## Layers

| Layer | Packages | What it tests |
|---|---|---|
| Unit | `internal/domain`, `internal/service` | Pure rules, table-driven, no I/O, injected clock. |
| Integration | `internal/store`, `internal/blob`, `internal/exchange` | Real SQLite files in `t.TempDir()` and real temporary directories. The database is never mocked. |
| HTTP | `internal/web` | Status codes, redirects, auth enforcement, CSRF, rendered output, headers. |
| Round trip | `internal/exchange` | Export then import into a fresh instance, compared field by field (`E-10`). |
| Security regression | across layers | Raw HTML in Markdown, `javascript:` URLs, path traversal, oversized fields, FTS syntax injection. |
| Performance | `make bench` | 100,000 items against `N-2`. |

## Naming tests after rules

Every test that enforces a spec rule must name the rule in the test name or a
comment, for example:

```go
func TestItemTitleDefaultsToFirstAttachment_I2(t *testing.T)
```

Then update [`../traceability.md`](../traceability.md) in the same change:
replace the gap with the test name and mark it covered. A milestone is not done
while one of its rules is still a gap.

## What M0 covers

| Test | Proves |
|---|---|
| `TestLoad_*` | Configuration precedence and validation. |
| `TestOpen_*` | Pragmas, file permissions, foreign keys, unusual paths. |
| `TestMigrator_*` | Ordered, transactional, idempotent migrations; strict naming. |
| `TestFTS5_AvailableAndUsable` | The driver really supports FTS5, prefix matching, case folding and bm25. This gates ADR 0002. |
| `TestHealthz_*`, `TestUnknownRoute_NotFound`, `TestSecurityHeaders_*` | The HTTP surface and its headers. |

No spec rule is implemented yet, so all rule tests are still gaps; the reasons
and milestones are listed in the traceability table.

## Writing a good test

- Prefer real dependencies over mocks, except for the clock and the network.
- Cover the failure path as carefully as the happy path: invalid input,
  not-found, conflicts, empty collections.
- Do not retry flaky tests. Fix the race or the assumption.
- Coverage is a signal, not a target; do not add meaningless assertions.
- Use `t.TempDir()` and `t.Cleanup` so tests can run in parallel.

## Related pages

- [Development guide](development.md)
- [Performance](../architecture/performance.md)
- [Traceability](../traceability.md)

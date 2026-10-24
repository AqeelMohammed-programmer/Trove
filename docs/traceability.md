# Traceability

This page maps every rule in [`spec.md`](spec.md) to the test or tests that
enforce it. `AGENTS.md` section 8 requires this table to be updated in the same
change that adds or modifies a rule's behaviour.

**Status:** at M0 no product rule is implemented yet, so every rule below is an
open gap with the milestone that will cover it. A gap is not "done"; do not mark
a milestone complete while one of its rules is still a gap.

## How to use this page

- When you write a test for a rule, name or comment it with the rule ID, for
  example `TestItemTitleDefaultsToFirstAttachment_I2`.
- Replace the `—` in the "Test(s)" column with the test name and change the
  status to **covered**.
- If a rule needs more than one test (happy path and failures), list them all.

## Items (`I-*`)

| Rule | Test(s) | Status |
|---|---|---|
| I-1 | — | gap (M2) |
| I-2 | — | gap (M2) |
| I-3 | — | gap (M2) |
| I-4 | — | gap (M2) |
| I-5 | — | gap (M2) |
| I-6 | — | gap (M2) |
| I-7 | — | gap (M2) |
| I-8 | — | gap (runtime guard, M2/M9) |
| I-9 | — | gap (M2) |

## Attachments (`A-*`)

| Rule | Test(s) | Status |
|---|---|---|
| A-1 | — | gap (M3) |
| A-2 | — | gap (M3) |
| A-3 | — | gap (M3) |
| A-4 | — | gap (M3) |
| A-5 | — | gap (M3) |
| A-6 | — | gap (M3) |
| A-7 | — | gap (M3) |
| A-8 | — | gap (M3) |
| A-9 | — | gap (M5) |

## Tags and favorites (`T-*`, `F-*`)

| Rule | Test(s) | Status |
|---|---|---|
| T-1 | — | gap (M2) |
| T-2 | — | gap (M2) |
| T-3 | — | gap (M2) |
| T-4 | — | gap (M2) |
| T-5 | — | gap (M2) |
| T-6 | — | gap (M2) |
| F-1 | — | gap (M2) |
| F-2 | — | gap (M2) |
| F-3 | — | gap (M2) |

## Browsing (`B-*`)

| Rule | Test(s) | Status |
|---|---|---|
| B-1 | — | gap (M2) |
| B-2 | — | gap (M2) |
| B-3 | — | gap (M2) |
| B-4 | — | gap (M2) |
| B-5 | — | gap (M2) |
| B-6 | — | gap (M2) |
| B-7 | — | gap (M2) |

## Search (`S-*`)

| Rule | Test(s) | Status |
|---|---|---|
| S-1 | — | gap (M4) |
| S-2 | — | gap (M4) |
| S-3 | — | gap (M4) |
| S-4 | — | gap (M4) |
| S-5 | — | gap (M4) |
| S-6 | — | gap (M4) |

## Duplicates (`D-*`, `R-*`)

| Rule | Test(s) | Status |
|---|---|---|
| D-1 | — | gap (M6) |
| D-2 | — | gap (M6) |
| D-3 | — | gap (M6) |
| D-4 | — | gap (M6) |
| D-5 | — | gap (M6) |
| R-1 | — | gap (M6) |
| R-2 | — | gap (M6) |
| R-3 | — | gap (M6) |
| R-4 | — | gap (M6) |
| R-5 | — | gap (M6) |
| R-6 | — | gap (M6) |

## Trash (`X-*`)

| Rule | Test(s) | Status |
|---|---|---|
| X-1 | — | gap (M5) |
| X-2 | — | gap (M5) |
| X-3 | — | gap (M5) |
| X-4 | — | gap (M5) |
| X-5 | — | gap (M5) |
| X-6 | — | gap (M5) |
| X-7 | — | gap (M5) |
| X-8 | — | gap (M5) |
| X-9 | — | gap (M5) |
| X-10 | — | gap (M5) |
| X-11 | — | gap (M5) |
| X-12 | — | gap (M5) |
| X-13 | — | gap (M5) |

## Tag management (`M-*`)

| Rule | Test(s) | Status |
|---|---|---|
| M-1 | — | gap (M7) |
| M-2 | — | gap (M7) |
| M-3 | — | gap (M7) |
| M-4 | — | gap (M7) |
| M-5 | — | gap (M7) |

## Markdown (`V-*`)

| Rule | Test(s) | Status |
|---|---|---|
| V-1 | — | gap (M2) |
| V-2 | — | gap (M2) |
| V-3 | — | gap (M2) |
| V-4 | — | gap (M2) |

## Authentication (`U-*`)

| Rule | Test(s) | Status |
|---|---|---|
| U-1 | — | gap (M1) |
| U-2 | — | gap (M1) |
| U-3 | — | gap (M1) |
| U-4 | — | gap (M1) |
| U-5 | — | gap (M1) |
| U-6 | — | gap (M1) |
| U-7 | — | gap (M1) |
| U-8 | — | gap (M1) |
| U-9 | — | gap (M1) |
| U-10 | — | gap (M1) |
| U-11 | — | gap (deployment, out of scope) |

## Export and import (`E-*`)

| Rule | Test(s) | Status |
|---|---|---|
| E-1 | — | gap (M8) |
| E-2 | — | gap (M8) |
| E-3 | — | gap (M8) |
| E-4 | — | gap (M8) |
| E-5 | — | gap (M8) |
| E-6 | — | gap (M8) |
| E-7 | — | gap (M8) |
| E-8 | — | gap (M8) |
| E-9 | — | gap (M8) |
| E-10 | — | gap (M8) |

## Scale (`N-*`)

| Rule | Test(s) | Status |
|---|---|---|
| N-1 | — | gap (limits documented in M9) |
| N-2 | — | gap (`make bench`, M9) |

## Infrastructure tests (not spec rules)

These tests protect the foundations the spec rules depend on.

| Area | Test | What it proves |
|---|---|---|
| Configuration | `TestLoad_Defaults`, `TestLoad_EnvOverridesDefaults`, `TestLoad_FlagsOverrideEnv`, `TestLoad_InvalidInput` | Flags beat environment variables, which beat defaults; invalid input fails at startup. |
| SQLite setup | `TestOpen_AppliesPragmas`, `TestOpen_EnforcesForeignKeys` | Foreign keys on, WAL journal, busy timeout set. |
| File safety | `TestOpen_CreatesPrivateFile`, `TestOpen_PathWithSpaces` | The database file is `0600`; unusual paths work. |
| Migrations | `TestMigrator_AppliesInOrderAndIsIdempotent`, `TestMigrator_RollsBackFailedMigration`, `TestMigrator_RejectsMalformedNames` | Ordered, transactional, idempotent, strict naming. |
| FTS5 | `TestFTS5_AvailableAndUsable` | The driver supports FTS5, prefix matching, case folding and bm25 ranking (see ADR 0002). |
| HTTP surface | `TestHealthz_ReturnsOK`, `TestHealthz_OnlyAllowsGET`, `TestUnknownRoute_NotFound`, `TestHealthz_RevealsNoData`, `TestSecurityHeaders_AppliedToEveryResponse` | `/healthz` is minimal and read-only, and security headers are on every response. |

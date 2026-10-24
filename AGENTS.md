# AGENTS.md: Guide for AI agents (and humans) working on this project

This file tells you how to work on this codebase. Read it fully before changing anything. It is written for AI coding agents, but a human developer should follow the same rules.

---

## 1. What this project is

A **personal repository**: one person stores items (title, description, Markdown note, tags, favorite flag) with any number of attachments (links or copied files), then finds them later by search, tags, and favorites. One instance serves one user, protected by one password, used in a browser on a trusted local network.

- **The source of truth for behavior is `docs/spec.md`.** It has numbered rules (`I-2`, `D-1`, `X-4`, and so on). Your job is to implement those rules exactly.
- This file (`AGENTS.md`) is the source of truth for **how** to build and maintain the system: stack, structure, standards, process.
- The product name is not decided yet. Never hard-code it. Use one constant (`internal/app/name.go`) and one place in `go.mod`, so renaming later is a one-line change.

### Read order at the start of every session

1. This file.
2. `docs/spec.md` (the parts relevant to your task, plus section 1.3 "Out of scope").
3. `docs/README.md` (the wiki index) and the page for the area you will touch.
4. `docs/decisions/` (architecture decision records) for anything that looks like a deliberate choice.
5. Run `make test` to confirm the project is green before you start.

---

## 2. Ground rules (the short version)

1. **The spec wins.** Do not change behavior the spec defines. Do not add features the spec does not define.
2. **If the spec is ambiguous, wrong, or silent, stop and ask.** Do not guess. Write the question in `docs/open-questions.md`, and ask the user. Once answered, record the answer (in the spec if it changes behavior, or in an ADR if it is a technical choice).
3. **Never edit `docs/spec.md` without the user's explicit approval.** When approved, bump its status line and note the change in `CHANGELOG.md`.
4. **Everything in "Out of scope" (spec section 1.3) stays out.** No multi-user, folders, ratings, page fetching, inline note images, file-content search, or path references. Do not "helpfully" add them.
5. **No outside network calls from the running app** (spec `I-8`). That includes CDNs for scripts, styles, or fonts, analytics, update checks, and telemetry. Vendor every front-end asset into the repository.
6. **Small, reviewable, working steps.** Each change leaves `make check` green.
7. **Docs are part of the code.** A change is not done until the relevant wiki page is updated (section 9).
8. **Prefer the standard library and boring solutions.** Every new dependency needs a written justification (section 4.3).

---

## 3. Technology stack (decided)

| Concern | Choice | Notes |
|---|---|---|
| Language | **Go** (latest stable release) | Set the version in `go.mod`. Check go.dev for the current stable release at project start. |
| HTTP server and routing | Standard library `net/http` | Use method-and-path patterns in `http.ServeMux`. No web framework. |
| Pages | Server-rendered HTML with `html/template` | Templates embedded in the binary with `embed`. |
| Interactivity | **HTMX**, vendored | Small fragments of HTML returned by the server. Pages must still work through normal links and forms. |
| Styling | Plain CSS, hand-written | No build step, no CSS framework, no Node toolchain. |
| Database | **SQLite** with FTS5 for search | Use a pure-Go driver (for example `modernc.org/sqlite`) so the build needs no C compiler. **First task of milestone M0: prove with a test that the chosen driver supports FTS5.** If it does not, record the finding in an ADR and pick another driver. |
| Data access | `database/sql` with hand-written SQL | No ORM. |
| Migrations | Numbered, forward-only SQL files embedded in the binary | Applied automatically at startup, inside transactions. |
| Markdown | `goldmark` for rendering, `bluemonday` for sanitizing | Raw HTML in notes must never be rendered (spec `V-2`). |
| Password hashing | Argon2id (`golang.org/x/crypto/argon2`) | Salted, with parameters stored alongside the hash. |
| Logging | `log/slog` (standard library) | Structured logs. |
| Build output | One self-contained binary | Plus a Dockerfile as an optional convenience (milestone M9). |

Keep the stack small on purpose. Changing any row of this table requires an ADR and the user's approval.

---

## 4. Architecture

### 4.1 Layout

```
.
├── AGENTS.md
├── CHANGELOG.md
├── Makefile
├── go.mod
├── cmd/
│   └── server/main.go          # wiring only: config, dependencies, start
├── internal/
│   ├── app/                    # name constant, version info
│   ├── config/                 # configuration loading and validation
│   ├── domain/                 # entities and pure rules; no I/O, no imports of other internal packages
│   ├── service/                # use cases implementing the spec rules
│   ├── store/                  # SQLite persistence, migrations, FTS
│   ├── blob/                   # copied-file storage on disk
│   ├── auth/                   # passwords, sessions, login throttling
│   ├── exchange/               # export and import archive
│   └── web/                    # handlers, templates, static assets, middleware
├── migrations/                 # numbered .sql files (embedded)
└── docs/                       # the wiki (section 9)
```

### 4.2 Dependency direction

`web` → `service` → `domain`, and `store`, `blob`, `auth`, `exchange` implement interfaces that `service` defines.

- `domain` imports nothing from this project.
- `service` never imports `web`, `store`, or `database/sql`. It talks to persistence through small interfaces it declares itself.
- `web` handlers do three things only: parse and validate the request, call a service, render a template. No business rules and no SQL in handlers or templates.
- All wiring happens in `cmd/server/main.go`. No global mutable state, no `init()` side effects, dependencies passed in through constructors.

### 4.3 Dependencies

Before adding a third-party module, add a short entry to `docs/decisions/` explaining: what it is for, why the standard library is not enough, how maintained it is, and its license. Prefer well-maintained modules with a permissive license. Run `go mod tidy` and `go mod verify`, and commit `go.sum`.

---

## 5. Coding standards (Go)

- **Format and lint:** `gofmt` and `goimports` are mandatory. `go vet` and `golangci-lint` must pass with the configuration committed in `.golangci.yml` (enable at least: `errcheck`, `govet`, `staticcheck`, `revive`, `gosec`, `errorlint`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `unused`, `ineffassign`).
- **Errors:** return errors, do not panic in request paths. Wrap with context using `fmt.Errorf("doing X: %w", err)`. Define sentinel or typed errors for conditions the caller must distinguish (not found, validation, conflict). Never ignore an error without a comment explaining why.
- **Context:** pass `context.Context` as the first parameter of anything that does I/O, and honor cancellation.
- **Validation:** validate at the boundary (limits, formats, allowed URL schemes) using the defaults in spec section 8.2. Domain constructors enforce invariants so invalid objects cannot exist.
- **Naming and size:** short, clear names; no stuttering (`item.Item` is fine, `item.ItemItem` is not). Keep functions short and single-purpose. Avoid files over about 400 lines.
- **Interfaces:** small, defined where they are used (the consumer), not where they are implemented.
- **No premature abstraction.** Do not add layers, generics, or plugin points "for later".
- **Comments:** every exported identifier and every package has a doc comment. Comments explain *why*, not *what*. Reference spec rule IDs where a block implements a rule (for example `// Implements I-2`).
- **Concurrency:** keep it simple. If you start a goroutine, document who stops it and how. Run tests with `-race`.
- **Time and IDs:** inject a clock interface so tests are deterministic. Store timestamps in UTC as ISO 8601 text. Generate IDs as 128-bit random values from `crypto/rand`, hex-encoded (opaque, never reused).

---

## 6. Data and storage rules

- **SQLite settings at connection time:** `PRAGMA foreign_keys = ON`, WAL journal mode, a busy timeout, and `STRICT` tables in migrations.
- **Migrations:** one numbered file per change (`0001_init.sql`, `0002_...sql`). Never edit a migration after it has been merged. Add a new one. Each migration runs in a transaction. Document every schema change in `docs/architecture/data-model.md` in the same change.
- **Transactions:** any operation that changes more than one row or table is one transaction (trash and restore, tag merge, item save with attachments, import).
- **Search (FTS5):** keep the index in sync inside the same transaction as the data change (`S-6`). Build search queries from user input by tokenizing it, quoting each token, and appending `*` for prefix matching. **Never pass raw user input as FTS syntax.**
- **Duplicates:** add indexes on link URL and file `sha256`. Store a normalized URL column (fragment and one trailing slash removed, rule `R-2`) at write time so the report is fast.
- **Files on disk:**
  - Stored under the data directory by attachment id, never by user-supplied file name.
  - Write to a temporary file in the same directory while streaming and computing SHA-256, then rename atomically. Never load a whole upload into memory.
  - Delete stored content only on permanent deletion (`X-9`). Do it after the database transaction commits, and make a failed disk delete recoverable (log it, and keep a cleanup routine that removes orphaned blobs).
  - Protect against path traversal: file names are metadata only and never used to build paths.
- **Import:** validate the whole archive first (`E-6`), then import inside one transaction plus a staging directory, so a failure leaves the repository empty (`E-7`).
- **Data directory permissions:** create with `0700`, files with `0600`.

---

## 7. Security rules

These implement spec section 5 and common web practice. Treat each as a requirement.

- **Authentication on everything.** Every route requires a session, except first-run setup, login, static assets, and a minimal `/healthz` that returns no data. A test must list all registered routes and verify that unauthenticated requests are rejected (`U-3`).
- **Sessions:** server-side sessions with a random token. The cookie has no `Expires` or `Max-Age` (so it ends with the browser, `U-5`), and is `HttpOnly` and `SameSite=Lax` (add `Secure` when served over HTTPS, controlled by configuration). Regenerate the session on login.
- **CSRF:** protect every state-changing request (check the `Origin` or `Sec-Fetch-Site` header and/or use tokens). GET requests never change state.
- **Login throttling** per `U-9`, using the defaults in spec section 8.2. Use constant-time comparison for secrets.
- **Passwords:** Argon2id only. Never log passwords, hashes, or session tokens.
- **Password reset** (`U-10`): implement as a command-line subcommand of the same binary (for example `server reset-password`) that prompts on the terminal and works only on the machine running the system. It must not be reachable over HTTP. It ends all sessions and touches no other data.
- **Output safety:**
  - Use `html/template` auto-escaping everywhere. Do not mark user content as trusted HTML.
  - Render Markdown with raw HTML disabled, then sanitize the result (`V-2`).
  - Set a strict `Content-Security-Policy` (`default-src 'self'`, no inline scripts or styles) plus `X-Content-Type-Options: nosniff`, `Referrer-Policy`, and `X-Frame-Options` or `frame-ancestors`. Configure the vendored HTMX for strict CSP (check its documentation for the settings that disable `eval` and inline indicator styles).
  - Serve downloads with `Content-Disposition: attachment`, a safe encoded file name, and `nosniff`, so uploaded HTML or SVG files can never run in the app's origin.
  - Render link attachments only for `http` and `https` URLs (`A-2`), with `target="_blank"` and `rel="noopener noreferrer"`.
- **Limits:** apply spec length limits to text fields. Uploads are streamed with no size cap (`N-1`), but request bodies for ordinary forms have a small cap.
- **Secrets and config** never go in the repository. Run `gosec` in CI and `govulncheck` regularly.

---

## 8. Testing

Tests are how the spec is enforced. Write them with the code, not after.

- **Traceability:** every spec rule ID must be covered by at least one test whose name or comment includes the ID (for example `TestItemTitleDefaultsToFirstAttachment_I2`). Maintain `docs/traceability.md`, a table of rule ID → test(s), updated in the same change. A rule with no test is an open gap and must be listed as such.
- **Layers:**
  - **Unit tests** for `domain` and `service` (table-driven, fast, no I/O, injected clock).
  - **Integration tests** for `store`, `blob`, and `exchange` against a **real SQLite database and real temporary directories**. Do not mock the database.
  - **HTTP tests** with `httptest` for handlers: status codes, redirects, authentication required, CSRF, rendered output.
  - **Round-trip test** for export then import into a fresh instance (`E-10`), and tests for every import validation failure (`E-6`, `E-7`).
  - **Security regression tests:** Markdown with raw HTML, `javascript:` URLs, path-traversal file names, oversized text fields, FTS syntax injection.
  - **Performance check:** a script or benchmark that seeds 100,000 items and measures list and search time against `N-2`. Report the numbers in `docs/architecture/performance.md`.
- **Always run with `-race`** in CI.
- **No flaky tests.** Fix or delete them, never retry-until-green.
- Coverage is a signal, not a goal. Do not write meaningless tests to raise a number.

### Commands (the `Makefile` must provide these)

| Command | What it does |
|---|---|
| `make run` | Run the server locally with a temporary data directory. |
| `make fmt` | Format code. |
| `make lint` | `go vet` and `golangci-lint`. |
| `make test` | Unit, integration, and HTTP tests with `-race`. |
| `make check` | Format check, lint, tests, build, and doc link check. This is what CI runs. |
| `make build` | Build the single binary with version info injected. |
| `make bench` | Run the 100,000-item performance check. |

CI (for example GitHub Actions) runs `make check` on every push and pull request.

---

## 9. Documentation (the wiki)

Documentation lives in the repository under `docs/`, written in Markdown, so it is versioned with the code and can be published as a wiki (for example by pointing a GitHub wiki or a static site generator at the folder). **A change that alters behavior, structure, configuration, or operation is not done until the matching page is updated in the same change.**

### 9.1 Structure

```
docs/
├── README.md                   # wiki home: what this is, index of all pages, how to read them
├── spec.md                     # behavior specification (owned by the user)
├── traceability.md             # spec rule ID → tests
├── open-questions.md           # unresolved questions waiting for the user
├── architecture/
│   ├── overview.md             # big picture, package responsibilities, request flow (with diagrams)
│   ├── data-model.md           # tables, relationships, indexes, migration history
│   ├── storage.md              # on-disk layout of copied files, atomic writes, cleanup
│   ├── search.md               # how FTS5 is used, query building, ranking
│   ├── security.md             # threat model, controls, how each spec rule is met
│   └── performance.md          # targets, how measured, results
├── guides/
│   ├── getting-started.md      # clone, run, test in five minutes
│   ├── development.md          # workflow, make targets, code conventions, how to add a feature
│   ├── testing.md              # test layers, how to write and run them
│   └── operations.md           # configuration, backup and restore, password reset, logs, upgrades
└── decisions/
    ├── 0000-template.md
    ├── 0001-use-go.md
    ├── 0002-sqlite-and-fts5.md
    ├── 0003-server-rendered-html-with-htmx.md
    └── ...                     # one record per significant decision
```

### 9.2 Writing rules

- Write for a developer who has never seen the project. Short sentences, plain words, and define a term the first time it appears.
- Task-oriented: each guide starts with what the reader will achieve, then numbered steps with copy-pasteable commands. **Run the commands you document.**
- Use Mermaid diagrams for architecture and flows (they render in most Markdown viewers and live as text in the repository).
- Every page starts with a one-paragraph purpose and links to related pages. `docs/README.md` links to every page, and CI fails on broken links.
- Do not duplicate the spec. Link to rule IDs instead of restating them.
- Keep pages focused. Split a page that grows past what a person reads in one sitting.
- **Architecture Decision Records (ADRs):** one file per significant decision, using `0000-template.md` (Title, Status, Context, Decision, Consequences, Alternatives considered). Never rewrite history. Supersede an old record with a new one and mark the old one as superseded.
- **In the first milestone (M0), write ADRs for:** Go, SQLite and FTS5 (including the driver choice and the FTS5 check result), server-rendered HTML with HTMX, password-reset as a CLI command, and the import/export archive handling.

### 9.3 Other project files

- `README.md` (repository root): one screen covering what the project is, how to run it, and a link to `docs/README.md`.
- `CHANGELOG.md`: follows the "Keep a Changelog" format, with an `Unreleased` section updated with every user-visible change.
- `LICENSE`: **ask the user which license to use. Do not choose one yourself.**

---

## 10. Working process

### 10.1 Milestones

Build in vertical slices in this order. Do not start a milestone before the previous one is green and documented. Each milestone ends with every rule it covers having tests, and its wiki pages updated.

| # | Milestone | Covers |
|---|---|---|
| M0 | **Skeleton.** Repository, `Makefile`, CI, lint config, config loading, migrations runner, health route, FTS5 proof test, initial ADRs, docs skeleton. | Foundations |
| M1 | **Authentication.** First-run setup, login, logout, change password, session handling, throttling, reset command, route-protection test. | Section 5 (`U-*`) |
| M2 | **Items and tags.** Add and edit form, item detail, Markdown rendering, tags with autocomplete, favorites, list rows, sorting, filters, pagination. | `I-*`, `T-*`, `F-*`, `B-*`, `V-*` |
| M3 | **Attachments.** Links, file upload with streaming and SHA-256, multi-file add, download, labels. | `A-*` |
| M4 | **Search.** FTS5 index, prefix and multi-word matching, relevance ranking, combination with filters. | `S-*` |
| M5 | **Trash.** Item and attachment soft delete, restore, permanent delete, empty trash, orphan cleanup. | `X-*` |
| M6 | **Duplicates.** Add-time warnings and the duplicate report. | `D-*`, `R-*` |
| M7 | **Tag management.** Rename, merge, delete. | `M-*` |
| M8 | **Export and import.** Archive format, validation, restore-only import, round-trip test. | `E-*` |
| M9 | **Hardening.** Performance check at 100,000 items, security review, Dockerfile and compose example, accessibility pass, documentation review. | `N-*`, polish |

### 10.2 For every task

1. Identify the spec rule IDs involved. Re-read them.
2. State a short plan (a few lines) before coding. For anything that touches the schema, security, or the file format, write or update an ADR first.
3. Write the tests first or alongside the code.
4. Implement the smallest change that satisfies the rules.
5. Run `make check`. Fix everything before moving on.
6. Update docs, `docs/traceability.md`, and `CHANGELOG.md`.
7. Commit.

### 10.3 Git conventions

- Branch per feature or milestone; never commit directly to `main`.
- **Conventional Commits** for messages (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`), with the rule IDs in the body when relevant.
- Small, focused commits. Each commit builds and passes tests.
- Pull request description includes: what changed, spec rule IDs covered, how it was tested, doc pages updated, migration notes if any.
- Never commit secrets, data directories, or built binaries.
- Version with Semantic Versioning. Inject the version at build time with `-ldflags`.

### 10.4 Configuration

Configuration comes from command-line flags and environment variables, validated at startup, with safe defaults and clear error messages. Document every option in `docs/guides/operations.md`. At minimum: listen address, data directory, and a switch for the `Secure` cookie flag.

---

## 11. User interface rules

- **Progressive enhancement:** every action works through normal links and forms. HTMX only makes it smoother. Back/forward buttons and URLs must behave sensibly, and filter state belongs in the URL so views can be bookmarked.
- Follow the screens in spec section 4 exactly (layout, list rows, detail view, form, tag screen, trash, duplicates, export and import).
- **Accessibility:** semantic HTML, labels on every control, visible focus, full keyboard operation, sufficient contrast, and text alternatives for icons (for example the favorite star and attachment icons).
- **Responsive:** usable on a phone-sized screen, since the app is reached from a browser on other devices.
- Clear empty states and error messages in plain language. Confirm only what the spec says to confirm (`X-13`, section 8.2).

---

## 12. Definition of done

A task is done only when all of these are true:

- [ ] Behavior matches the spec rules involved, and nothing outside the spec was added.
- [ ] Tests exist for every rule involved, including failure and edge cases, and pass with `-race`.
- [ ] `make check` passes (format, lint, tests, build, doc links).
- [ ] No new network calls from the running app; no unjustified new dependency.
- [ ] Security rules in section 7 are respected for the code touched.
- [ ] Docs updated: relevant wiki pages, `docs/traceability.md`, ADR if a decision was made, `CHANGELOG.md`.
- [ ] Migrations (if any) are new files, tested on an empty database and on the previous schema with data.
- [ ] Open questions are written in `docs/open-questions.md`, not silently decided.

---

## 13. When you are stuck or unsure

1. Re-read the spec rule and the relevant wiki page.
2. Search the codebase for how a similar case was handled.
3. If still unclear, **ask the user** with a specific question and your recommended answer. Put it in `docs/open-questions.md` and continue with unrelated work meanwhile.
4. If you find a mistake or contradiction in the spec, report it. Do not work around it silently.
5. If you find a mistake in your own earlier work, fix it, add a test that would have caught it, and note it in the changelog if it affected users.

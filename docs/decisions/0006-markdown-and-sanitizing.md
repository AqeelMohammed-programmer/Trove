# 0006. Markdown rendering with goldmark and bluemonday

**Status:** Accepted
**Date:** 2026-10-24

## Context

Item notes are free-form Markdown (spec 2.1, `V-1`). Rendering them safely is
security-critical: a note is user input turned into HTML on a page that shares
the app's origin, so raw HTML must never be rendered or executed (`V-2`),
Markdown image syntax must not fetch or display remote content (`V-3`), and
links in notes must open in a new tab with a safe `rel` (`V-4`).

`AGENTS.md` section 3 fixes the stack: **goldmark** for rendering and
**bluemonday** for sanitizing.

## Decision

Render notes with **goldmark** (GFM extension) and sanitize the result with
**bluemonday**.

- A custom goldmark AST transformer rewrites every `Image` node into a link, so
  image syntax becomes an ordinary link instead of an `<img>` (`V-3`).
- goldmark's default renderer omits raw HTML; bluemonday's UGC policy then
  strips anything that survives (`V-2`).
- The sanitizer adds `target="_blank"` and `rel="noopener"` to links (`V-4`).
- Tables and their `colspan`/`rowspan` attributes are allowed on top of the UGC
  policy, because `V-1` lists tables as allowed.
- Rendered HTML is the only value the web layer marks as `template.HTML`; it is
  sanitized first, and a `#nosec` comment records why that is safe.

### Dependencies

| Module | Version | License | Why not the standard library |
|---|---|---|---|
| `github.com/yuin/goldmark` | v1.8.6 | MIT | The standard library has no Markdown parser. golmark is the de-facto Go implementation: CommonMark-compliant, extended, and actively maintained. |
| `github.com/microcosm-cc/bluemonday` | v1.0.27 | BSD-3-Clause | The standard library has no HTML sanitizer. bluemonday is the standard Go choice, policy-driven and widely reviewed. |

Both are permissively licensed and maintained. `go mod tidy` and
`go mod verify` are run in `make check`.

## Consequences

- Note rendering is a pure function of the note text; it needs no database or
  network access and is covered by unit tests for `V-1`–`V-4` plus an
  end-to-end HTTP test that a `<script>` and inline HTML never reach the page.
- The added modules bring a small transitive tree (`gorilla/css`,
  `aymerick/douceur`), all permissive.
- Because sanitizing is separate from parsing, correctness of the security
  boundary is easy to test in isolation.
- Rendered HTML is produced on every detail view; it is cheap at this scale but
  is not cached.

## Alternatives considered

- **`bluemonday` alone on raw input**: cannot produce Markdown structure.
- **`gomarkdown/markdown`**: comparable renderer, but goldmark is the one the
  project's stack table names and is faster and more actively developed.
- **Rendering raw HTML and trusting the template escaping**: `html/template`
  escapes text, not already-built HTML, so this would defeat `V-2`.
- **An external Markdown service or client-side renderer**: violates `I-8`
  (no external calls) and weakens the CSP.

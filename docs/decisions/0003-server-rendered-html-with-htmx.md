# 0003. Server-rendered HTML with vendored HTMX

**Status:** Accepted
**Date:** 2026-10-24

## Context

Trove is used occasionally, from a browser, on a trusted local network. It has
a small set of screens (list, detail, form, tags, trash, duplicates, export).
The application must keep working without JavaScript where possible, must not
contact any external origin (`I-8`), and must satisfy a strict
Content-Security-Policy with no inline scripts or styles.

## Decision

Render pages on the server with `html/template`, embedded in the binary, and
use **HTMX** for partial updates. HTMX is **vendored** into the repository
(pinned version, license recorded when added) and served from the application's
own origin.

- Every action is a normal link or form submission first; HTMX only avoids a
  full page reload.
- Filter and sort state lives in the URL so views are bookmarkable and the
  back/forward buttons behave.
- HTMX is configured for strict CSP: no `eval`, no inline indicator styles.
- No front-end build step, no package manager, no CDN.

## Consequences

- The whole UI is inspectable in Go templates; there is no client-side state
  store to keep in sync with the server.
- Progressive enhancement keeps the application usable if a page's script
  fails, and makes HTTP tests meaningful.
- Each interaction returns an HTML fragment; handlers must produce both full
  pages and fragments from the same templates.
- The CSP stays strict, which rules out convenience features that need inline
  script, inline style or third-party assets.
- Vendoring means a manual, recorded upgrade step when a new HTMX version is
  taken.

## Alternatives considered

- **A single-page application framework**: better for highly interactive UIs,
  but adds a build toolchain, a large dependency tree and a second source of
  truth for routing, for a UI that is mostly lists and forms.
- **Plain HTML with full page reloads**: simplest and fully functional, but
  noticeably slower for common actions such as toggling a favorite or adding a
  tag filter. HTMX is a small, local enhancement on top of exactly that
  baseline.
- **Unpoly or Alpine.js**: comparable, but HTMX's attribute-driven model maps
  well onto server-rendered fragments and has no build step.

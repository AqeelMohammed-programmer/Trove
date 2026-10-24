# Security

This page describes the threat model and the controls that protect a Trove
instance. The rules being implemented are in [`../spec.md`](../spec.md) section
5; `AGENTS.md` section 7 is the engineering checklist. This page tracks what is
actually in place.

## Threat model

Trove is a single-user application on a trusted local network, reached from a
browser. The realistic threats are:

| Threat | Impact | Primary controls |
|---|---|---|
| Someone on the network finds the port | Reads and changes everything | Password (`U-1`, `U-2`), mandatory session on every route (`U-3`) |
| Password guessing | Account takeover | Argon2id hashing (`U-8`), throttling and blocks (`U-9`) |
| Malicious content in a note | Script execution in the app's origin | Markdown with raw HTML disabled, sanitizing (`V-2`), strict CSP |
| Uploaded HTML or SVG opened in the app's origin | Script execution | Downloads forced as attachments with `nosniff` |
| CSRF from another site | Unwanted state changes | `SameSite=Lax` cookies, `Origin`/`Sec-Fetch-Site` checks, tokens |
| A tampered export file | Corrupt or forged data | Full archive validation before import (`E-6`), all-or-nothing import (`E-7`) |
| Path traversal through a file name | Reading or writing outside the data directory | Files stored by attachment id; names are metadata only |
| Data leakage to third parties | Privacy loss | No outbound requests (`I-8`), no CDNs, no telemetry |

Deployment-level concerns (TLS, reverse proxy hardening, host security) are
outside the spec (`U-11`). The operations guide explains the `Secure` cookie
switch for HTTPS deployments.

## Controls

### In place at M0

| Control | Where | Notes |
|---|---|---|
| Strict Content-Security-Policy | `internal/web/server.go` | `default-src 'self'`, no inline script or style, `frame-ancestors 'none'`, `base-uri 'none'`, `form-action 'self'`, `object-src 'none'`. |
| `X-Content-Type-Options: nosniff` | same | Every response, including 404s. |
| `X-Frame-Options: DENY`, `frame-ancestors 'none'` | same | Clickjacking defence for older browsers. |
| `Referrer-Policy: same-origin` | same | No referrer leakage to other sites. |
| `Cross-Origin-Opener-Policy: same-origin` | same | Isolates the browsing context. |
| Minimal unauthenticated surface | `internal/web/server.go` | Only `GET /healthz`, which returns `ok` and no data. |
| Private data files | `internal/store/store.go` | Database created with `0600`; data directory created with `0700`. |
| No outbound network calls | whole binary | No HTTP client exists in the application; dependencies are vendored or compiled in. |
| No secrets in logs | `cmd/server` | The configuration contains no secrets; password values are never logged (M1 will extend this). |
| Pure-Go SQLite | `internal/store` | No CGO, no shelling out to a `sqlite3` binary. |

### Planned

| Control | Milestone | Rules |
|---|---|---|
| First-run setup, login, logout, change password | M1 | `U-1`, `U-2`, `U-4`, `U-6`, `U-7` |
| Argon2id with stored parameters, constant-time comparison | M1 | `U-8` |
| Server-side sessions; `HttpOnly`, `SameSite=Lax`, no expiry cookie | M1 | `U-3`, `U-5` |
| Login throttling: 5 failures, 60 s block, doubling to 15 min | M1 | `U-9` |
| Session regeneration on login; end all sessions on password change and reset | M1 | `U-4`, `U-7`, `U-10` |
| CSRF protection on every state-changing request | M1 | AGENTS.md section 7 |
| Route-protection test enumerating every registered route | M1 | `U-3` |
| Markdown rendering with raw HTML disabled, then sanitizing | M2 | `V-1`–`V-4` |
| Output auto-escaping in all templates; no trusted user HTML | M2 | AGENTS.md section 7 |
| Request body caps for ordinary forms; streaming with no cap for uploads | M2/M3 | `N-1`, AGENTS.md section 7 |
| `Content-Disposition: attachment` downloads with encoded names | M3 | `A-7` |
| Link rendering only for `http`/`https`, `rel="noopener noreferrer"` | M3 | `A-2` |
| ZIP-slip protection and SHA-256 verification on import | M8 | `E-5`, `E-6`, `E-7` |
| Password reset only as a local CLI subcommand, ending sessions | M1 | `U-10` |
| Orphaned blob cleanup that stays inside the data directory | M5/M9 | `X-12` |

## Notes on specific decisions

- **Cookie lifetime.** The session cookie has no `Expires` or `Max-Age`, so it
  dies with the browser (`U-5`). `Secure` is enabled by configuration when the
  instance is behind HTTPS.
- **CSRF.** `SameSite=Lax` alone is not treated as sufficient. The
  implementation will also verify the request origin for state-changing
  methods and reject GET requests that would change state.
- **Password reset.** Implemented as `trove reset-password`, a subcommand of
  the same binary that prompts on the terminal. It is never routed over HTTP
  ([ADR 0004](../decisions/0004-password-reset-as-cli-command.md)).
- **Import.** Validation happens fully before any write; the import then runs
  in one transaction with a staging directory, so a failure leaves the
  repository empty (`E-7`).

## Related pages

- [Overview](overview.md)
- [Storage](storage.md)
- [Operations guide](../guides/operations.md)
- [ADR 0004: password reset](../decisions/0004-password-reset-as-cli-command.md)

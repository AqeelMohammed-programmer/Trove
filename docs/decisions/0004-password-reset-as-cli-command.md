# 0004. Password reset as a local CLI subcommand

**Status:** Accepted
**Date:** 2026-10-24

## Context

There is exactly one password and no username. The spec forbids in-app
recovery (`U-10`): a forgotten password can only be reset by an action on the
machine that runs Trove. A reset must not change stored data and should end all
sessions. The application is one binary, so the reset mechanism should not need
a separate tool or a database client.

## Decision

Implement the reset as a subcommand of the same binary:

```sh
./bin/trove reset-password
```

- It prompts twice on the terminal (password and confirmation) with echo
  disabled; it never accepts the password as a command-line argument or
  environment variable, which would leak through process listings.
- It enforces the same minimum length as setup (`U-2`, 8 characters) and uses
  the same Argon2id parameters as normal password changes.
- It updates only the password hash and its parameters, and ends every session.
- It is only reachable by executing the binary locally; **no HTTP route** is
  registered for it.
- It is documented in the operations guide and the startup help.

The command arrives with M1, together with authentication.

## Consequences

- Recovery requires shell access to the host, which matches the trust model of
  a personal instance on a local network.
- No reset tokens, email or recovery questions exist, so there is no
  additional attack surface.
- The command must be tested as a real process or through an injected
  terminal, including the "repository not set up yet" case.
- Operators need a documented way onto the host; the operations guide covers
  it.

## Alternatives considered

- **A reset flag on startup** (`-reset-password`): works, but conflates
  starting the server with an administrative action and risks accidental use
  with the wrong data directory. A subcommand reads more clearly and can
  refuse to run while the server is up.
- **A separate helper binary**: duplicates configuration and database access
  for one rare action.
- **Resetting by deleting the database**: destroys data, which `U-10`
  explicitly forbids.
- **An HTTP reset endpoint protected by a local-only check**: unnecessary
  network surface and fragile to proxy misconfiguration.

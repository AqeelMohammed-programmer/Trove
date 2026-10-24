# Getting started

This guide takes you from a fresh checkout to a running Trove instance and a
green test suite in about five minutes. It assumes Linux or macOS.

## What you need

- **Go 1.26 or newer.** Check with `go version`.
- **GNU Make.** Check with `make --version`.
- `git`, if you are cloning rather than copying the folder.

No Node.js, no C compiler, no database server.

## 1. Get the code

```sh
cd /path/to/Trove
```

All commands below run from the repository root.

## 2. Install the pinned developer tools

```sh
make tools
```

This installs `goimports` and `golangci-lint` into `.tools/bin/` (git-ignored)
at the versions pinned in the `Makefile`. Nothing is installed system-wide.

## 3. Run the tests

```sh
make test
```

Expected: all packages pass, with the race detector on. The SQLite and
migration tests use real temporary databases, not mocks.

## 4. Start the server

```sh
make run
```

This creates a throwaway data directory, prints its path, and starts the server
on `127.0.0.1:8080`. In another terminal:

```sh
curl -i http://127.0.0.1:8080/healthz
```

Expected: `200 OK`, body `ok`, and the security headers described in
[security](../architecture/security.md).

Stop it with `Ctrl+C`; the server shuts down gracefully.

Open <http://127.0.0.1:8080/> and complete first-run password setup. The item
collection arrives with M2; see the [milestone table](../README.md#milestones).

## 5. Build the binary

```sh
make build
./bin/trove -h
```

The build injects the version, commit and build date from git.

## 6. Run everything CI would run

```sh
make check
```

This is the gate: formatting check, `go vet`, `golangci-lint`, tests with
`-race`, the build, and the documentation link check.

## Where to go next

- [Development guide](development.md) — workflow and conventions.
- [Testing guide](testing.md) — test layers and how to add tests.
- [Operations guide](operations.md) — configuration, backup, upgrades.
- [Architecture overview](../architecture/overview.md) — how the code fits together.

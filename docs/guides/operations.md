# Operations guide

This page covers running a Trove instance: configuration, the data directory,
backup and restore, upgrades, logs and the password reset. It describes what
exists at M0 and marks features that arrive with later milestones.

## Configuration

Configuration comes from command-line flags and environment variables. Flags
win over environment variables, which win over defaults. Everything is
validated at startup, and a bad value stops the process with a clear message.

| Setting | Flag | Environment variable | Default |
|---|---|---|---|
| Listen address | `-listen` | `TROVE_LISTEN_ADDR` | `127.0.0.1:8080` |
| Data directory | `-data-dir` | `TROVE_DATA_DIR` | `data` |
| Secure session cookie | `-secure-cookie` | `TROVE_SECURE_COOKIE` | `false` |

Examples:

```sh
# Local development, ephemeral data
./bin/trove -data-dir /tmp/trove-data

# Long-running instance, HTTPS behind a reverse proxy
TROVE_DATA_DIR=/var/lib/trove \
TROVE_LISTEN_ADDR=127.0.0.1:8080 \
TROVE_SECURE_COOKIE=true \
./bin/trove
```

`-secure-cookie=true` only makes sense when the browser reaches Trove over
HTTPS. Setting it on plain HTTP makes login impossible because the browser
will not send the cookie back.

## Data directory

The data directory holds the SQLite database and, from M3, copied files. It is
created with mode `0700` if missing, and the database file with `0600`. See
[storage](../architecture/storage.md) for the layout.

If you create the directory yourself, use `install -d -m 700` so the mode is
correct before the server starts.

## Backups

Stop the server, then copy the whole data directory:

```sh
systemctl stop trove          # or Ctrl+C
cp -a /var/lib/trove /var/backups/trove-$(date +%F)
systemctl start trove
```

For an online backup, use SQLite's backup mechanism instead of copying a live
database:

```sh
sqlite3 /var/lib/trove/repository.sqlite ".backup '/var/backups/trove.sqlite'"
```

The M8 export produces a portable ZIP of the whole repository and is the
recommended long-term archive once it exists.

## Restore

1. Stop the server.
2. Move the current data directory aside.
3. Restore the backup directory into place with the original ownership and
   mode (directory `0700`, files `0600`).
4. Start the server. Migrations run automatically and bring an older schema
   forward.

There is no down-migration: keep a backup from before an upgrade.

## Upgrades

Replace the binary and restart. On startup the server applies any new
migrations inside transactions. If a migration fails, the process exits with an
error and the database stays at the previous version.

Builds inject version information. Check it with:

```sh
./bin/trove -h    # usage
```

The startup log line includes the build string, the listen address and the data
directory (never secrets).

## Logs

Logs are structured JSON on standard output, one line per request plus startup
and shutdown events. They contain method, path, status, byte count and
duration; they never contain passwords, hashes or session tokens. Ship them
with your service manager (`journalctl -u trove`, for example).

## Password reset

The application has no in-app password recovery (`U-10`). The reset is a
subcommand of the same binary, run on the machine that hosts Trove:

```sh
./bin/trove reset-password
```

It prompts twice on the terminal without echoing, updates the hash, ends all
sessions and touches no other data. It never accepts the password as an
argument, so it cannot leak through a process list. It is never reachable over
HTTP.

## Health check

```sh
curl -fsS http://127.0.0.1:8080/healthz
```

Returns `ok` when the process is up. It reveals no configuration or data, so it
is safe to expose to a local supervisor.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `listen address ...: port must be a number` | Bad `-listen` value; use `host:port`. |
| `address already in use` | Another process holds the port; change `-listen`. |
| Login loop with `-secure-cookie=true` on HTTP | Enable it only behind HTTPS. |
| Permission denied creating the data directory | The service user cannot write to the parent directory. |

## Related pages

- [Getting started](getting-started.md)
- [Storage](../architecture/storage.md)
- [Security](../architecture/security.md)

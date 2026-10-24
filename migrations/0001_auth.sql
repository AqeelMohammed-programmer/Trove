-- 0001_auth: single-password authentication state.
--
-- The password itself is never stored, only an Argon2id hash with its
-- parameters (U-8). Sessions are server-side and hold only the SHA-256 of the
-- cookie token, so a database leak does not hand over live sessions.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    csrf_token TEXT NOT NULL,
    created_at TEXT NOT NULL
) STRICT;

-- Login throttling state per client address (U-9).
CREATE TABLE login_throttle (
    client        TEXT PRIMARY KEY,
    failures      INTEGER NOT NULL DEFAULT 0,
    blocked_until TEXT
) STRICT;

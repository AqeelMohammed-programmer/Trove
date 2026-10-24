-- 0002_items: items, tags, and the attachment table they own.
--
-- Attachments are created here rather than in a later migration so the list
-- filters (has a link / has a file) have a stable schema from the start. The
-- attachment UI arrives in M3.

CREATE TABLE items (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    note        TEXT NOT NULL DEFAULT '',
    favorite    INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1)),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    trashed_at  TEXT
) STRICT;

CREATE INDEX items_active_created ON items (trashed_at, created_at DESC);
CREATE INDEX items_active_updated ON items (trashed_at, updated_at DESC);
CREATE INDEX items_active_title ON items (trashed_at, title COLLATE NOCASE);

CREATE TABLE tags (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    -- Lower-cased name: uniqueness is case-insensitive (T-4) and the original
    -- spelling stays in name.
    name_folded TEXT NOT NULL UNIQUE,
    created_at  TEXT NOT NULL
) STRICT;

CREATE TABLE item_tags (
    item_id TEXT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    tag_id  TEXT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
) STRICT;

CREATE INDEX item_tags_tag ON item_tags (tag_id, item_id);

CREATE TABLE attachments (
    id             TEXT PRIMARY KEY,
    item_id        TEXT NOT NULL REFERENCES items (id) ON DELETE CASCADE,
    kind           TEXT NOT NULL CHECK (kind IN ('link', 'file')),
    label          TEXT NOT NULL DEFAULT '',
    url            TEXT,
    normalized_url TEXT,
    file_name      TEXT,
    size_bytes     INTEGER,
    media_type     TEXT NOT NULL DEFAULT '',
    sha256         TEXT,
    created_at     TEXT NOT NULL,
    state          TEXT NOT NULL CHECK (state IN ('active', 'trashed_individually', 'trashed_with_item')),
    trashed_at     TEXT,
    CHECK (
        (kind = 'link' AND url IS NOT NULL AND normalized_url IS NOT NULL
            AND file_name IS NULL AND size_bytes IS NULL AND sha256 IS NULL)
        OR
        (kind = 'file' AND file_name IS NOT NULL AND size_bytes IS NOT NULL
            AND sha256 IS NOT NULL AND url IS NULL AND normalized_url IS NULL)
    )
) STRICT;

CREATE INDEX attachments_item ON attachments (item_id, created_at);
CREATE INDEX attachments_sha ON attachments (sha256);
CREATE INDEX attachments_normalized_url ON attachments (normalized_url);
CREATE INDEX attachments_state ON attachments (state, trashed_at);

-- 0003_search: full-text search over title, description, note and tags.
--
-- The index holds its own copy of the searchable text and shares the items
-- table's rowid, so a row can be refreshed or removed by rowid. It is updated
-- inside the same transaction as the item change (S-6).
--
-- The default unicode61 tokenizer folds case and removes diacritics, giving
-- case-insensitive matching (S-2). The searchable text excludes URLs, labels
-- and file names on purpose (S-1).

CREATE VIRTUAL TABLE items_fts USING fts5 (
    title,
    description,
    note,
    tags,
    tokenize = 'unicode61 remove_diacritics 2'
);

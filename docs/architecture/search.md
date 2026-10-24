# Search

Trove uses SQLite's FTS5 extension. This page records how the index is built
and, most importantly, how user input is turned into a safe FTS query. The
behaviour itself is specified by `S-1` to `S-6` in [`../spec.md`](../spec.md).

## Why FTS5

It ships with SQLite, needs no extra service, and supports prefix matching,
case folding and bm25 relevance ranking — everything `S-2` and `S-3` need. The
driver proof is recorded in
[ADR 0002](../decisions/0002-sqlite-and-fts5.md).

## Index shape (M4)

An FTS5 table over the four searchable fields (`S-1`):

```sql
CREATE VIRTUAL TABLE attachments_fts USING fts5(
    title,
    description,
    note,
    tags,
    tokenize = 'unicode61 remove_diacritics 2'
);
```

- Link URLs, attachment labels and file names are **not** indexed (`S-1`).
- Markdown is stripped before indexing the note so formatting symbols do not
  affect matching (`S-4`).
- The index is updated in the same transaction as the item change, so a saved
  change is searchable immediately (`S-6`).

Keeping the index in sync means every write path (create, edit, trash,
restore, permanent delete, import, tag rename/merge/delete) updates it. The
service layer owns that; handlers never touch SQL.

## Building a query

User input is never passed to `MATCH` as-is. The algorithm:

1. Trim and split the input on whitespace.
2. Drop empty tokens.
3. Escape embedded double quotes by doubling them.
4. Wrap each token in double quotes and append `*` for prefix matching.
5. Join tokens with a space (FTS5 treats that as AND, which is `S-2`).
6. Bind the result as a parameter, never concatenate it into SQL.

Example: input `rust  own"ership` becomes:

```text
"rust"* "own""ership"*
```

This makes FTS operators typed by the user (`OR`, `NEAR`, `-`, `*`, `:`,
parentheses) ordinary text instead of syntax. A security regression test seeds
queries such as `"`, `*`, `OR`, `NEAR(a b)` and a 10,000-character token to
prove there is no syntax injection or error.

## Ranking

Results are ordered by `bm25` with per-column weights that put title matches
first (`S-3`):

```sql
ORDER BY bm25(attachments_fts, 10.0, 3.0, 1.0, 5.0)
```

The weights are a starting point and may be tuned with the M9 performance
check. When the user picks another sort order (`B-2`), that order replaces
relevance.

## Combining with filters

The FTS match is a subquery joined to `items`, so the same query applies the
tag filters, favorites filter, attachment-kind filters and trash exclusion
(`B-3`, `B-6`, `S-5`). All filters combine with AND.

## Current state (M0)

Only the driver proof exists: `TestFTS5_AvailableAndUsable` in
`internal/store/fts5_test.go` proves that FTS5 can be created, that prefix and
case-insensitive matching work, and that bm25 ranks a title match above a note
match. There is no index, no query builder and no search endpoint yet.

## Related pages

- [Data model](data-model.md)
- [Performance](performance.md)
- [ADR 0002](../decisions/0002-sqlite-and-fts5.md)

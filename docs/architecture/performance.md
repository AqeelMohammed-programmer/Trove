# Performance

This page records the performance target, how it is measured, and the results.
The requirement is `N-2` in [`../spec.md`](../spec.md): listing, filtering and
searching should respond in under one second with at least 100,000 items on
ordinary hardware.

## Target

| Operation | Target | Rule |
|---|---|---|
| List / filter / sort 100,000 items | < 1 s per page of 50 | `N-2`, `B-7` |
| Search 100,000 items | < 1 s per page of 50 | `N-2`, `S-2` |
| Save a change and find it by search immediately | Same request cycle | `S-6` |

## How it is measured

`make bench` runs Go benchmarks with no normal tests:

```sh
make bench
```

The M9 benchmark seeds a temporary database with 100,000 items (mixed
attachments, tags and trash states), then measures:

- the default list (page 1 and a deep page),
- each sort order,
- a tag filter and the combined filters,
- single-word and multi-word prefix searches,
- the duplicate report.

Numbers are reported below with the hardware and the commit they were measured
on. A regression that pushes any operation over one second is a bug.

## Design choices that protect the target

- Pagination with a fixed page size (default 50) so the browser never receives
  a whole collection (`B-7`).
- Indexes for every sort and filter listed in the
  [data model](data-model.md#indexes).
- FTS5 with prefix matching for search, so no full table scan is needed.
- The duplicate report is computed on demand with indexed lookups, not stored
  (`R-1`).
- File content never enters the database, so item queries stay small
  ([storage](storage.md)).

## Current results (M0)

No measurements yet: there is no item schema and no benchmark. The toolchain
supports benchmarks from M0 (`make bench` passes with "no benchmarks"), and the
first real fixture arrives with M2.

Hardware used for future measurements will be recorded here together with the
Go version and commit.

## Related pages

- [Search](search.md)
- [Data model](data-model.md)
- [Testing guide](../guides/testing.md)

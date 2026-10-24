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

## Results (M9)

`make bench` seeds a temporary database with 100,000 items (mixed attachments,
tags, favorites and some trashed) and measures reads. A representative run:

| Benchmark | Rule | Time per query | Target |
|---|---|---|---|
| Default list, page 1 | `N-2`, `B-1` | ~12 ms | < 1 s |
| Default list, page 1000 | `N-2`, `B-7` | ~40 ms | < 1 s |
| Tag filter | `N-2`, `B-3` | ~91 ms | < 1 s |
| Search, single prefix word | `N-2`, `S-2` | ~11 ms | < 1 s |
| Search, two prefix words | `N-2`, `S-2` | ~149 ms | < 1 s |

Every operation is comfortably under the one-second target. The tag filter and
the multi-word search are the slowest because they combine a join with ranking;
both remain well inside budget.

Measured on 13th Gen Intel Core i7-13700HX, `go1.26.8`, commit `b2e17b4`.

Run it yourself:

```sh
make bench
```

The numbers include nothing from seeding: the timer starts after the fixture is
built.

## Limits (N-1)

The specification sets no maximum number of items or file size. The
implementation adds only practical, documented bounds:

- Ordinary form bodies are capped at 1 MiB; file uploads are **not** size-capped
  and stream to disk.
- Multipart import buffers at most 1 MiB in memory before spilling to a
  temporary file; the archive is not size-capped.
- One request may add at most 1000 files at once; this bounds a pathological
  form, not file size.

There is no limit on the number of items, tags or attachments.

## Related pages

- [Search](search.md)
- [Data model](data-model.md)
- [Testing guide](../guides/testing.md)

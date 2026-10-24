package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"Trove/internal/domain"
	"Trove/migrations"
)

// benchItemCount is the collection size required by N-2.
const benchItemCount = 100_000

// newBenchStore returns a migrated Store in a temporary directory.
func newBenchStore(b *testing.B) *Store {
	b.Helper()
	st, err := Open(context.Background(), b.TempDir()+"/bench.sqlite")
	if err != nil {
		b.Fatalf("Open(): %v", err)
	}
	b.Cleanup(func() { _ = st.Close() })
	migrator := &Migrator{DB: st.DB(), FS: migrations.FS, Dir: "."}
	if err := migrator.Up(context.Background()); err != nil {
		b.Fatalf("migrating: %v", err)
	}
	return st
}

// seedLargeRepo inserts n items with mixed tags, favorites and attachments,
// plus some trashed items, in one transaction. It is deliberately direct SQL:
// the benchmark measures reads, not the write path.
func seedLargeRepo(b *testing.B, st *Store, n int) {
	b.Helper()
	ctx := context.Background()

	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	tagNames := []string{"reading", "rust", "go", "sqlite", "notes", "papers", "work", "ideas", "archive", "later"}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatalf("BeginTx(): %v", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	itemStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO items (id, title, description, note, favorite, created_at, updated_at, trashed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		b.Fatalf("prepare item: %v", err)
	}
	defer itemStmt.Close() //nolint:errcheck // statement
	tagStmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO tags (id, name, name_folded, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		b.Fatalf("prepare tag: %v", err)
	}
	defer tagStmt.Close() //nolint:errcheck // statement
	linkStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO item_tags (item_id, tag_id) VALUES (?, ?)`)
	if err != nil {
		b.Fatalf("prepare item_tags: %v", err)
	}
	defer linkStmt.Close() //nolint:errcheck // statement
	attStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO attachments (id, item_id, kind, label, url, normalized_url, file_name, size_bytes,
		    media_type, sha256, created_at, state, trashed_at)
		 VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, 'active', NULL)`)
	if err != nil {
		b.Fatalf("prepare attachment: %v", err)
	}
	defer attStmt.Close() //nolint:errcheck // statement

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("item-%08d", i)
		title := fmt.Sprintf("Item %d about topic%d", i, i%1000)
		description := fmt.Sprintf("description number %d with keywords alpha%d beta", i, i%500)
		note := fmt.Sprintf("# Note %d\n\nsome body text gamma%d delta", i, i%700)
		favorite := 0
		if i%7 == 0 {
			favorite = 1
		}
		var trashedAt any
		if i%13 == 0 {
			trashedAt = formatTime(now)
		}
		created := now.Add(-time.Duration(i) * time.Minute)
		if _, err := itemStmt.ExecContext(ctx, id, title, description, note, favorite,
			formatTime(created), formatTime(created), trashedAt); err != nil {
			b.Fatalf("insert item: %v", err)
		}

		// Two tags per item (rotating).
		for k := 0; k < 2; k++ {
			name := tagNames[(i+k)%len(tagNames)]
			tagID := "tag-" + name
			if _, err := tagStmt.ExecContext(ctx, tagID, name, domain.TagKey(name), formatTime(now)); err != nil {
				b.Fatalf("insert tag: %v", err)
			}
			if _, err := linkStmt.ExecContext(ctx, id, tagID); err != nil {
				b.Fatalf("link tag: %v", err)
			}
		}

		// Every third item has a link; every fifth has a file.
		if i%3 == 0 {
			if _, err := attStmt.ExecContext(ctx, "link-"+id, id,
				"link", "https://example.org/"+id, "https://example.org/"+id,
				nil, nil, "", nil, formatTime(now)); err != nil {
				b.Fatalf("insert link: %v", err)
			}
		}
		if i%5 == 0 {
			if _, err := attStmt.ExecContext(ctx, "file-"+id, id,
				"file", nil, nil, "file-"+id+".bin", int64(i), "application/octet-stream",
				fmt.Sprintf("%064d", i), formatTime(now)); err != nil {
				b.Fatalf("insert file: %v", err)
			}
		}

		if err := syncSearchIndex(ctx, tx, id, title, description, note,
			[]string{tagNames[i%len(tagNames)], tagNames[(i+1)%len(tagNames)]}); err != nil {
			b.Fatalf("index: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
}

func benchFilter() domain.ListFilter {
	return domain.ListFilter{Page: 1, PageSize: 50, Sort: domain.SortNewest}
}

func BenchmarkListFirstPage_N2(b *testing.B) {
	st := newBenchStore(b)
	seedLargeRepo(b, st, benchItemCount)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.ListItems(ctx, benchFilter()); err != nil {
			b.Fatalf("ListItems(): %v", err)
		}
	}
}

func BenchmarkListDeepPage_N2(b *testing.B) {
	st := newBenchStore(b)
	seedLargeRepo(b, st, benchItemCount)
	ctx := context.Background()
	filter := benchFilter()
	filter.Page = 1000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.ListItems(ctx, filter); err != nil {
			b.Fatalf("ListItems(): %v", err)
		}
	}
}

func BenchmarkListTagFilter_N2(b *testing.B) {
	st := newBenchStore(b)
	seedLargeRepo(b, st, benchItemCount)
	ctx := context.Background()
	filter := benchFilter()
	filter.Tags = []string{"reading"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.ListItems(ctx, filter); err != nil {
			b.Fatalf("ListItems(): %v", err)
		}
	}
}

func BenchmarkSearchSingleWord_N2(b *testing.B) {
	st := newBenchStore(b)
	seedLargeRepo(b, st, benchItemCount)
	ctx := context.Background()
	filter := benchFilter()
	filter.Search = "topic42"
	filter.Sort = domain.SortRelevance
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.ListItems(ctx, filter); err != nil {
			b.Fatalf("ListItems(): %v", err)
		}
	}
}

func BenchmarkSearchMultiWord_N2(b *testing.B) {
	st := newBenchStore(b)
	seedLargeRepo(b, st, benchItemCount)
	ctx := context.Background()
	filter := benchFilter()
	filter.Search = "item alpha"
	filter.Sort = domain.SortRelevance
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.ListItems(ctx, filter); err != nil {
			b.Fatalf("ListItems(): %v", err)
		}
	}
}

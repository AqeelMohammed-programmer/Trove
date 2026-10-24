package store

import (
	"context"
	"testing"
	"time"

	"Trove/internal/domain"
)

func seedAttachment(t *testing.T, st *Store, itemID string, a domain.Attachment) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	a.CreatedAt = now
	if a.State == "" {
		a.State = domain.StateActive
	}

	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx(): %v", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO items (id, title, description, note, favorite, created_at, updated_at, trashed_at)
		 VALUES (?, ?, '', '', 0, ?, ?, NULL)`,
		itemID, itemID, formatTime(now), formatTime(now)); err != nil {
		t.Fatalf("inserting item %s: %v", itemID, err)
	}
	if err := insertAttachments(ctx, tx, itemID, []domain.Attachment{a}); err != nil {
		t.Fatalf("inserting attachment: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("committing seed: %v", err)
	}
}

func TestDuplicateMatchesForItem_D1_D2_D4(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	seedAttachment(t, st, "item", domain.Attachment{ID: "a", Kind: domain.KindLink, URL: "https://example.org/x", NormalizedURL: "https://example.org/x"})
	// Another item with the same exact URL (D-1).
	seedAttachment(t, st, "other", domain.Attachment{ID: "b", Kind: domain.KindLink, URL: "https://example.org/x", NormalizedURL: "https://example.org/x"})
	// A file duplicate (D-2).
	seedAttachment(t, st, "item", domain.Attachment{ID: "f1", Kind: domain.KindFile, FileName: "one", SizeBytes: 1, SHA256: "same"})
	seedAttachment(t, st, "other", domain.Attachment{ID: "f2", Kind: domain.KindFile, FileName: "two", SizeBytes: 1, SHA256: "same"})
	// A same-item duplicate (D-4).
	seedAttachment(t, st, "item", domain.Attachment{ID: "f3", Kind: domain.KindFile, FileName: "three", SizeBytes: 1, SHA256: "same"})

	matches, err := st.DuplicateMatchesForItem(ctx, "item")
	if err != nil {
		t.Fatalf("DuplicateMatchesForItem(): %v", err)
	}
	found := map[string]bool{}
	for _, m := range matches {
		found[m.Attachment.ID] = true
	}
	// b matches a (link). f2 and f3 match f1/f3 (file). f1 matches f2/f3.
	for _, id := range []string{"b", "f2", "f3", "f1"} {
		if !found[id] {
			t.Errorf("expected %s among duplicate matches: %v (D-1/D-2/D-4)", id, found)
		}
	}
}

func TestDuplicateMatches_IncludesTrashed_D5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	seedAttachment(t, st, "item", domain.Attachment{ID: "a", Kind: domain.KindLink, URL: "https://example.org/x", NormalizedURL: "https://example.org/x"})
	seedAttachment(t, st, "other", domain.Attachment{ID: "b", Kind: domain.KindLink, URL: "https://example.org/x", NormalizedURL: "https://example.org/x"})

	// Trash the other item.
	if err := st.TrashItem(ctx, "other", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	matches, err := st.DuplicateMatchesForItem(ctx, "item")
	if err != nil {
		t.Fatalf("DuplicateMatchesForItem(): %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1 including the trashed item (D-5)", len(matches))
	}
	if !matches[0].ItemTrashed {
		t.Error("trashed match is not marked as trashed (D-5)")
	}
}

func TestDuplicateReport_R1_R2_R3_R6(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	// Links that differ only by fragment and trailing slash match (R-2).
	seedAttachment(t, st, "a", domain.Attachment{ID: "l1", Kind: domain.KindLink, URL: "https://example.org/page#one", NormalizedURL: "https://example.org/page"})
	seedAttachment(t, st, "b", domain.Attachment{ID: "l2", Kind: domain.KindLink, URL: "https://example.org/page/", NormalizedURL: "https://example.org/page"})
	// A unique attachment must not appear.
	seedAttachment(t, st, "c", domain.Attachment{ID: "u", Kind: domain.KindLink, URL: "https://other.example", NormalizedURL: "https://other.example"})
	// Files by SHA-256.
	seedAttachment(t, st, "a", domain.Attachment{ID: "f1", Kind: domain.KindFile, FileName: "one", SizeBytes: 1, SHA256: "digest"})
	seedAttachment(t, st, "b", domain.Attachment{ID: "f2", Kind: domain.KindFile, FileName: "two", SizeBytes: 1, SHA256: "digest"})

	groups, err := st.DuplicateReport(ctx)
	if err != nil {
		t.Fatalf("DuplicateReport(): %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2 (R-2)", len(groups))
	}
	for _, group := range groups {
		if len(group.Entries) < 2 {
			t.Errorf("group %q has %d entries, want >= 2 (R-2)", group.Key, len(group.Entries))
		}
	}
}

func TestDuplicateReport_IncludesTrashed_R3(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	seedAttachment(t, st, "a", domain.Attachment{ID: "l1", Kind: domain.KindLink, URL: "https://example.org/x", NormalizedURL: "https://example.org/x"})
	seedAttachment(t, st, "b", domain.Attachment{ID: "l2", Kind: domain.KindLink, URL: "https://example.org/x", NormalizedURL: "https://example.org/x"})
	if err := st.TrashItem(ctx, "b", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	groups, err := st.DuplicateReport(ctx)
	if err != nil {
		t.Fatalf("DuplicateReport(): %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	trashed := false
	for _, entry := range groups[0].Entries {
		if entry.ItemTrashed {
			trashed = true
		}
	}
	if !trashed {
		t.Error("trashed entry is not marked in the report (R-3)")
	}
}

func TestDuplicateReport_EmptyWhenNoDuplicates_R6(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	seedAttachment(t, st, "a", domain.Attachment{ID: "l1", Kind: domain.KindLink, URL: "https://one.example", NormalizedURL: "https://one.example"})
	seedAttachment(t, st, "b", domain.Attachment{ID: "l2", Kind: domain.KindLink, URL: "https://two.example", NormalizedURL: "https://two.example"})

	groups, err := st.DuplicateReport(ctx)
	if err != nil {
		t.Fatalf("DuplicateReport(): %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("groups = %d, want 0 (R-6)", len(groups))
	}
}

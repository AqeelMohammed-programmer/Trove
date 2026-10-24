package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"Trove/internal/domain"
)

func seedTaggedItem(t *testing.T, st *Store, id string, tagNames []string) domain.Item {
	t.Helper()
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	item := domain.Item{ID: id, Title: id, CreatedAt: now, UpdatedAt: now}
	created, err := st.CreateItem(context.Background(), item, tagNames, nil)
	if err != nil {
		t.Fatalf("CreateItem(%s): %v", id, err)
	}
	return created
}

func tagIDFor(t *testing.T, st *Store, name string) string {
	t.Helper()
	tag, err := st.TagByNameCaseInsensitive(context.Background(), name)
	if err != nil {
		t.Fatalf("TagByNameCaseInsensitive(%s): %v", name, err)
	}
	return tag.ID
}

func TestRenameTag_M2_M5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	created := seedTaggedItem(t, st, "item", []string{"Reading"})
	id := tagIDFor(t, st, "Reading")

	if err := st.RenameTag(ctx, id, "Books"); err != nil {
		t.Fatalf("RenameTag(): %v", err)
	}
	item, err := st.GetItem(ctx, "item")
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	if len(item.Tags) != 1 || item.Tags[0].Name != "Books" {
		t.Errorf("tags after rename = %+v, want Books (M-2)", item.Tags)
	}
	if !item.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("rename changed updated_at (M-5): %v -> %v", created.UpdatedAt, item.UpdatedAt)
	}
	// Search reflects the new tag name (S-6).
	result, err := st.ListItems(ctx, domain.ListFilter{Search: "Books"})
	if err != nil {
		t.Fatalf("ListItems(): %v", err)
	}
	if result.Total != 1 {
		t.Errorf("renamed tag not searchable: total = %d (S-6)", result.Total)
	}
	if old, _ := st.ListItems(ctx, domain.ListFilter{Search: "Reading"}); old.Total != 0 {
		t.Errorf("old tag name still searchable: total = %d", old.Total)
	}
}

func TestRenameTag_ConflictOtherTag_M2(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	seedTaggedItem(t, st, "a", []string{"Books"})
	seedTaggedItem(t, st, "b", []string{"Reading"})
	id := tagIDFor(t, st, "Reading")

	if err := st.RenameTag(ctx, id, "books"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("RenameTag(clash) = %v, want ErrConflict (M-2)", err)
	}
}

func TestRenameTag_ChangeOfCapitalizationAllowed_M2(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	seedTaggedItem(t, st, "a", []string{"Reading"})
	id := tagIDFor(t, st, "Reading")

	if err := st.RenameTag(ctx, id, "READING"); err != nil {
		t.Fatalf("RenameTag(recase) = %v, want nil (M-2)", err)
	}
	tag, err := st.TagByID(ctx, id)
	if err != nil {
		t.Fatalf("TagByID(): %v", err)
	}
	if tag.Name != "READING" {
		t.Errorf("name = %q, want READING", tag.Name)
	}
}

func TestMergeTags_M3_M5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	// One item has both tags; one has only source; one (trashed) has source.
	seedTaggedItem(t, st, "both", []string{"Reading", "Books"})
	created := seedTaggedItem(t, st, "only", []string{"Reading"})
	seedTaggedItem(t, st, "trashed", []string{"Reading"})
	if err := st.TrashItem(ctx, "trashed", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	source := tagIDFor(t, st, "Reading")
	target := tagIDFor(t, st, "Books")
	if err := st.MergeTags(ctx, source, target); err != nil {
		t.Fatalf("MergeTags(): %v", err)
	}

	// Source is gone.
	if _, err := st.TagByID(ctx, source); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("source tag survived the merge (M-3): %v", err)
	}
	// No duplicates on the item that had both.
	both, err := st.GetItem(ctx, "both")
	if err != nil {
		t.Fatalf("GetItem(both): %v", err)
	}
	books := 0
	for _, tag := range both.Tags {
		if tag.ID == target {
			books++
		}
	}
	if books != 1 {
		t.Errorf("target tag count on 'both' = %d, want 1 (no duplicates, M-3)", books)
	}
	// The trashed item gains the target tag too (M-3).
	trashed, err := st.GetItem(ctx, "trashed")
	if err != nil {
		t.Fatalf("GetItem(trashed): %v", err)
	}
	gained := false
	for _, tag := range trashed.Tags {
		if tag.ID == target {
			gained = true
		}
	}
	if !gained {
		t.Error("trashed item did not gain the target tag (M-3)")
	}
	// updated_at is unchanged (M-5).
	only, err := st.GetItem(ctx, "only")
	if err != nil {
		t.Fatalf("GetItem(only): %v", err)
	}
	if !only.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("merge changed updated_at (M-5): %v -> %v", created.UpdatedAt, only.UpdatedAt)
	}
}

func TestDeleteTag_M4_M5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	created := seedTaggedItem(t, st, "item", []string{"Doomed", "Kept"})
	seedTaggedItem(t, st, "trashed", []string{"Doomed"})
	if err := st.TrashItem(ctx, "trashed", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	id := tagIDFor(t, st, "Doomed")

	usage, err := st.TagUsage(ctx, id)
	if err != nil {
		t.Fatalf("TagUsage(): %v", err)
	}
	if usage != 2 {
		t.Errorf("usage = %d, want 2 (active and trashed, M-4)", usage)
	}

	if err := st.DeleteTag(ctx, id); err != nil {
		t.Fatalf("DeleteTag(): %v", err)
	}
	item, err := st.GetItem(ctx, "item")
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	for _, tag := range item.Tags {
		if tag.Name == "Doomed" {
			t.Error("tag was not removed from the item (M-4)")
		}
	}
	if !item.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("delete changed updated_at (M-5): %v -> %v", created.UpdatedAt, item.UpdatedAt)
	}
	// The item itself remains.
	if item.Title != "item" {
		t.Error("item was altered by tag deletion (M-4)")
	}
}

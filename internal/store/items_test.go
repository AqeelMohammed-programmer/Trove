package store

import (
	"context"
	"testing"
	"time"

	"Trove/internal/domain"
	"Trove/migrations"
)

// newTestStore returns a migrated Store backed by a real file.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	st, err := Open(context.Background(), t.TempDir()+"/test.sqlite")
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	migrator := &Migrator{DB: st.DB(), FS: migrations.FS, Dir: "."}
	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return st
}

func sampleItem(title string) domain.Item {
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	return domain.Item{
		ID:        title + "-id",
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestStore_CreateAndGetItem_I2(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	item := sampleItem("first")
	created, err := st.CreateItem(ctx, item, []string{"Reading", "rust"})
	if err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}
	if len(created.Tags) != 2 {
		t.Fatalf("created tags = %d, want 2", len(created.Tags))
	}

	got, err := st.GetItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	if got.Title != "first" || got.Favorite {
		t.Errorf("GetItem() = %+v, want title first and not favorite", got)
	}
	if len(got.Tags) != 2 {
		t.Errorf("GetItem() tags = %d, want 2", len(got.Tags))
	}
	if got.UpdatedAt.IsZero() || got.CreatedAt.IsZero() {
		t.Error("timestamps were not round-tripped")
	}
}

func TestStore_GetItem_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	if _, err := st.GetItem(ctx, "missing"); err == nil {
		t.Fatal("GetItem(missing) error = nil, want an error")
	}
}

func TestStore_TagReuseIsCaseInsensitive_T4(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	if _, err := st.CreateItem(ctx, sampleItem("a"), []string{"Reading"}); err != nil {
		t.Fatalf("CreateItem(a): %v", err)
	}
	// A second item uses a different capitalization; it must reuse the tag and
	// keep the first spelling.
	if _, err := st.CreateItem(ctx, sampleItem("b"), []string{"reading"}); err != nil {
		t.Fatalf("CreateItem(b): %v", err)
	}

	tags, err := st.ActiveTags(ctx)
	if err != nil {
		t.Fatalf("ActiveTags(): %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("ActiveTags() = %d tags, want 1 (T-4)", len(tags))
	}
	if tags[0].Name != "Reading" {
		t.Errorf("tag name = %q, want the first spelling %q", tags[0].Name, "Reading")
	}
	if tags[0].Count != 2 {
		t.Errorf("tag count = %d, want 2", tags[0].Count)
	}
}

func TestStore_UpdateReplacesTags_T5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	item := sampleItem("a")
	created, err := st.CreateItem(ctx, item, []string{"one", "two"})
	if err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}

	created.Title = "a renamed"
	created.Tags = nil
	created.UpdatedAt = created.UpdatedAt.Add(time.Hour)
	updated, err := st.UpdateItem(ctx, created, []string{"two", "three"})
	if err != nil {
		t.Fatalf("UpdateItem(): %v", err)
	}
	if len(updated.Tags) != 2 {
		t.Fatalf("updated tags = %d, want 2", len(updated.Tags))
	}

	// The tag "one" is now unused and must disappear (spec 8.2).
	tags, err := st.ActiveTags(ctx)
	if err != nil {
		t.Fatalf("ActiveTags(): %v", err)
	}
	names := map[string]bool{}
	for _, tag := range tags {
		names[tag.Name] = true
	}
	if names["one"] {
		t.Error("unused tag \"one\" was not removed")
	}
	if !names["two"] || !names["three"] {
		t.Errorf("active tags = %v, want two and three", names)
	}
}

func TestStore_ListItems_FiltersAndSort_B1_B2_B3(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	insert := func(id, title string, favorite bool, created time.Time) {
		item := domain.Item{ID: id, Title: title, Favorite: favorite, CreatedAt: created, UpdatedAt: created}
		if _, err := st.CreateItem(ctx, item, nil); err != nil {
			t.Fatalf("CreateItem(%s): %v", id, err)
		}
	}
	insert("a", "Alpha", false, base)
	insert("b", "Beta", true, base.Add(time.Hour))
	insert("c", "gamma", false, base.Add(2*time.Hour))

	// Newest first (B-1).
	result, err := st.ListItems(ctx, domain.ListFilter{Sort: domain.SortNewest})
	if err != nil {
		t.Fatalf("ListItems(): %v", err)
	}
	if result.Total != 3 || len(result.Items) != 3 {
		t.Fatalf("ListItems() total = %d, items = %d, want 3", result.Total, len(result.Items))
	}
	if result.Items[0].ID != "c" {
		t.Errorf("newest first = %q, want c", result.Items[0].ID)
	}

	// Title sort ignores case (B-2).
	result, err = st.ListItems(ctx, domain.ListFilter{Sort: domain.SortTitle})
	if err != nil {
		t.Fatalf("ListItems(title): %v", err)
	}
	want := []string{"a", "b", "c"} // Alpha, Beta, gamma
	for i, id := range want {
		if result.Items[i].ID != id {
			t.Errorf("title sort position %d = %q, want %q", i, result.Items[i].ID, id)
		}
	}

	// Favorites only (F-2, B-3).
	result, err = st.ListItems(ctx, domain.ListFilter{FavoritesOnly: true})
	if err != nil {
		t.Fatalf("ListItems(favorites): %v", err)
	}
	if result.Total != 1 || result.Items[0].ID != "b" {
		t.Errorf("favorites filter = %+v, want only b", result.Items)
	}
}

func TestStore_ListItems_Pagination_B7(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		item := domain.Item{
			ID:        string(rune('a' + i)),
			Title:     string(rune('a' + i)),
			CreatedAt: base.Add(time.Duration(i) * time.Hour),
			UpdatedAt: base,
		}
		if _, err := st.CreateItem(ctx, item, nil); err != nil {
			t.Fatalf("CreateItem(): %v", err)
		}
	}

	page1, err := st.ListItems(ctx, domain.ListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListItems(page1): %v", err)
	}
	if page1.Total != 5 || len(page1.Items) != 2 {
		t.Fatalf("page1 total = %d, items = %d, want 5 total, 2 items", page1.Total, len(page1.Items))
	}
	page3, err := st.ListItems(ctx, domain.ListFilter{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("ListItems(page3): %v", err)
	}
	if len(page3.Items) != 1 {
		t.Fatalf("page3 items = %d, want 1", len(page3.Items))
	}
}

func TestStore_ListItems_TagFilterRequiresAll_B4(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	item1 := domain.Item{ID: "a", Title: "A", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	item2 := domain.Item{ID: "b", Title: "B", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := st.CreateItem(ctx, item1, []string{"one", "two"}); err != nil {
		t.Fatalf("CreateItem(a): %v", err)
	}
	if _, err := st.CreateItem(ctx, item2, []string{"one"}); err != nil {
		t.Fatalf("CreateItem(b): %v", err)
	}

	result, err := st.ListItems(ctx, domain.ListFilter{Tags: []string{"one", "two"}})
	if err != nil {
		t.Fatalf("ListItems(): %v", err)
	}
	if result.Total != 1 || result.Items[0].ID != "a" {
		t.Fatalf("all-tags filter = %+v, want only a (B-4)", result.Items)
	}
}

func TestStore_SetFavorite_DoesNotTouchUpdatedAt_I5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	item := sampleItem("a")
	created, err := st.CreateItem(ctx, item, nil)
	if err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}

	if err := st.SetFavorite(ctx, created.ID, true); err != nil {
		t.Fatalf("SetFavorite(): %v", err)
	}
	got, err := st.GetItem(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	if !got.Favorite {
		t.Error("favorite flag was not set")
	}
	if !got.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("updated_at changed on favorite toggle: %v -> %v (I-5)", created.UpdatedAt, got.UpdatedAt)
	}
}

func TestStore_SuggestTags_T2(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	if _, err := st.CreateItem(ctx, sampleItem("a"), []string{"reading", "readonly", "rust"}); err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}

	got, err := st.SuggestTags(ctx, "rea", 10)
	if err != nil {
		t.Fatalf("SuggestTags(): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("SuggestTags(rea) = %v, want 2", got)
	}

	// A LIKE wildcard in the prefix must not act as a wildcard.
	got, err = st.SuggestTags(ctx, "%", 10)
	if err != nil {
		t.Fatalf("SuggestTags(%%): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("SuggestTags(%%) = %v, want none", got)
	}
}

func TestStore_ActiveTags_ExcludesTrashedOnlyTags_T6(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	item := sampleItem("a")
	if _, err := st.CreateItem(ctx, item, []string{"only-here"}); err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}

	// Simulate the item being trashed directly; the trash milestone owns the
	// service method, but the tag visibility rule is testable here (T-6).
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE items SET trashed_at = ? WHERE id = ?`, formatTime(time.Now()), item.ID); err != nil {
		t.Fatalf("trashing item: %v", err)
	}

	tags, err := st.ActiveTags(ctx)
	if err != nil {
		t.Fatalf("ActiveTags(): %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("ActiveTags() = %v, want none while the only item is trashed (T-6)", tags)
	}
}

func TestStore_DeleteTagWhenUnused(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	item := sampleItem("a")
	if _, err := st.CreateItem(ctx, item, []string{"temp"}); err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}
	if _, err := st.UpdateItem(ctx, item, nil); err != nil {
		t.Fatalf("UpdateItem(): %v", err)
	}

	var count int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM tags`).Scan(&count); err != nil {
		t.Fatalf("counting tags: %v", err)
	}
	if count != 0 {
		t.Errorf("tags remaining = %d, want 0 after the last use was removed", count)
	}
}

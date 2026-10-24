package service

import (
	"errors"
	"testing"

	"Trove/internal/domain"
)

func TestItemService_RenameTag_ValidatesName(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	if err := svc.RenameTag(t.Context(), "id", "a,b"); err == nil {
		t.Fatal("RenameTag(name with comma) error = nil, want a validation error")
	}
	if err := svc.RenameTag(t.Context(), "id", "   "); err == nil {
		t.Fatal("RenameTag(blank) error = nil, want a validation error")
	}
}

func TestItemService_MergeTags_RejectsSameAndEmpty(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	for _, tc := range [][2]string{{"a", "a"}, {"a", ""}, {"", "b"}} {
		var validation *domain.ValidationError
		err := svc.MergeTags(t.Context(), tc[0], tc[1])
		if !errors.As(err, &validation) {
			t.Errorf("MergeTags(%q, %q) error = %v, want *ValidationError", tc[0], tc[1], err)
		}
	}
}

func TestItemService_TagsForManagement_CountOrder(t *testing.T) {
	t.Parallel()

	store := newFakeItemStore()
	svc := newTestItemService(store, nil)

	svcTags := []domain.Tag{
		{ID: "1", Name: "alpha"},
		{ID: "2", Name: "beta"},
	}
	store.tags["alpha"] = svcTags[0]
	store.tags["beta"] = svcTags[1]

	tags, err := svc.TagsForManagement(t.Context(), "count")
	if err != nil {
		t.Fatalf("TagsForManagement(): %v", err)
	}
	// The fake returns no counts, but the order must still be deterministic.
	if len(tags) != 2 {
		t.Fatalf("tags = %d, want 2", len(tags))
	}
	if tags[0].Name > tags[1].Name {
		t.Errorf("count order is not name-stable for equal counts: %v", tags)
	}
}

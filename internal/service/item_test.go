package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"Trove/internal/blob"
	"Trove/internal/domain"
)

// fakeItemStore is an in-memory ItemStore for fast, I/O-free service tests.
type fakeItemStore struct {
	items     map[string]domain.Item
	tags      map[string]domain.Tag
	lastList  domain.ListFilter
	createErr error
}

func newFakeItemStore() *fakeItemStore {
	return &fakeItemStore{
		items: make(map[string]domain.Item),
		tags:  make(map[string]domain.Tag),
	}
}

func (f *fakeItemStore) CreateItem(_ context.Context, item domain.Item, tagNames []string, attachments []domain.Attachment) (domain.Item, error) {
	if f.createErr != nil {
		return domain.Item{}, f.createErr
	}
	item.Tags = f.tagsFor(tagNames)
	item.Attachments = append(item.Attachments, attachments...)
	f.items[item.ID] = item
	return item, nil
}

func (f *fakeItemStore) UpdateItem(_ context.Context, item domain.Item, tagNames []string, newAttachments []domain.Attachment) (domain.Item, error) {
	if _, ok := f.items[item.ID]; !ok {
		return domain.Item{}, domain.ErrNotFound
	}
	item.Tags = f.tagsFor(tagNames)
	item.Attachments = append(item.Attachments, newAttachments...)
	f.items[item.ID] = item
	return item, nil
}

func (f *fakeItemStore) GetItem(_ context.Context, id string) (domain.Item, error) {
	item, ok := f.items[id]
	if !ok {
		return domain.Item{}, domain.ErrNotFound
	}
	return item, nil
}

func (f *fakeItemStore) ListItems(_ context.Context, filter domain.ListFilter) (domain.ListResult, error) {
	f.lastList = filter
	return domain.ListResult{Page: filter.Page, PageSize: filter.PageSize, Total: len(f.items)}, nil
}

func (f *fakeItemStore) SetFavorite(_ context.Context, id string, favorite bool) error {
	item, ok := f.items[id]
	if !ok {
		return domain.ErrNotFound
	}
	item.Favorite = favorite
	f.items[id] = item
	return nil
}

func (f *fakeItemStore) ActiveTags(context.Context) ([]domain.TagCount, error) {
	out := make([]domain.TagCount, 0, len(f.tags))
	for _, tag := range f.tags {
		out = append(out, domain.TagCount{Tag: tag})
	}
	return out, nil
}

func (f *fakeItemStore) SuggestTags(context.Context, string, int) ([]string, error) {
	return nil, nil
}

func (f *fakeItemStore) AttachmentByID(_ context.Context, id string) (domain.Attachment, error) {
	for _, item := range f.items {
		for _, a := range item.Attachments {
			if a.ID == id {
				return a, nil
			}
		}
	}
	return domain.Attachment{}, domain.ErrNotFound
}

func (f *fakeItemStore) ItemTrashed(_ context.Context, itemID string) (bool, error) {
	item, ok := f.items[itemID]
	if !ok {
		return false, domain.ErrNotFound
	}
	return item.TrashedAt != nil, nil
}

func (f *fakeItemStore) tagsFor(names []string) []domain.Tag {
	tags := make([]domain.Tag, 0, len(names))
	for _, name := range names {
		tag, ok := f.tags[domain.TagKey(name)]
		if !ok {
			tag = domain.Tag{ID: "tag-" + domain.TagKey(name), Name: name}
			f.tags[domain.TagKey(name)] = tag
		}
		tags = append(tags, tag)
	}
	return tags
}

func newTestItemService(store ItemStore, now func() time.Time) *ItemService {
	return NewItemService(store, nil, now)
}

func TestItemService_Create_ValidatesTitle_I2(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	if _, err := svc.Create(t.Context(), ItemInput{Title: ""}); err == nil {
		t.Fatal("Create(no title, no attachments) error = nil, want a validation error")
	}

	created, err := svc.Create(t.Context(), ItemInput{
		Title: "",
		Links: []LinkInput{{URL: "https://example.org"}},
	})
	if err != nil {
		t.Fatalf("Create(titled from attachment): %v", err)
	}
	if created.Title != "https://example.org" {
		t.Errorf("title = %q, want the attachment URL (I-2)", created.Title)
	}
}

func TestItemService_Create_StoresTagsCaseInsensitively_T3_T5(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	created, err := svc.Create(t.Context(), ItemInput{
		Title: "x",
		Tags:  []string{"  Reading ", "reading", "RUST", "to  read"},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	names := make([]string, 0, len(created.Tags))
	for _, tag := range created.Tags {
		names = append(names, tag.Name)
	}
	want := []string{"Reading", "RUST", "to read"}
	if len(names) != len(want) {
		t.Fatalf("tags = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("tag %d = %q, want %q", i, names[i], want[i])
		}
	}
}

func TestItemService_Create_RejectsInvalidTag(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	if _, err := svc.Create(t.Context(), ItemInput{Title: "x", Tags: []string{"a,b"}}); err == nil {
		t.Fatal("Create(tag with comma) error = nil, want a validation error")
	}
}

func TestItemService_Update_SetsUpdatedAt_I5(t *testing.T) {
	t.Parallel()

	times := []time.Time{
		time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
	}
	calls := 0
	svc := newTestItemService(newFakeItemStore(), func() time.Time {
		t := times[calls]
		if calls < len(times)-1 {
			calls++
		}
		return t
	})

	created, err := svc.Create(t.Context(), ItemInput{Title: "x"})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if !created.CreatedAt.Equal(times[0]) {
		t.Fatalf("created_at = %v, want %v", created.CreatedAt, times[0])
	}

	updated, err := svc.Update(t.Context(), created.ID, ItemInput{Title: "x edited"})
	if err != nil {
		t.Fatalf("Update(): %v", err)
	}
	if !updated.UpdatedAt.Equal(times[1]) {
		t.Errorf("updated_at = %v, want %v (I-5)", updated.UpdatedAt, times[1])
	}
	if !updated.CreatedAt.Equal(times[0]) {
		t.Errorf("created_at changed on edit: %v", updated.CreatedAt)
	}
}

func TestItemService_Update_NotFound(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	if _, err := svc.Update(t.Context(), "missing", ItemInput{Title: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrNotFound", err)
	}
}

func TestItemService_List_DefaultsAndClampsPageSize_B7(t *testing.T) {
	t.Parallel()

	store := newFakeItemStore()
	svc := newTestItemService(store, nil)

	if _, err := svc.List(t.Context(), domain.ListFilter{}); err != nil {
		t.Fatalf("List(): %v", err)
	}
	if store.lastList.PageSize != DefaultPageSize {
		t.Errorf("default page size = %d, want %d", store.lastList.PageSize, DefaultPageSize)
	}
	if store.lastList.Sort != domain.SortNewest {
		t.Errorf("default sort = %q, want newest", store.lastList.Sort)
	}

	if _, err := svc.List(t.Context(), domain.ListFilter{PageSize: 10_000, Page: -5}); err != nil {
		t.Fatalf("List(): %v", err)
	}
	if store.lastList.PageSize > maxPageSize {
		t.Errorf("page size = %d, want it capped at %d", store.lastList.PageSize, maxPageSize)
	}
	if store.lastList.Page < 1 {
		t.Errorf("page = %d, want at least 1", store.lastList.Page)
	}
}

func TestItemService_List_DropsInvalidTagFilters(t *testing.T) {
	t.Parallel()

	store := newFakeItemStore()
	svc := newTestItemService(store, nil)
	if _, err := svc.List(t.Context(), domain.ListFilter{Tags: []string{"good", "bad,tag"}}); err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(store.lastList.Tags) != 1 || store.lastList.Tags[0] != "good" {
		t.Errorf("filter tags = %v, want only good", store.lastList.Tags)
	}
}

func TestItemService_SetFavorite_WrapsNotFound(t *testing.T) {
	t.Parallel()

	svc := newTestItemService(newFakeItemStore(), nil)
	if err := svc.SetFavorite(t.Context(), "missing", true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("SetFavorite(missing) error = %v, want ErrNotFound", err)
	}
}

func TestItemService_Create_StoresFileWithSHA256_A3_A4(t *testing.T) {
	t.Parallel()

	blobs, err := blob.New(t.TempDir())
	if err != nil {
		t.Fatalf("blob.New(): %v", err)
	}
	store := newFakeItemStore()
	svc := NewItemService(store, blobs, nil)

	content := "the exact bytes"
	created, err := svc.Create(t.Context(), ItemInput{
		Title: "with a file",
		Files: []FileInput{{FileName: "notes.txt", MediaType: "text/plain", Content: strings.NewReader(content)}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if len(created.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(created.Attachments))
	}
	a := created.Attachments[0]
	want := sha256.Sum256([]byte(content))
	if a.SHA256 != hex.EncodeToString(want[:]) {
		t.Errorf("sha256 = %q, want %q (A-4)", a.SHA256, hex.EncodeToString(want[:]))
	}
	if a.SizeBytes != int64(len(content)) {
		t.Errorf("size = %d, want %d", a.SizeBytes, len(content))
	}
	if exists, err := blobs.Exists(a.ID); err != nil || !exists {
		t.Errorf("content not stored: exists=%v err=%v", exists, err)
	}
}

func TestItemService_Create_RollsBackBlobsOnValidationError(t *testing.T) {
	t.Parallel()

	blobs, err := blob.New(t.TempDir())
	if err != nil {
		t.Fatalf("blob.New(): %v", err)
	}
	svc := NewItemService(newFakeItemStore(), blobs, nil)

	// A bad link is rejected before any file is written.
	if _, err := svc.Create(t.Context(), ItemInput{
		Title: "x",
		Links: []LinkInput{{URL: "ftp://example.org"}},
		Files: []FileInput{{FileName: "a.txt", Content: strings.NewReader("x")}},
	}); err == nil {
		t.Fatal("Create(bad link) error = nil, want a validation error")
	}
	ids, err := blobs.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("blob store is not empty after a rejected save: %v", ids)
	}
}

func TestItemService_Download_RefusesNonFile(t *testing.T) {
	t.Parallel()

	blobs, err := blob.New(t.TempDir())
	if err != nil {
		t.Fatalf("blob.New(): %v", err)
	}
	store := newFakeItemStore()
	svc := NewItemService(store, blobs, nil)

	created, err := svc.Create(t.Context(), ItemInput{
		Title: "link only",
		Links: []LinkInput{{URL: "https://example.org"}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if _, err := svc.Download(t.Context(), created.Attachments[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Download(link) error = %v, want ErrNotFound", err)
	}
}

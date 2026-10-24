package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"Trove/internal/domain"
)

func newTrashStore(t *testing.T) *Store {
	t.Helper()
	return newTestStore(t)
}

func seededItem(t *testing.T, st *Store, id string, attachments []domain.Attachment) domain.Item {
	t.Helper()
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	item := domain.Item{ID: id, Title: id, CreatedAt: now, UpdatedAt: now}
	for i := range attachments {
		attachments[i].CreatedAt = now
		if attachments[i].State == "" {
			attachments[i].State = domain.StateActive
		}
	}
	created, err := st.CreateItem(context.Background(), item, nil, attachments)
	if err != nil {
		t.Fatalf("CreateItem(%s): %v", id, err)
	}
	return created
}

func activeLink(id string) domain.Attachment {
	return domain.Attachment{ID: id, Kind: domain.KindLink, URL: "https://example.org/" + id, NormalizedURL: "https://example.org/" + id}
}

func activeFile(id string) domain.Attachment {
	return domain.Attachment{ID: id, Kind: domain.KindFile, FileName: id + ".bin", SizeBytes: 3, SHA256: "digest-" + id}
}

func attachmentState(t *testing.T, st *Store, id string) domain.AttachmentState {
	t.Helper()
	a, err := st.AttachmentByID(context.Background(), id)
	if err != nil {
		t.Fatalf("AttachmentByID(%s): %v", id, err)
	}
	return a.State
}

func TestTrashItem_X1_X13(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seedWithActive := seededItem(t, st, "item", []domain.Attachment{activeLink("a"), activeFile("b")})
	// Trash one attachment individually first (X-2).
	if err := st.TrashAttachment(ctx, "b", time.Now().UTC()); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}

	if err := st.TrashItem(ctx, "item", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	_ = seedWithActive

	if got := attachmentState(t, st, "a"); got != domain.StateTrashedWithItem {
		t.Errorf("active attachment after item trash = %q, want trashed_with_item (X-1)", got)
	}
	if got := attachmentState(t, st, "b"); got != domain.StateTrashedIndividually {
		t.Errorf("individually trashed attachment changed to %q, want it to stay (X-1)", got)
	}

	// Trashing again is not found (already trashed).
	if err := st.TrashItem(ctx, "item", time.Now().UTC()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second TrashItem() = %v, want ErrNotFound", err)
	}
}

func TestTrashItem_ExcludedFromListAndSearch_B6(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seededItem(t, st, "item", nil)

	if err := st.TrashItem(ctx, "item", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	result, err := st.ListItems(ctx, domain.ListFilter{})
	if err != nil {
		t.Fatalf("ListItems(): %v", err)
	}
	if result.Total != 0 {
		t.Errorf("trashed item still listed: total = %d (B-6)", result.Total)
	}
	searched, err := st.ListItems(ctx, domain.ListFilter{Search: "item"})
	if err != nil {
		t.Fatalf("ListItems(search): %v", err)
	}
	if searched.Total != 0 {
		t.Errorf("trashed item still searchable: total = %d (B-6)", searched.Total)
	}
}

func TestRestoreItem_X4(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seededItem(t, st, "item", []domain.Attachment{activeLink("a"), activeFile("b")})
	if err := st.TrashAttachment(ctx, "b", time.Now().UTC()); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}
	if err := st.TrashItem(ctx, "item", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	if err := st.RestoreItem(ctx, "item"); err != nil {
		t.Fatalf("RestoreItem(): %v", err)
	}
	if got := attachmentState(t, st, "a"); got != domain.StateActive {
		t.Errorf("trashed_with_item attachment after restore = %q, want active (X-4)", got)
	}
	if got := attachmentState(t, st, "b"); got != domain.StateTrashedIndividually {
		t.Errorf("individually trashed attachment after restore = %q, want it to stay (X-4)", got)
	}

	// The restored item is active again.
	item, err := st.GetItem(ctx, "item")
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	if item.TrashedAt != nil {
		t.Error("item is still trashed after restore")
	}
}

func TestRestoreAttachment_X5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seededItem(t, st, "item", []domain.Attachment{activeLink("a")})
	if err := st.TrashAttachment(ctx, "a", time.Now().UTC()); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}
	if err := st.RestoreAttachment(ctx, "a"); err != nil {
		t.Fatalf("RestoreAttachment(): %v", err)
	}
	if got := attachmentState(t, st, "a"); got != domain.StateActive {
		t.Errorf("state = %q, want active (X-5)", got)
	}
}

func TestDeleteItemPermanent_X6_X9(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seededItem(t, st, "item", []domain.Attachment{activeLink("a"), activeFile("b")})
	if err := st.TrashItem(ctx, "item", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	blobs, err := st.DeleteItemPermanent(ctx, "item")
	if err != nil {
		t.Fatalf("DeleteItemPermanent(): %v", err)
	}
	if len(blobs) != 1 || blobs[0] != "b" {
		t.Errorf("blob ids = %v, want [b] (only file content is removed, X-9)", blobs)
	}
	if _, err := st.GetItem(ctx, "item"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetItem() after delete = %v, want ErrNotFound", err)
	}
	// The FTS row must be gone too.
	result, err := st.ListItems(ctx, domain.ListFilter{Search: "item"})
	if err != nil {
		t.Fatalf("ListItems(search): %v", err)
	}
	if result.Total != 0 {
		t.Errorf("search row survived permanent deletion: total = %d", result.Total)
	}
	// Active items are never deleted permanently.
	if err := st.TrashItem(ctx, "other", time.Now().UTC()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("TrashItem(missing) = %v, want ErrNotFound", err)
	}
}

func TestDeleteAttachmentPermanent_Isolated_X7_X10(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	// Two attachments with identical content (same sha256) on different items.
	seededItem(t, st, "a", []domain.Attachment{activeFile("file1")})
	seededItem(t, st, "b", []domain.Attachment{{ID: "file2", Kind: domain.KindFile, FileName: "x.bin", SizeBytes: 3, SHA256: "digest-file1"}})

	if err := st.TrashAttachment(ctx, "file1", time.Now().UTC()); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}
	blobs, err := st.DeleteAttachmentPermanent(ctx, "file1")
	if err != nil {
		t.Fatalf("DeleteAttachmentPermanent(): %v", err)
	}
	if len(blobs) != 1 || blobs[0] != "file1" {
		t.Errorf("blob ids = %v, want [file1]", blobs)
	}
	// The other attachment with identical content is untouched (X-10).
	if _, err := st.AttachmentByID(ctx, "file2"); err != nil {
		t.Errorf("identical-content attachment was affected (X-10): %v", err)
	}
	// An active attachment cannot be permanently deleted.
	if _, err := st.DeleteAttachmentPermanent(ctx, "file2"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("DeleteAttachmentPermanent(active) = %v, want ErrNotFound", err)
	}
}

func TestEmptyTrash_X8(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seededItem(t, st, "keep", []domain.Attachment{activeLink("k")})
	seededItem(t, st, "gone", []domain.Attachment{activeFile("g")})
	if err := st.TrashItem(ctx, "gone", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	// An individually trashed attachment of an active item is also emptied.
	if err := st.TrashAttachment(ctx, "k", time.Now().UTC()); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}

	blobs, err := st.EmptyTrash(ctx)
	if err != nil {
		t.Fatalf("EmptyTrash(): %v", err)
	}
	if len(blobs) != 1 || blobs[0] != "g" {
		t.Errorf("blob ids = %v, want [g]", blobs)
	}
	if _, err := st.GetItem(ctx, "gone"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("trashed item survived empty trash: %v", err)
	}
	// The active item survives; its individually trashed attachment is gone.
	if _, err := st.GetItem(ctx, "keep"); err != nil {
		t.Errorf("active item removed by empty trash: %v", err)
	}
	if _, err := st.AttachmentByID(ctx, "k"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("individually trashed attachment survived empty trash: %v", err)
	}
}

func TestTrashListing_X3(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	seededItem(t, st, "item", []domain.Attachment{activeLink("a"), activeFile("b")})
	seededItem(t, st, "active", []domain.Attachment{activeLink("ind")})
	if err := st.TrashItem(ctx, "item", time.Now().UTC()); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	if err := st.TrashAttachment(ctx, "ind", time.Now().UTC()); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}

	items, err := st.TrashedItems(ctx)
	if err != nil {
		t.Fatalf("TrashedItems(): %v", err)
	}
	if len(items) != 1 || items[0].ID != "item" {
		t.Fatalf("TrashedItems() = %+v, want the trashed item", items)
	}
	if len(items[0].Attachments) != 2 {
		t.Errorf("trashed item attachments = %d, want all 2 shown beneath it (X-3)", len(items[0].Attachments))
	}

	attachments, err := st.TrashedAttachments(ctx)
	if err != nil {
		t.Fatalf("TrashedAttachments(): %v", err)
	}
	if len(attachments) != 1 || attachments[0].ID != "ind" || attachments[0].ItemTitle != "active" {
		t.Fatalf("TrashedAttachments() = %+v, want ind on active", attachments)
	}
}

func TestTrashItem_KeepsTags_X11(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTrashStore(t)
	now := time.Now().UTC()
	item := domain.Item{ID: "item", Title: "T", CreatedAt: now, UpdatedAt: now}
	if _, err := st.CreateItem(ctx, item, []string{"kept"}, nil); err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}
	if err := st.TrashItem(ctx, "item", now); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	// The tag is not listed while only trashed items use it (T-6).
	tags, err := st.ActiveTags(ctx)
	if err != nil {
		t.Fatalf("ActiveTags(): %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("tag shown while its only item is trashed: %v (T-6)", tags)
	}
	// After restore the tag reappears (X-11).
	if err := st.RestoreItem(ctx, "item"); err != nil {
		t.Fatalf("RestoreItem(): %v", err)
	}
	tags, err = st.ActiveTags(ctx)
	if err != nil {
		t.Fatalf("ActiveTags(): %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "kept" {
		t.Errorf("tag did not reappear after restore: %v (X-11)", tags)
	}
}

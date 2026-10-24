package service

import (
	"strings"
	"testing"

	"Trove/internal/blob"
)

func newTrashService(t *testing.T) (*ItemService, *blob.Store) {
	t.Helper()
	blobs, err := blob.New(t.TempDir())
	if err != nil {
		t.Fatalf("blob.New(): %v", err)
	}
	return NewItemService(newFakeItemStore(), blobs, nil), blobs
}

func TestItemService_ContentRemovedOnlyOnPermanentDelete_X9(t *testing.T) {
	t.Parallel()

	svc, blobs := newTrashService(t)
	ctx := t.Context()

	created, err := svc.Create(ctx, ItemInput{
		Title: "file item",
		Files: []FileInput{{FileName: "a.bin", Content: strings.NewReader("content")}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	fileID := created.Attachments[0].ID

	if err := svc.TrashItem(ctx, created.ID); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	if exists, _ := blobs.Exists(fileID); !exists {
		t.Fatal("content was removed when merely trashing the item (X-9)")
	}

	if err := svc.DeleteItemPermanent(ctx, created.ID); err != nil {
		t.Fatalf("DeleteItemPermanent(): %v", err)
	}
	if exists, _ := blobs.Exists(fileID); exists {
		t.Fatal("content was not removed on permanent deletion (X-6, X-9)")
	}
}

func TestItemService_EmptyTrashRemovesContent_X8(t *testing.T) {
	t.Parallel()

	svc, blobs := newTrashService(t)
	ctx := t.Context()

	created, err := svc.Create(ctx, ItemInput{
		Title: "file item",
		Files: []FileInput{{FileName: "a.bin", Content: strings.NewReader("content")}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	fileID := created.Attachments[0].ID

	if err := svc.TrashItem(ctx, created.ID); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}
	if err := svc.EmptyTrash(ctx); err != nil {
		t.Fatalf("EmptyTrash(): %v", err)
	}
	if exists, _ := blobs.Exists(fileID); exists {
		t.Fatal("content survived empty trash (X-8)")
	}
}

func TestItemService_CleanupOrphans_X12(t *testing.T) {
	t.Parallel()

	svc, blobs := newTrashService(t)
	ctx := t.Context()

	// A referenced file attachment.
	created, err := svc.Create(ctx, ItemInput{
		Title: "kept",
		Files: []FileInput{{FileName: "keep.bin", Content: strings.NewReader("keep")}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	keptID := created.Attachments[0].ID

	// An orphan: content with no attachment row.
	orphanID := "abcdefabcdefabcdefabcdefabcdefab"
	if _, _, err := blobs.Save(orphanID, strings.NewReader("orphan")); err != nil {
		t.Fatalf("Save(orphan): %v", err)
	}

	removed, err := svc.CleanupOrphans(ctx)
	if err != nil {
		t.Fatalf("CleanupOrphans(): %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if exists, _ := blobs.Exists(orphanID); exists {
		t.Error("orphan content was not removed (X-12)")
	}
	if exists, _ := blobs.Exists(keptID); !exists {
		t.Error("referenced content was removed (X-12)")
	}
}

func TestItemService_DeleteAttachmentPermanent_X7_X10(t *testing.T) {
	t.Parallel()

	svc, blobs := newTrashService(t)
	ctx := t.Context()

	// Two files with identical bytes, on two items.
	first, err := svc.Create(ctx, ItemInput{Title: "one", Files: []FileInput{{FileName: "a", Content: strings.NewReader("same")}}})
	if err != nil {
		t.Fatalf("Create(one): %v", err)
	}
	second, err := svc.Create(ctx, ItemInput{Title: "two", Files: []FileInput{{FileName: "b", Content: strings.NewReader("same")}}})
	if err != nil {
		t.Fatalf("Create(two): %v", err)
	}
	firstID := first.Attachments[0].ID
	secondID := second.Attachments[0].ID

	if err := svc.TrashAttachment(ctx, firstID); err != nil {
		t.Fatalf("TrashAttachment(): %v", err)
	}
	if err := svc.DeleteAttachmentPermanent(ctx, firstID); err != nil {
		t.Fatalf("DeleteAttachmentPermanent(): %v", err)
	}
	if exists, _ := blobs.Exists(firstID); exists {
		t.Error("content of the deleted attachment survived (X-7)")
	}
	if exists, _ := blobs.Exists(secondID); !exists {
		t.Error("identical-content attachment was affected (X-10)")
	}
}

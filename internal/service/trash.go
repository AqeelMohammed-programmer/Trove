package service

import (
	"context"
	"errors"
	"fmt"

	"Trove/internal/domain"
)

// TrashItem moves an item to the trash (I-7, X-1). Trashing never asks for
// confirmation (X-13).
func (s *ItemService) TrashItem(ctx context.Context, id string) error {
	if err := s.store.TrashItem(ctx, id, s.now().UTC()); err != nil {
		return wrapTrashError("trashing item", err)
	}
	return nil
}

// RestoreItem brings a trashed item back (X-4).
func (s *ItemService) RestoreItem(ctx context.Context, id string) error {
	if err := s.store.RestoreItem(ctx, id); err != nil {
		return wrapTrashError("restoring item", err)
	}
	return nil
}

// TrashAttachment moves one attachment to the trash individually (A-9, X-2).
func (s *ItemService) TrashAttachment(ctx context.Context, id string) error {
	if err := s.store.TrashAttachment(ctx, id, s.now().UTC()); err != nil {
		return wrapTrashError("trashing attachment", err)
	}
	return nil
}

// RestoreAttachment brings an individually trashed attachment back (X-5).
func (s *ItemService) RestoreAttachment(ctx context.Context, id string) error {
	if err := s.store.RestoreAttachment(ctx, id); err != nil {
		return wrapTrashError("restoring attachment", err)
	}
	return nil
}

// DeleteItemPermanent removes a trashed item and its content for good (X-6).
// Content is deleted only after the database transaction commits, so a failed
// disk delete leaves only an orphan to clean up, never a dangling row (X-9).
func (s *ItemService) DeleteItemPermanent(ctx context.Context, id string) error {
	blobs, err := s.store.DeleteItemPermanent(ctx, id)
	if err != nil {
		return wrapTrashError("deleting item", err)
	}
	s.deleteContent(blobs)
	return nil
}

// DeleteAttachmentPermanent removes one trashed attachment for good (X-7).
func (s *ItemService) DeleteAttachmentPermanent(ctx context.Context, id string) error {
	blobs, err := s.store.DeleteAttachmentPermanent(ctx, id)
	if err != nil {
		return wrapTrashError("deleting attachment", err)
	}
	s.deleteContent(blobs)
	return nil
}

// EmptyTrash permanently deletes everything in the trash (X-8). Nothing is
// removed automatically or by age.
func (s *ItemService) EmptyTrash(ctx context.Context) error {
	blobs, err := s.store.EmptyTrash(ctx)
	if err != nil {
		return fmt.Errorf("emptying trash: %w", err)
	}
	s.deleteContent(blobs)
	return nil
}

// Trash returns everything the trash view shows (X-3).
func (s *ItemService) Trash(ctx context.Context) (domain.TrashList, error) {
	items, err := s.store.TrashedItems(ctx)
	if err != nil {
		return domain.TrashList{}, fmt.Errorf("listing trashed items: %w", err)
	}
	attachments, err := s.store.TrashedAttachments(ctx)
	if err != nil {
		return domain.TrashList{}, fmt.Errorf("listing trashed attachments: %w", err)
	}
	return domain.TrashList{Items: items, Attachments: attachments}, nil
}

// CleanupOrphans removes stored content that no attachment references. It is
// safe to run at any time and never touches anything outside the content
// directory (X-12). It returns how many files were removed.
func (s *ItemService) CleanupOrphans(ctx context.Context) (int, error) {
	if s.blobs == nil {
		return 0, nil
	}
	referenced, err := s.store.AllAttachmentIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing attachments: %w", err)
	}
	known := make(map[string]struct{}, len(referenced))
	for _, id := range referenced {
		known[id] = struct{}{}
	}

	lister, ok := s.blobs.(interface{ List() ([]string, error) })
	if !ok {
		return 0, nil
	}
	stored, err := lister.List()
	if err != nil {
		return 0, fmt.Errorf("listing stored content: %w", err)
	}
	removed := 0
	for _, id := range stored {
		if _, ok := known[id]; ok {
			continue
		}
		if err := s.blobs.Delete(id); err != nil {
			return removed, fmt.Errorf("removing orphaned content %q: %w", id, err)
		}
		removed++
	}
	return removed, nil
}

// deleteContent removes blobs after a committed transaction. A failure is
// swallowed: the row is gone, the file is merely orphaned, and CleanupOrphans
// will remove it later.
func (s *ItemService) deleteContent(ids []string) {
	if s.blobs == nil {
		return
	}
	for _, id := range ids {
		_ = s.blobs.Delete(id)
	}
}

func wrapTrashError(action string, err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}

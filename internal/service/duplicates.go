package service

import (
	"context"
	"fmt"

	"Trove/internal/domain"
)

// Duplicates returns the duplicate report, computed on demand from stored data
// (R-1). It is read-only; resolving duplicates is left to the user via the
// trash (R-5).
func (s *ItemService) Duplicates(ctx context.Context) ([]domain.DuplicateGroup, error) {
	groups, err := s.store.DuplicateReport(ctx)
	if err != nil {
		return nil, fmt.Errorf("building duplicate report: %w", err)
	}
	return groups, nil
}

// ItemDuplicates returns other attachments that match this item's attachments
// by exact URL or SHA-256, including trashed ones (D-1, D-2, D-4, D-5).
func (s *ItemService) ItemDuplicates(ctx context.Context, itemID string) ([]domain.DuplicateMatch, error) {
	matches, err := s.store.DuplicateMatchesForItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("finding duplicates: %w", err)
	}
	return matches, nil
}

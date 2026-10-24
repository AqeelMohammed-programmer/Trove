package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"Trove/internal/domain"
)

// TagsForManagement lists tags shown under T-6 with their active item counts.
// The default order is alphabetical; "count" orders by usage then name (M-1).
func (s *ItemService) TagsForManagement(ctx context.Context, order string) ([]domain.TagCount, error) {
	tags, err := s.store.ActiveTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	if order == "count" {
		sort.SliceStable(tags, func(i, j int) bool {
			if tags[i].Count != tags[j].Count {
				return tags[i].Count > tags[j].Count
			}
			return tags[i].Name < tags[j].Name
		})
	}
	return tags, nil
}

// RenameTag changes a tag's name everywhere (M-2). A clash with another tag
// ignoring case is refused with a message suggesting a merge.
func (s *ItemService) RenameTag(ctx context.Context, id, newName string) error {
	name, err := domain.CleanTagName(newName)
	if err != nil {
		return err
	}
	if err := s.store.RenameTag(ctx, id, name); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			conflict, findErr := s.store.TagByNameCaseInsensitive(ctx, name)
			if findErr != nil {
				return &domain.ValidationError{Field: "name", Message: "Another tag already has that name. Merge the tags instead."}
			}
			return &domain.ValidationError{
				Field:   "name",
				Message: fmt.Sprintf("A tag named %q already exists. Merge the two tags instead.", conflict.Name),
			}
		}
		return wrapTagError("renaming tag", err)
	}
	return nil
}

// MergeTags moves every item's source tag to the target tag and removes source
// (M-3).
func (s *ItemService) MergeTags(ctx context.Context, sourceID, targetID string) error {
	if sourceID == "" || targetID == "" {
		return &domain.ValidationError{Field: "target", Message: "Choose a tag to merge into."}
	}
	if sourceID == targetID {
		return &domain.ValidationError{Field: "target", Message: "Choose a different tag to merge into."}
	}
	if err := s.store.MergeTags(ctx, sourceID, targetID); err != nil {
		return wrapTagError("merging tags", err)
	}
	return nil
}

// DeleteTag removes a tag from every item and returns how many items were
// affected (M-4).
func (s *ItemService) DeleteTag(ctx context.Context, id string) (int, error) {
	affected, err := s.store.TagUsage(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("counting tag usage: %w", err)
	}
	if err := s.store.DeleteTag(ctx, id); err != nil {
		return 0, wrapTagError("deleting tag", err)
	}
	return affected, nil
}

// TagUsage reports how many items use a tag, for the delete confirmation (M-4).
func (s *ItemService) TagUsage(ctx context.Context, id string) (int, error) {
	affected, err := s.store.TagUsage(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("counting tag usage: %w", err)
	}
	return affected, nil
}

// TagByID loads a tag for the management forms.
func (s *ItemService) TagByID(ctx context.Context, id string) (domain.Tag, error) {
	return s.store.TagByID(ctx, id)
}

func wrapTagError(action string, err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return domain.ErrNotFound
	case errors.Is(err, domain.ErrConflict):
		return &domain.ValidationError{Field: "target", Message: "That change is not allowed."}
	default:
		return fmt.Errorf("%s: %w", action, err)
	}
}

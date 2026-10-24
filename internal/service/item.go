// Package service implements Trove's use cases and the rules of the
// specification. It talks to persistence through interfaces it declares here,
// and contains no HTTP or SQL code.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"Trove/internal/domain"
)

// DefaultPageSize is the list page size from spec section 8.2.
const DefaultPageSize = 50

// maxPageSize caps a user-supplied page size.
const maxPageSize = 100

// ItemStore is the persistence contract for items and tags.
type ItemStore interface {
	CreateItem(ctx context.Context, item domain.Item, tagNames []string) (domain.Item, error)
	UpdateItem(ctx context.Context, item domain.Item, tagNames []string) (domain.Item, error)
	GetItem(ctx context.Context, id string) (domain.Item, error)
	ListItems(ctx context.Context, filter domain.ListFilter) (domain.ListResult, error)
	SetFavorite(ctx context.Context, id string, favorite bool) error
	ActiveTags(ctx context.Context) ([]domain.TagCount, error)
	SuggestTags(ctx context.Context, prefix string, limit int) ([]string, error)
}

// ItemService implements item and tag use cases.
type ItemService struct {
	store ItemStore
	now   func() time.Time
}

// NewItemService wires the service. A nil now means time.Now.
func NewItemService(store ItemStore, now func() time.Time) *ItemService {
	if now == nil {
		now = time.Now
	}
	return &ItemService{store: store, now: now}
}

// ItemInput is validated user input for creating or editing an item.
type ItemInput struct {
	Title       string
	Description string
	Note        string
	Tags        []string
	Favorite    bool
	// Attachments is used by I-2 to derive an empty title. On edit, the
	// service falls back to the item's existing active attachments when this
	// is nil.
	Attachments []domain.Attachment
}

// Create validates input and stores a new item.
func (s *ItemService) Create(ctx context.Context, in ItemInput) (domain.Item, error) {
	title, err := domain.CleanTitle(in.Title, in.Attachments)
	if err != nil {
		return domain.Item{}, err
	}
	description, err := domain.CleanDescription(in.Description)
	if err != nil {
		return domain.Item{}, err
	}
	note, err := domain.CleanNote(in.Note)
	if err != nil {
		return domain.Item{}, err
	}
	tags, err := cleanTags(in.Tags)
	if err != nil {
		return domain.Item{}, err
	}
	id, err := domain.NewID()
	if err != nil {
		return domain.Item{}, err
	}

	now := s.now().UTC()
	item := domain.Item{
		ID:          id,
		Title:       title,
		Description: description,
		Note:        note,
		Favorite:    in.Favorite,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := s.store.CreateItem(ctx, item, tags)
	if err != nil {
		return domain.Item{}, fmt.Errorf("creating item: %w", err)
	}
	return created, nil
}

// Update validates input and saves an existing active item. Attachments are
// handled by the attachment service; this call preserves them.
func (s *ItemService) Update(ctx context.Context, id string, in ItemInput) (domain.Item, error) {
	existing, err := s.store.GetItem(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	if existing.TrashedAt != nil {
		return domain.Item{}, domain.ErrNotFound
	}

	attachments := in.Attachments
	if attachments == nil {
		attachments = existing.Attachments
	}
	title, err := domain.CleanTitle(in.Title, attachments)
	if err != nil {
		return domain.Item{}, err
	}
	description, err := domain.CleanDescription(in.Description)
	if err != nil {
		return domain.Item{}, err
	}
	note, err := domain.CleanNote(in.Note)
	if err != nil {
		return domain.Item{}, err
	}
	tags, err := cleanTags(in.Tags)
	if err != nil {
		return domain.Item{}, err
	}

	existing.Title = title
	existing.Description = description
	existing.Note = note
	existing.Favorite = in.Favorite
	existing.UpdatedAt = s.now().UTC()

	updated, err := s.store.UpdateItem(ctx, existing, tags)
	if err != nil {
		return domain.Item{}, fmt.Errorf("updating item: %w", err)
	}
	return updated, nil
}

// Get returns one item, including its active attachments. Trashed items are
// returned too; the web layer decides whether to show them.
func (s *ItemService) Get(ctx context.Context, id string) (domain.Item, error) {
	return s.store.GetItem(ctx, id)
}

// List returns one page of active items with all filters applied (B-1..B-7).
func (s *ItemService) List(ctx context.Context, filter domain.ListFilter) (domain.ListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = DefaultPageSize
	}
	if filter.PageSize > maxPageSize {
		filter.PageSize = maxPageSize
	}
	if !filter.Sort.IsValid() {
		filter.Sort = domain.SortNewest
	}

	tags := make([]string, 0, len(filter.Tags))
	for _, raw := range filter.Tags {
		name, err := domain.CleanTagName(raw)
		if err != nil {
			continue // an invalid tag filter simply cannot match anything
		}
		tags = append(tags, name)
	}
	filter.Tags = tags

	result, err := s.store.ListItems(ctx, filter)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("listing items: %w", err)
	}
	return result, nil
}

// SetFavorite toggles the favorite flag and deliberately does not touch
// updated_at (I-5).
func (s *ItemService) SetFavorite(ctx context.Context, id string, favorite bool) error {
	if err := s.store.SetFavorite(ctx, id, favorite); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("setting favorite: %w", err)
	}
	return nil
}

// Tags lists tags used by active items, with counts (T-6, M-1).
func (s *ItemService) Tags(ctx context.Context) ([]domain.TagCount, error) {
	return s.store.ActiveTags(ctx)
}

// SuggestTags returns tag autocomplete suggestions (T-2).
func (s *ItemService) SuggestTags(ctx context.Context, prefix string) ([]string, error) {
	return s.store.SuggestTags(ctx, prefix, 10)
}

// cleanTags validates and de-duplicates tag names case-insensitively while
// keeping the first spelling the user typed (T-3, T-4, T-5).
func cleanTags(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, value := range raw {
		name, err := domain.CleanTagName(value)
		if err != nil {
			return nil, err
		}
		key := domain.TagKey(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

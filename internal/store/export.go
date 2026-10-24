package store

import (
	"context"
	"fmt"

	"Trove/internal/domain"
)

// AllItems returns every item, active or trashed, with its tags and all of its
// attachments in any state, ordered by creation time. It is the source for an
// export (E-1, E-2, E-4).
func (s *Store) AllItems(ctx context.Context) ([]domain.Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, note, favorite, created_at, updated_at, trashed_at
		 FROM items ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("listing all items: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	items := make([]domain.Item, 0)
	for rows.Next() {
		item, err := scanItemRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating all items: %w", err)
	}
	if err := s.loadTagsForItems(ctx, items); err != nil {
		return nil, err
	}
	for i := range items {
		attachments, err := s.loadAttachments(ctx, items[i].ID, true)
		if err != nil {
			return nil, err
		}
		items[i].Attachments = attachments
	}
	return items, nil
}

// RepositoryEmpty reports whether the repository has no items and no tags
// (E-5). Import is allowed only into an empty repository.
func (s *Store) RepositoryEmpty(ctx context.Context) (bool, error) {
	var items, tags int
	if err := s.db.QueryRowContext(ctx,
		`SELECT (SELECT COUNT(*) FROM items), (SELECT COUNT(*) FROM tags)`).Scan(&items, &tags); err != nil {
		return false, fmt.Errorf("checking repository: %w", err)
	}
	return items == 0 && tags == 0, nil
}

// ImportItems inserts a whole repository in one transaction (E-7, E-8). It is
// meant for an empty repository and preserves identifiers, timestamps, trash
// states, favorites, labels and attachment states. The caller has already
// validated the archive and written file content.
func (s *Store) ImportItems(ctx context.Context, items []domain.Item) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting import transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	for _, item := range items {
		var trashedAt any
		if item.TrashedAt != nil {
			trashedAt = formatTime(*item.TrashedAt)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO items (id, title, description, note, favorite, created_at, updated_at, trashed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID, item.Title, item.Description, item.Note, boolToInt(item.Favorite),
			formatTime(item.CreatedAt), formatTime(item.UpdatedAt), trashedAt); err != nil {
			return fmt.Errorf("importing item %q: %w", item.ID, err)
		}

		tagNames := tagNameList(item.Tags)
		tags, err := replaceTags(ctx, tx, item.ID, tagNames, item.CreatedAt)
		if err != nil {
			return err
		}
		if err := insertAttachments(ctx, tx, item.ID, item.Attachments); err != nil {
			return err
		}
		if err := syncSearchIndex(ctx, tx, item.ID, item.Title, item.Description, item.Note, tagNameList(tags)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing import: %w", err)
	}
	return nil
}

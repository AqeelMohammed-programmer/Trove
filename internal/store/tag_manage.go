package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"Trove/internal/domain"
)

// TagByID loads one tag.
func (s *Store) TagByID(ctx context.Context, id string) (domain.Tag, error) {
	var tag domain.Tag
	err := s.db.QueryRowContext(ctx, `SELECT id, name FROM tags WHERE id = ?`, id).
		Scan(&tag.ID, &tag.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Tag{}, fmt.Errorf("reading tag: %w", err)
	}
	return tag, nil
}

// TagByNameCaseInsensitive loads a tag by its folded name (T-4).
func (s *Store) TagByNameCaseInsensitive(ctx context.Context, name string) (domain.Tag, error) {
	var tag domain.Tag
	err := s.db.QueryRowContext(ctx, `SELECT id, name FROM tags WHERE name_folded = ?`, domain.TagKey(name)).
		Scan(&tag.ID, &tag.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Tag{}, fmt.Errorf("reading tag: %w", err)
	}
	return tag, nil
}

// TagUsage returns how many items (active or trashed) use a tag (M-4).
func (s *Store) TagUsage(ctx context.Context, id string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM item_tags WHERE tag_id = ?`, id).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting tag usage: %w", err)
	}
	return count, nil
}

// RenameTag changes a tag's name everywhere. A new name that matches another
// tag ignoring case is refused with domain.ErrConflict, except when it is the
// same tag with different capitalization (M-2). It does not touch updated_at
// (M-5).
func (s *Store) RenameTag(ctx context.Context, id, newName string) error {
	key := domain.TagKey(newName)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting rename transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	var existingID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name_folded = ?`, key).Scan(&existingID)
	switch {
	case err == nil && existingID != id:
		return domain.ErrConflict
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("checking tag name: %w", err)
	}

	res, err := tx.ExecContext(ctx, `UPDATE tags SET name = ? WHERE id = ?`, newName, id)
	if err != nil {
		return fmt.Errorf("renaming tag: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("checking renamed tag: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	// The new spelling changes the searchable tag text (S-6) but not
	// updated_at (M-5).
	if err := refreshSearchForTag(ctx, tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing rename: %w", err)
	}
	return nil
}

// MergeTags gives every item that had source the target tag, without
// duplicates, then removes source (M-3). updated_at is untouched (M-5).
func (s *Store) MergeTags(ctx context.Context, sourceID, targetID string) error {
	if sourceID == targetID {
		return domain.ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting merge transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	for _, id := range []string{sourceID, targetID} {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tags WHERE id = ?`, id).Scan(&exists); err != nil {
			return fmt.Errorf("checking tag: %w", err)
		}
		if exists == 0 {
			return domain.ErrNotFound
		}
	}

	// Items that will gain the target tag.
	ids, err := itemIDsForTag(ctx, tx, sourceID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_tags (item_id, tag_id)
		 SELECT item_id, ? FROM item_tags WHERE tag_id = ?`, targetID, sourceID); err != nil {
		return fmt.Errorf("moving tag links: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, sourceID); err != nil {
		return fmt.Errorf("deleting source tag: %w", err)
	}
	if err := refreshSearchForItems(ctx, tx, ids); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing merge: %w", err)
	}
	return nil
}

// DeleteTag removes a tag from every item, active or trashed, without deleting
// the items (M-4). updated_at is untouched (M-5).
func (s *Store) DeleteTag(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting delete transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	ids, err := itemIDsForTag(ctx, tx, id)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("checking deleted tag: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	if err := refreshSearchForItems(ctx, tx, ids); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing delete: %w", err)
	}
	return nil
}

func itemIDsForTag(ctx context.Context, tx *sql.Tx, tagID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT item_id FROM item_tags WHERE tag_id = ?`, tagID)
	if err != nil {
		return nil, fmt.Errorf("listing tag items: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning tag item: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tag items: %w", err)
	}
	return ids, nil
}

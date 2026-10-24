package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"Trove/internal/domain"
)

// replaceTags sets an item's tags, reusing existing tags case-insensitively
// (T-4) and removing tags that no item uses any more.
func replaceTags(ctx context.Context, tx *sql.Tx, itemID string, names []string, now time.Time) ([]domain.Tag, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_tags WHERE item_id = ?`, itemID); err != nil {
		return nil, fmt.Errorf("clearing item tags: %w", err)
	}

	seen := make(map[string]struct{}, len(names))
	tags := make([]domain.Tag, 0, len(names))
	for _, name := range names {
		key := domain.TagKey(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		var tag domain.Tag
		err := tx.QueryRowContext(ctx,
			`SELECT id, name FROM tags WHERE name_folded = ?`, key).Scan(&tag.ID, &tag.Name)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			id, idErr := domain.NewID()
			if idErr != nil {
				return nil, idErr
			}
			tag = domain.Tag{ID: id, Name: name}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO tags (id, name, name_folded, created_at) VALUES (?, ?, ?, ?)`,
				tag.ID, tag.Name, key, formatTime(now)); err != nil {
				return nil, fmt.Errorf("creating tag %q: %w", name, err)
			}
		case err != nil:
			return nil, fmt.Errorf("looking up tag %q: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO item_tags (item_id, tag_id) VALUES (?, ?)`, itemID, tag.ID); err != nil {
			return nil, fmt.Errorf("linking tag %q: %w", name, err)
		}
		tags = append(tags, tag)
	}

	// A tag that no item (active or trashed) uses disappears (spec 2.3, 8.2).
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM tags WHERE NOT EXISTS (SELECT 1 FROM item_tags it WHERE it.tag_id = tags.id)`); err != nil {
		return nil, fmt.Errorf("removing unused tags: %w", err)
	}
	return tags, nil
}

// ActiveTags lists tags used by at least one active item, with counts (T-6).
func (s *Store) ActiveTags(ctx context.Context) ([]domain.TagCount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, COUNT(DISTINCT it.item_id) AS uses
		 FROM tags t
		 JOIN item_tags it ON it.tag_id = t.id
		 JOIN items i ON i.id = it.item_id AND i.trashed_at IS NULL
		 GROUP BY t.id, t.name
		 ORDER BY t.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	tags := make([]domain.TagCount, 0)
	for rows.Next() {
		var tc domain.TagCount
		if err := rows.Scan(&tc.ID, &tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tags: %w", err)
	}
	return tags, nil
}

// SuggestTags returns active tags whose name starts with prefix (T-2).
func (s *Store) SuggestTags(ctx context.Context, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	pattern := escapeLike(domain.TagKey(strings.TrimSpace(prefix))) + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.name
		 FROM tags t
		 WHERE EXISTS (
		     SELECT 1 FROM item_tags it JOIN items i ON i.id = it.item_id
		     WHERE it.tag_id = t.id AND i.trashed_at IS NULL)
		   AND t.name_folded LIKE ? ESCAPE '\'
		 ORDER BY t.name COLLATE NOCASE
		 LIMIT ?`, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("suggesting tags: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	names := make([]string, 0, limit)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning tag suggestion: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tag suggestions: %w", err)
	}
	return names, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

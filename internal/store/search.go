package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"Trove/internal/domain"
)

// syncSearchIndex replaces the FTS row for an item with the current searchable
// text. It runs inside the item's transaction so a saved change is searchable
// immediately on the next query (S-6). Only title, description, note and tags
// are indexed (S-1).
func syncSearchIndex(ctx context.Context, tx *sql.Tx, itemID, title, description, note string, tagNames []string) error {
	var rowid int64
	if err := tx.QueryRowContext(ctx, `SELECT rowid FROM items WHERE id = ?`, itemID).Scan(&rowid); err != nil {
		if err == sql.ErrNoRows {
			return domain.ErrNotFound
		}
		return fmt.Errorf("looking up search row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM items_fts WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("clearing search row: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO items_fts (rowid, title, description, note, tags) VALUES (?, ?, ?, ?, ?)`,
		rowid, title, description, note, strings.Join(tagNames, " ")); err != nil {
		return fmt.Errorf("writing search row: %w", err)
	}
	return nil
}

// removeSearchIndex deletes the FTS row for an item. It must run before the
// item row is deleted, because it looks the rowid up in items.
func removeSearchIndex(ctx context.Context, tx *sql.Tx, itemID string) error {
	var rowid int64
	err := tx.QueryRowContext(ctx, `SELECT rowid FROM items WHERE id = ?`, itemID).Scan(&rowid)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("looking up search row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM items_fts WHERE rowid = ?`, rowid); err != nil {
		return fmt.Errorf("removing search row: %w", err)
	}
	return nil
}

// buildMatch turns user input into an FTS5 MATCH expression. Each whitespace
// token is quoted (with internal quotes doubled) and given a trailing `*` for
// prefix matching (S-2). Raw input is never used as FTS syntax (AGENTS.md
// section 6). An input with no tokens yields an empty string.
func buildMatch(query string) string {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		escaped := strings.ReplaceAll(token, `"`, `""`)
		parts = append(parts, `"`+escaped+`"*`)
	}
	// A space between terms is an implicit AND, so every word must match (S-2).
	return strings.Join(parts, " ")
}

// refreshSearchForTag re-indexes every item that uses a tag after the tag's
// spelling changed (M-2). updated_at is not touched (M-5).
func refreshSearchForTag(ctx context.Context, tx *sql.Tx, tagID string) error {
	ids, err := itemIDsForTag(ctx, tx, tagID)
	if err != nil {
		return err
	}
	return refreshSearchForItems(ctx, tx, ids)
}

// refreshSearchForItems re-reads each item's searchable text and rewrites its
// FTS row. It is used after tag changes, which alter the tags text without
// changing updated_at (M-3, M-4, M-5).
func refreshSearchForItems(ctx context.Context, tx *sql.Tx, itemIDs []string) error {
	for _, id := range itemIDs {
		var (
			title       string
			description string
			note        string
		)
		err := tx.QueryRowContext(ctx,
			`SELECT title, description, note FROM items WHERE id = ?`, id).
			Scan(&title, &description, &note)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading item for search refresh: %w", err)
		}
		tagNames, err := tagNamesForItem(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := syncSearchIndex(ctx, tx, id, title, description, note, tagNames); err != nil {
			return err
		}
	}
	return nil
}

func tagNamesForItem(ctx context.Context, tx *sql.Tx, itemID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT t.name FROM item_tags it JOIN tags t ON t.id = it.tag_id WHERE it.item_id = ?`, itemID)
	if err != nil {
		return nil, fmt.Errorf("reading item tags: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning item tag: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating item tags: %w", err)
	}
	return names, nil
}

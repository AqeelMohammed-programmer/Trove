package store

import (
	"context"
	"database/sql"
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

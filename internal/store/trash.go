package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"Trove/internal/domain"
)

// TrashItem moves an item and its currently active attachments to the trash
// (X-1). Attachments already trashed individually keep their state. The item's
// updated_at is left alone; putting something in the trash is not a content
// edit.
func (s *Store) TrashItem(ctx context.Context, id string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting trash transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	res, err := tx.ExecContext(ctx,
		`UPDATE items SET trashed_at = ? WHERE id = ? AND trashed_at IS NULL`,
		formatTime(now), id)
	if err != nil {
		return fmt.Errorf("trashing item: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("checking trashed item: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE attachments SET state = 'trashed_with_item', trashed_at = ?
		 WHERE item_id = ? AND state = 'active'`, formatTime(now), id); err != nil {
		return fmt.Errorf("trashing item attachments: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing trash: %w", err)
	}
	return nil
}

// RestoreItem makes a trashed item active again (X-4). Its trashed_with_item
// attachments become active; individually trashed attachments stay trashed.
func (s *Store) RestoreItem(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting restore transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	res, err := tx.ExecContext(ctx,
		`UPDATE items SET trashed_at = NULL WHERE id = ? AND trashed_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("restoring item: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("checking restored item: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE attachments SET state = 'active', trashed_at = NULL
		 WHERE item_id = ? AND state = 'trashed_with_item'`, id); err != nil {
		return fmt.Errorf("restoring item attachments: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing restore: %w", err)
	}
	return nil
}

// TrashAttachment moves one active attachment to the trash individually (X-2).
func (s *Store) TrashAttachment(ctx context.Context, id string, now time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE attachments SET state = 'trashed_individually', trashed_at = ?
		 WHERE id = ? AND state = 'active'`, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("trashing attachment: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("checking trashed attachment: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RestoreAttachment makes an individually trashed attachment active (X-5).
func (s *Store) RestoreAttachment(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE attachments SET state = 'active', trashed_at = NULL
		 WHERE id = ? AND state = 'trashed_individually'`, id)
	if err != nil {
		return fmt.Errorf("restoring attachment: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("checking restored attachment: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteItemPermanent removes a trashed item, every attachment in any state
// and returns the file blob ids whose content must be removed from disk (X-6,
// X-9). The caller deletes the content after this transaction commits.
func (s *Store) DeleteItemPermanent(ctx context.Context, id string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting delete transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	var exists int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM items WHERE id = ? AND trashed_at IS NOT NULL`, id).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking item: %w", err)
	}
	if exists == 0 {
		return nil, domain.ErrNotFound
	}

	blobs, err := fileBlobIDs(ctx, tx,
		`kind = 'file' AND item_id = ?`, id)
	if err != nil {
		return nil, err
	}
	if err := removeSearchIndex(ctx, tx, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("deleting item: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing delete: %w", err)
	}
	return blobs, nil
}

// DeleteAttachmentPermanent removes one trashed attachment and returns the
// blob id of its content, if it is a file (X-7). It touches no other
// attachment, even one with identical content (X-10).
func (s *Store) DeleteAttachmentPermanent(ctx context.Context, id string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting delete transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	var state domain.AttachmentState
	var kind domain.AttachmentKind
	err = tx.QueryRowContext(ctx, `SELECT state, kind FROM attachments WHERE id = ?`, id).
		Scan(&state, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading attachment: %w", err)
	}
	if state == domain.StateActive {
		// Only trashed attachments can be permanently deleted from the trash.
		return nil, domain.ErrNotFound
	}

	var blobs []string
	if kind == domain.KindFile {
		blobs = append(blobs, id)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("deleting attachment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing delete: %w", err)
	}
	return blobs, nil
}

// EmptyTrash permanently deletes every trashed item and every individually
// trashed attachment, returning the file blob ids to remove (X-8).
func (s *Store) EmptyTrash(ctx context.Context) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting empty-trash transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	// File content that belongs to attachments about to be removed:
	// everything under a trashed item, plus every individually trashed
	// attachment.
	blobs, err := fileBlobIDs(ctx, tx, `kind = 'file' AND (state = 'trashed_individually'
		OR item_id IN (SELECT id FROM items WHERE trashed_at IS NOT NULL))`)
	if err != nil {
		return nil, err
	}

	ids, err := trashedItemIDs(ctx, tx)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		if err := removeSearchIndex(ctx, tx, id); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE trashed_at IS NOT NULL`); err != nil {
		return nil, fmt.Errorf("deleting trashed items: %w", err)
	}
	// Remaining individually trashed attachments belong to active items.
	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE state = 'trashed_individually'`); err != nil {
		return nil, fmt.Errorf("deleting trashed attachments: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing empty trash: %w", err)
	}
	return blobs, nil
}

// trashedItemIDs lists the ids of every trashed item.
func trashedItemIDs(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM items WHERE trashed_at IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("listing trashed items: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning trashed item: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating trashed items: %w", err)
	}
	return ids, nil
}

// fileBlobIDs returns the ids of file attachments matching the condition. The
// condition is a trusted fragment built by this package, never user input.
func fileBlobIDs(ctx context.Context, tx *sql.Tx, condition string, args ...any) ([]string, error) {
	// #nosec G202 -- the condition is a trusted fragment chosen by the caller;
	// every value is a bound argument.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM attachments WHERE `+condition, args...)
	if err != nil {
		return nil, fmt.Errorf("listing file attachments: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning attachment id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating attachment ids: %w", err)
	}
	return ids, nil
}

// TrashedItems returns trashed items, newest deleted first, each with all of
// its attachments (X-3).
func (s *Store) TrashedItems(ctx context.Context) ([]domain.Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, note, favorite, created_at, updated_at, trashed_at
		 FROM items WHERE trashed_at IS NOT NULL
		 ORDER BY trashed_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing trashed items: %w", err)
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
		return nil, fmt.Errorf("iterating trashed items: %w", err)
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

// TrashedAttachments returns individually trashed attachments of active items
// with their owning item title, newest deleted first (X-3).
func (s *Store) TrashedAttachments(ctx context.Context) ([]domain.TrashAttachment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.item_id, a.kind, a.label, a.url, a.normalized_url, a.file_name,
		        a.size_bytes, a.media_type, a.sha256, a.created_at, a.state, a.trashed_at,
		        i.title
		 FROM attachments a JOIN items i ON i.id = a.item_id
		 WHERE a.state = 'trashed_individually' AND i.trashed_at IS NULL
		 ORDER BY a.trashed_at DESC, a.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing trashed attachments: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	out := make([]domain.TrashAttachment, 0)
	for rows.Next() {
		var (
			a          domain.TrashAttachment
			url        sql.NullString
			normalized sql.NullString
			fileName   sql.NullString
			sizeBytes  sql.NullInt64
			sha256v    sql.NullString
			created    string
			trashedAt  sql.NullString
		)
		if err := rows.Scan(&a.ID, &a.ItemID, &a.Kind, &a.Label, &url, &normalized,
			&fileName, &sizeBytes, &a.MediaType, &sha256v, &created, &a.State, &trashedAt,
			&a.ItemTitle); err != nil {
			return nil, fmt.Errorf("scanning trashed attachment: %w", err)
		}
		a.URL = url.String
		a.NormalizedURL = normalized.String
		a.FileName = fileName.String
		a.SizeBytes = sizeBytes.Int64
		a.SHA256 = sha256v.String
		if a.CreatedAt, err = parseTime(created); err != nil {
			return nil, err
		}
		if a.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating trashed attachments: %w", err)
	}
	return out, nil
}

// AllAttachmentIDs returns the id of every attachment in any state. The orphan
// cleanup compares this with the content directory (X-12).
func (s *Store) AllAttachmentIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM attachments`)
	if err != nil {
		return nil, fmt.Errorf("listing attachment ids: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning attachment id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating attachment ids: %w", err)
	}
	return ids, nil
}

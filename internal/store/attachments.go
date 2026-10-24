package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"Trove/internal/domain"
)

// insertAttachments writes attachment rows for an item inside an existing
// transaction. It is used by item create and edit so attachments are saved
// atomically with their item (I-5, spec section 6).
func insertAttachments(ctx context.Context, tx *sql.Tx, itemID string, attachments []domain.Attachment) error {
	for _, a := range attachments {
		var (
			url        any
			normalized any
			fileName   any
			sizeBytes  any
			sha256v    any
		)
		switch a.Kind {
		case domain.KindLink:
			url = a.URL
			normalized = a.NormalizedURL
		case domain.KindFile:
			fileName = a.FileName
			sizeBytes = a.SizeBytes
			sha256v = a.SHA256
		default:
			return fmt.Errorf("inserting attachment %q: unknown kind %q", a.ID, a.Kind)
		}

		state := a.State
		if state == "" {
			state = domain.StateActive
		}
		var trashedAt any
		if a.TrashedAt != nil {
			trashedAt = formatTime(*a.TrashedAt)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO attachments
			   (id, item_id, kind, label, url, normalized_url, file_name, size_bytes,
			    media_type, sha256, created_at, state, trashed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID, itemID, string(a.Kind), a.Label, url, normalized, fileName, sizeBytes,
			a.MediaType, sha256v, formatTime(a.CreatedAt), string(state), trashedAt); err != nil {
			return fmt.Errorf("inserting attachment %q: %w", a.ID, err)
		}
	}
	return nil
}

// AttachmentByID loads one attachment regardless of its state, together with
// the owning item's title and trash state. It is used by the download handler,
// which must refuse trashed items.
func (s *Store) AttachmentByID(ctx context.Context, id string) (domain.Attachment, error) {
	var (
		a             domain.Attachment
		url           sql.NullString
		normalizedURL sql.NullString
		fileName      sql.NullString
		sizeBytes     sql.NullInt64
		sha256v       sql.NullString
		created       string
		trashedAt     sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, item_id, kind, label, url, normalized_url, file_name, size_bytes,
		        media_type, sha256, created_at, state, trashed_at
		 FROM attachments WHERE id = ?`, id).
		Scan(&a.ID, &a.ItemID, &a.Kind, &a.Label, &url, &normalizedURL,
			&fileName, &sizeBytes, &a.MediaType, &sha256v, &created, &a.State, &trashedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Attachment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("reading attachment: %w", err)
	}
	a.URL = url.String
	a.NormalizedURL = normalizedURL.String
	a.FileName = fileName.String
	a.SizeBytes = sizeBytes.Int64
	a.SHA256 = sha256v.String
	if a.CreatedAt, err = parseTime(created); err != nil {
		return domain.Attachment{}, err
	}
	if a.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
		return domain.Attachment{}, err
	}
	return a, nil
}

// ItemTrashed reports whether the item with the given id is in the trash.
func (s *Store) ItemTrashed(ctx context.Context, itemID string) (bool, error) {
	var trashedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT trashed_at FROM items WHERE id = ?`, itemID).Scan(&trashedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("reading item trash state: %w", err)
	}
	return trashedAt.Valid && trashedAt.String != "", nil
}

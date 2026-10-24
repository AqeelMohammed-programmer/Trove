package store

import (
	"context"
	"database/sql"
	"fmt"

	"Trove/internal/domain"
)

// DuplicateMatchesForItem finds, for every attachment of an item, other
// attachments with the same exact URL (links) or the same SHA-256 (files). It
// includes trashed attachments and items and matches within the same item
// (D-1, D-2, D-4), so the detail view can warn at add time and later.
func (s *Store) DuplicateMatchesForItem(ctx context.Context, itemID string) ([]domain.DuplicateMatch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a2.id, a2.item_id, a2.kind, a2.label, a2.url, a2.normalized_url, a2.file_name,
		        a2.size_bytes, a2.media_type, a2.sha256, a2.created_at, a2.state, a2.trashed_at,
		        i.title, i.trashed_at
		 FROM attachments a1
		 JOIN attachments a2
		   ON a2.id <> a1.id
		  AND ((a1.kind = 'link' AND a2.kind = 'link' AND a2.url = a1.url)
		    OR (a1.kind = 'file' AND a2.kind = 'file' AND a2.sha256 = a1.sha256))
		 JOIN items i ON i.id = a2.item_id
		 WHERE a1.item_id = ?
		 ORDER BY a2.created_at, a2.id`, itemID)
	if err != nil {
		return nil, fmt.Errorf("finding duplicate matches: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	matches := make([]domain.DuplicateMatch, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		match, err := scanDuplicateMatch(rows)
		if err != nil {
			return nil, err
		}
		// A single other attachment may match several of this item's
		// attachments; report it once.
		if _, ok := seen[match.Attachment.ID]; ok {
			continue
		}
		seen[match.Attachment.ID] = struct{}{}
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating duplicate matches: %w", err)
	}
	return matches, nil
}

// DuplicateReport groups attachments that match under the report rules (R-1,
// R-2): links by normalized URL, files by SHA-256. Trashed attachments and
// items are included (R-3). Only groups of two or more are returned.
func (s *Store) DuplicateReport(ctx context.Context) ([]domain.DuplicateGroup, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.item_id, a.kind, a.label, a.url, a.normalized_url, a.file_name,
		        a.size_bytes, a.media_type, a.sha256, a.created_at, a.state, a.trashed_at,
		        i.title, i.trashed_at
		 FROM attachments a JOIN items i ON i.id = a.item_id
		 ORDER BY a.kind, a.normalized_url, a.sha256, a.created_at, a.id`)
	if err != nil {
		return nil, fmt.Errorf("reading attachments for report: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	groups := make(map[string]*domain.DuplicateGroup)
	order := make([]string, 0)
	for rows.Next() {
		entry, kind, key, err := scanReportEntry(rows)
		if err != nil {
			return nil, err
		}
		if key == "" {
			continue // nothing to compare on
		}
		groupKey := string(kind) + "\x00" + key
		group, ok := groups[groupKey]
		if !ok {
			group = &domain.DuplicateGroup{Kind: kind, Key: key}
			groups[groupKey] = group
			order = append(order, groupKey)
		}
		group.Entries = append(group.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating report rows: %w", err)
	}

	result := make([]domain.DuplicateGroup, 0)
	for _, key := range order {
		if group := groups[key]; len(group.Entries) >= 2 {
			result = append(result, *group)
		}
	}
	return result, nil
}

func scanDuplicateMatch(row rowScanner) (domain.DuplicateMatch, error) {
	var (
		m             domain.DuplicateMatch
		url           sql.NullString
		normalizedURL sql.NullString
		fileName      sql.NullString
		sizeBytes     sql.NullInt64
		sha256v       sql.NullString
		created       string
		trashedAt     sql.NullString
		itemTrashedAt sql.NullString
	)
	if err := row.Scan(&m.Attachment.ID, &m.Attachment.ItemID, &m.Attachment.Kind, &m.Attachment.Label,
		&url, &normalizedURL, &fileName, &sizeBytes, &m.Attachment.MediaType, &sha256v,
		&created, &m.Attachment.State, &trashedAt, &m.ItemTitle, &itemTrashedAt); err != nil {
		return domain.DuplicateMatch{}, fmt.Errorf("scanning duplicate match: %w", err)
	}
	fillAttachment(&m.Attachment, url, normalizedURL, fileName, sizeBytes, sha256v)
	createdAt, err := parseTime(created)
	if err != nil {
		return domain.DuplicateMatch{}, err
	}
	m.Attachment.CreatedAt = createdAt
	if m.Attachment.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
		return domain.DuplicateMatch{}, err
	}
	m.ItemTrashed = itemTrashedAt.Valid && itemTrashedAt.String != ""
	m.AttachmentTrashed = m.Attachment.State != domain.StateActive
	m.ItemID = m.Attachment.ItemID
	return m, nil
}

func scanReportEntry(row rowScanner) (domain.DuplicateEntry, domain.AttachmentKind, string, error) {
	var (
		entry         domain.DuplicateEntry
		kind          domain.AttachmentKind
		url           sql.NullString
		normalizedURL sql.NullString
		fileName      sql.NullString
		sizeBytes     sql.NullInt64
		sha256v       sql.NullString
		created       string
		trashedAt     sql.NullString
		itemTrashedAt sql.NullString
	)
	if err := row.Scan(&entry.Attachment.ID, &entry.Attachment.ItemID, &kind, &entry.Attachment.Label,
		&url, &normalizedURL, &fileName, &sizeBytes, &entry.Attachment.MediaType, &sha256v,
		&created, &entry.Attachment.State, &trashedAt, &entry.ItemTitle, &itemTrashedAt); err != nil {
		return domain.DuplicateEntry{}, "", "", fmt.Errorf("scanning report row: %w", err)
	}
	fillAttachment(&entry.Attachment, url, normalizedURL, fileName, sizeBytes, sha256v)
	entry.Attachment.Kind = kind
	createdAt, err := parseTime(created)
	if err != nil {
		return domain.DuplicateEntry{}, "", "", err
	}
	entry.Attachment.CreatedAt = createdAt
	if entry.Attachment.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
		return domain.DuplicateEntry{}, "", "", err
	}
	entry.ItemTrashed = itemTrashedAt.Valid && itemTrashedAt.String != ""
	entry.ItemID = entry.Attachment.ItemID
	entry.AttachmentTrashed = entry.Attachment.State != domain.StateActive

	key := ""
	switch kind {
	case domain.KindLink:
		key = entry.Attachment.NormalizedURL
	case domain.KindFile:
		key = entry.Attachment.SHA256
	}
	return entry, kind, key, nil
}

func fillAttachment(a *domain.Attachment, url, normalizedURL, fileName sql.NullString, sizeBytes sql.NullInt64, sha256v sql.NullString) {
	a.URL = url.String
	a.NormalizedURL = normalizedURL.String
	a.FileName = fileName.String
	a.SizeBytes = sizeBytes.Int64
	a.SHA256 = sha256v.String
}

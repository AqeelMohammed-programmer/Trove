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

// CreateItem inserts an item, its attachments and its tags in one transaction
// (I-1, spec section 6).
func (s *Store) CreateItem(ctx context.Context, item domain.Item, tagNames []string, attachments []domain.Attachment) (domain.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Item{}, fmt.Errorf("starting item transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	_, err = tx.ExecContext(ctx,
		`INSERT INTO items (id, title, description, note, favorite, created_at, updated_at, trashed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
		item.ID, item.Title, item.Description, item.Note, boolToInt(item.Favorite),
		formatTime(item.CreatedAt), formatTime(item.UpdatedAt))
	if err != nil {
		return domain.Item{}, fmt.Errorf("inserting item: %w", err)
	}

	tags, err := replaceTags(ctx, tx, item.ID, tagNames, item.CreatedAt)
	if err != nil {
		return domain.Item{}, err
	}
	if err := insertAttachments(ctx, tx, item.ID, attachments); err != nil {
		return domain.Item{}, err
	}
	if err := syncSearchIndex(ctx, tx, item.ID, item.Title, item.Description, item.Note, tagNameList(tags)); err != nil {
		return domain.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Item{}, fmt.Errorf("committing item: %w", err)
	}
	item.Tags = tags
	item.Attachments = attachments
	return item, nil
}

// UpdateItem saves the editable fields, replaces the tag set and appends any
// new attachments in one transaction (I-5, I-6).
func (s *Store) UpdateItem(ctx context.Context, item domain.Item, tagNames []string, newAttachments []domain.Attachment) (domain.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Item{}, fmt.Errorf("starting item transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	res, err := tx.ExecContext(ctx,
		`UPDATE items
		 SET title = ?, description = ?, note = ?, favorite = ?, updated_at = ?
		 WHERE id = ? AND trashed_at IS NULL`,
		item.Title, item.Description, item.Note, boolToInt(item.Favorite),
		formatTime(item.UpdatedAt), item.ID)
	if err != nil {
		return domain.Item{}, fmt.Errorf("updating item: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.Item{}, fmt.Errorf("checking updated item: %w", err)
	}
	if affected == 0 {
		return domain.Item{}, domain.ErrNotFound
	}

	tags, err := replaceTags(ctx, tx, item.ID, tagNames, item.UpdatedAt)
	if err != nil {
		return domain.Item{}, err
	}
	if err := insertAttachments(ctx, tx, item.ID, newAttachments); err != nil {
		return domain.Item{}, err
	}
	if err := syncSearchIndex(ctx, tx, item.ID, item.Title, item.Description, item.Note, tagNameList(tags)); err != nil {
		return domain.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Item{}, fmt.Errorf("committing item: %w", err)
	}
	item.Tags = tags
	return item, nil
}

// GetItem loads an item with its tags and active attachments.
func (s *Store) GetItem(ctx context.Context, id string) (domain.Item, error) {
	var (
		item      domain.Item
		favorite  int
		created   string
		updated   string
		trashedAt sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, note, favorite, created_at, updated_at, trashed_at
		 FROM items WHERE id = ?`, id).
		Scan(&item.ID, &item.Title, &item.Description, &item.Note, &favorite, &created, &updated, &trashedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Item{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Item{}, fmt.Errorf("reading item: %w", err)
	}
	item.Favorite = favorite != 0
	if item.CreatedAt, err = parseTime(created); err != nil {
		return domain.Item{}, err
	}
	if item.UpdatedAt, err = parseTime(updated); err != nil {
		return domain.Item{}, err
	}
	if item.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
		return domain.Item{}, err
	}

	tags, err := s.itemTags(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	item.Tags = tags

	attachments, err := s.itemAttachments(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	item.Attachments = attachments
	for _, a := range attachments {
		switch a.Kind {
		case domain.KindLink:
			item.Counts.Links++
		case domain.KindFile:
			item.Counts.Files++
		}
	}
	return item, nil
}

// ListItems runs a filtered, sorted, paginated browse query (B-1..B-7).
func (s *Store) ListItems(ctx context.Context, filter domain.ListFilter) (domain.ListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 50
	}

	where, args := buildItemWhere(filter)

	match := buildMatch(filter.Search)
	from := "items i"
	order := orderBy(filter.Sort)
	if match != "" {
		from += ` JOIN items_fts ON items_fts.rowid = i.rowid`
		where += ` AND items_fts MATCH ?`
		args = append(args, match)
		if filter.Sort == domain.SortRelevance {
			// bm25: lower is better; title weighted highest (S-3).
			order = `bm25(items_fts, 10.0, 1.0, 1.0, 2.0), i.created_at DESC, i.id DESC`
		}
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+from+` WHERE `+where, args...).Scan(&total); err != nil {
		return domain.ListResult{}, fmt.Errorf("counting items: %w", err)
	}

	// where holds only column names and ? placeholders; orderBy returns a
	// constant. No user input is concatenated: every value is a bound arg.
	columns := `SELECT i.id, i.title, i.description, i.note, i.favorite, ` +
		`i.created_at, i.updated_at, i.trashed_at FROM ` // #nosec G202 -- assembly of trusted fragments
	query := columns + from + ` WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?` // #nosec G202 -- trusted fragments
	queryArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("listing items: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	items := make([]domain.Item, 0, filter.PageSize)
	for rows.Next() {
		item, err := scanItemRow(rows)
		if err != nil {
			return domain.ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("iterating items: %w", err)
	}

	if err := s.loadTagsForItems(ctx, items); err != nil {
		return domain.ListResult{}, err
	}
	if err := s.loadCountsForItems(ctx, items); err != nil {
		return domain.ListResult{}, err
	}
	return domain.ListResult{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// SetFavorite toggles the flag without touching updated_at (I-5).
func (s *Store) SetFavorite(ctx context.Context, id string, favorite bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE items SET favorite = ? WHERE id = ? AND trashed_at IS NULL`, boolToInt(favorite), id)
	if err != nil {
		return fmt.Errorf("setting favorite: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking favorite update: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func buildItemWhere(filter domain.ListFilter) (string, []any) {
	conditions := []string{"i.trashed_at IS NULL"}
	var args []any

	if filter.FavoritesOnly {
		conditions = append(conditions, "i.favorite = 1")
	}
	if filter.HasLink {
		conditions = append(conditions,
			"EXISTS (SELECT 1 FROM attachments a WHERE a.item_id = i.id AND a.kind = 'link' AND a.state = 'active')")
	}
	if filter.HasFile {
		conditions = append(conditions,
			"EXISTS (SELECT 1 FROM attachments a WHERE a.item_id = i.id AND a.kind = 'file' AND a.state = 'active')")
	}
	for _, tag := range filter.Tags {
		conditions = append(conditions,
			`EXISTS (SELECT 1 FROM item_tags it JOIN tags t ON t.id = it.tag_id
			         WHERE it.item_id = i.id AND t.name_folded = ?)`)
		args = append(args, domain.TagKey(tag))
	}
	return strings.Join(conditions, " AND "), args
}

func orderBy(sort domain.SortOrder) string {
	switch sort {
	case domain.SortUpdated:
		return "i.updated_at DESC, i.id DESC"
	case domain.SortTitle:
		return "i.title COLLATE NOCASE ASC, i.id ASC"
	case domain.SortNewest, domain.SortRelevance:
		// Relevance falls back to newest until the FTS index exists (M4).
		return "i.created_at DESC, i.id DESC"
	default:
		return "i.created_at DESC, i.id DESC"
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItemRow(row rowScanner) (domain.Item, error) {
	var (
		item      domain.Item
		favorite  int
		created   string
		updated   string
		trashedAt sql.NullString
	)
	if err := row.Scan(&item.ID, &item.Title, &item.Description, &item.Note,
		&favorite, &created, &updated, &trashedAt); err != nil {
		return domain.Item{}, fmt.Errorf("scanning item row: %w", err)
	}
	item.Favorite = favorite != 0
	var err error
	if item.CreatedAt, err = parseTime(created); err != nil {
		return domain.Item{}, err
	}
	if item.UpdatedAt, err = parseTime(updated); err != nil {
		return domain.Item{}, err
	}
	if item.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
		return domain.Item{}, err
	}
	return item, nil
}

func (s *Store) loadTagsForItems(ctx context.Context, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}
	byID := make(map[string]*domain.Item, len(items))
	args := make([]any, 0, len(items))
	for i := range items {
		byID[items[i].ID] = &items[i]
		args = append(args, items[i].ID)
	}
	query := `SELECT it.item_id, t.id, t.name FROM item_tags it JOIN tags t ON t.id = it.tag_id ` +
		`WHERE it.item_id IN (` + placeholders(len(items)) + `) ORDER BY t.name COLLATE NOCASE` // #nosec G202 -- placeholders is only "?,?,?"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("loading item tags: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	for rows.Next() {
		var itemID string
		var tag domain.Tag
		if err := rows.Scan(&itemID, &tag.ID, &tag.Name); err != nil {
			return fmt.Errorf("scanning item tag: %w", err)
		}
		if item, ok := byID[itemID]; ok {
			item.Tags = append(item.Tags, tag)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating item tags: %w", err)
	}
	return nil
}

func (s *Store) loadCountsForItems(ctx context.Context, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}
	byID := make(map[string]*domain.Item, len(items))
	args := make([]any, 0, len(items))
	for i := range items {
		byID[items[i].ID] = &items[i]
		args = append(args, items[i].ID)
	}
	query := `SELECT item_id, kind, COUNT(*) FROM attachments ` +
		`WHERE state = 'active' AND item_id IN (` + placeholders(len(items)) + `) GROUP BY item_id, kind` // #nosec G202 -- placeholders is only "?,?,?"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("loading attachment counts: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	for rows.Next() {
		var (
			itemID string
			kind   string
			count  int
		)
		if err := rows.Scan(&itemID, &kind, &count); err != nil {
			return fmt.Errorf("scanning attachment count: %w", err)
		}
		item, ok := byID[itemID]
		if !ok {
			continue
		}
		switch domain.AttachmentKind(kind) {
		case domain.KindLink:
			item.Counts.Links = count
		case domain.KindFile:
			item.Counts.Files = count
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating attachment counts: %w", err)
	}
	return nil
}

func (s *Store) itemTags(ctx context.Context, itemID string) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name FROM item_tags it JOIN tags t ON t.id = it.tag_id
		 WHERE it.item_id = ? ORDER BY t.name COLLATE NOCASE`, itemID)
	if err != nil {
		return nil, fmt.Errorf("loading tags: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	tags := make([]domain.Tag, 0)
	for rows.Next() {
		var tag domain.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tags: %w", err)
	}
	return tags, nil
}

func (s *Store) itemAttachments(ctx context.Context, itemID string) ([]domain.Attachment, error) {
	return s.loadAttachments(ctx, itemID, false)
}

// loadAttachments loads an item's attachments ordered by time added. When
// allStates is false only active attachments are returned; otherwise every
// state, which the trash views need (X-3).
func (s *Store) loadAttachments(ctx context.Context, itemID string, allStates bool) ([]domain.Attachment, error) {
	condition := " AND state = 'active'"
	if allStates {
		condition = ""
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, item_id, kind, label, url, normalized_url, file_name, size_bytes,
		        media_type, sha256, created_at, state, trashed_at
		 FROM attachments
		 WHERE item_id = ?`+condition+`
		 ORDER BY created_at, id`, itemID)
	if err != nil {
		return nil, fmt.Errorf("loading attachments: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	attachments := make([]domain.Attachment, 0)
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating attachments: %w", err)
	}
	return attachments, nil
}

func scanAttachment(row rowScanner) (domain.Attachment, error) {
	var (
		a             domain.Attachment
		url           sql.NullString
		normalizedURL sql.NullString
		fileName      sql.NullString
		sizeBytes     sql.NullInt64
		sha256        sql.NullString
		created       string
		trashedAt     sql.NullString
	)
	if err := row.Scan(&a.ID, &a.ItemID, &a.Kind, &a.Label, &url, &normalizedURL,
		&fileName, &sizeBytes, &a.MediaType, &sha256, &created, &a.State, &trashedAt); err != nil {
		return domain.Attachment{}, fmt.Errorf("scanning attachment: %w", err)
	}
	a.URL = url.String
	a.NormalizedURL = normalizedURL.String
	a.FileName = fileName.String
	a.SizeBytes = sizeBytes.Int64
	a.SHA256 = sha256.String
	var err error
	if a.CreatedAt, err = parseTime(created); err != nil {
		return domain.Attachment{}, err
	}
	if a.TrashedAt, err = parseNullableTime(trashedAt); err != nil {
		return domain.Attachment{}, err
	}
	return a, nil
}

func tagNameList(tags []domain.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func parseNullableTime(v sql.NullString) (*time.Time, error) {
	if !v.Valid || v.String == "" {
		return nil, nil
	}
	t, err := parseTime(v.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

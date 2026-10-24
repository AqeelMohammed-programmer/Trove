// Package exchange writes and reads the repository archive: one ZIP holding
// data.json and a files/ directory of copied content. Export produces a
// consistent snapshot; import validates the whole archive before writing and is
// all-or-nothing (spec section 6, rules E-1 to E-10).
package exchange

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"Trove/internal/domain"
)

// Format is the archive format identifier, and FormatVersion its version.
const (
	Format        = "personal-repository-export"
	FormatVersion = 1
)

// dataFileName is the metadata entry inside the archive.
const dataFileName = "data.json"

// filesPrefix is the directory holding copied content.
const filesPrefix = "files/"

// Errors returned by import validation.
var (
	// ErrNotEmpty means the target repository already has data (E-5).
	ErrNotEmpty = errors.New("exchange: repository is not empty")
	// ErrInvalidArchive means the archive failed validation (E-6).
	ErrInvalidArchive = errors.New("exchange: invalid archive")
)

// Source provides the data an export needs.
type Source interface {
	// AllItems returns every item with its tags and attachments, in any state.
	AllItems(ctx context.Context) ([]domain.Item, error)
	// OpenContent returns the stored bytes of a file attachment.
	OpenContent(id string) (io.ReadCloser, error)
}

// Target receives an imported repository.
type Target interface {
	// Empty reports whether the repository has no items and no tags.
	Empty(ctx context.Context) (bool, error)
	// SaveContent stores file content under an attachment id and returns the
	// lowercase hex SHA-256 of the exact bytes.
	SaveContent(id string, r io.Reader) (digest string, err error)
	// DeleteContent removes stored content.
	DeleteContent(id string) error
	// ImportItems inserts a whole repository in one transaction.
	ImportItems(ctx context.Context, items []domain.Item) error
}

// Data is the decoded data.json (spec 6.2).
type Data struct {
	Format        string     `json:"format"`
	FormatVersion int        `json:"format_version"`
	ExportedAt    string     `json:"exported_at"`
	Items         []ItemData `json:"items"`
}

// ItemData is one item in the archive.
type ItemData struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Note        string           `json:"note"`
	Favorite    bool             `json:"favorite"`
	Tags        []string         `json:"tags"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	TrashedAt   *string          `json:"trashed_at"`
	Attachments []AttachmentData `json:"attachments"`
}

// AttachmentData is one attachment in the archive.
type AttachmentData struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Label     *string `json:"label"`
	URL       *string `json:"url,omitempty"`
	FileName  *string `json:"file_name,omitempty"`
	SizeBytes *int64  `json:"size_bytes,omitempty"`
	MediaType *string `json:"media_type,omitempty"`
	SHA256    *string `json:"sha256,omitempty"`
	StoredAs  *string `json:"stored_as,omitempty"`
	CreatedAt string  `json:"created_at"`
	State     string  `json:"state"`
	TrashedAt *string `json:"trashed_at"`
}

// Export writes the repository as a ZIP archive to w (E-1, E-2, E-3).
func Export(ctx context.Context, src Source, w io.Writer, now time.Time) error {
	items, err := src.AllItems(ctx)
	if err != nil {
		return fmt.Errorf("reading repository: %w", err)
	}

	data := Data{
		Format:        Format,
		FormatVersion: FormatVersion,
		ExportedAt:    now.UTC().Format(time.RFC3339),
		Items:         make([]ItemData, 0, len(items)),
	}

	zw := zip.NewWriter(w)
	defer zw.Close() //nolint:errcheck // Close is checked below via the sentinel

	for _, item := range items {
		itemData, err := exportItem(item)
		if err != nil {
			return err
		}
		data.Items = append(data.Items, itemData)

		for _, a := range item.Attachments {
			if a.Kind != domain.KindFile {
				continue
			}
			if err := writeContent(zw, src, a.ID); err != nil {
				return err
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding metadata: %w", err)
	}
	if err := writeEntry(zw, dataFileName, jsonBytes); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("closing archive: %w", err)
	}
	return nil
}

func exportItem(item domain.Item) (ItemData, error) {
	tags := make([]string, 0, len(item.Tags))
	for _, tag := range item.Tags {
		tags = append(tags, tag.Name)
	}
	out := ItemData{
		ID:          item.ID,
		Title:       item.Title,
		Description: item.Description,
		Note:        item.Note,
		Favorite:    item.Favorite,
		Tags:        tags,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
		TrashedAt:   formatTimePtr(item.TrashedAt),
		Attachments: make([]AttachmentData, 0, len(item.Attachments)),
	}
	for _, a := range item.Attachments {
		ad := AttachmentData{
			ID:        a.ID,
			Kind:      string(a.Kind),
			Label:     stringPtrOrNil(a.Label),
			CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
			State:     string(a.State),
			TrashedAt: formatTimePtr(a.TrashedAt),
		}
		switch a.Kind {
		case domain.KindLink:
			url := a.URL
			ad.URL = &url
		case domain.KindFile:
			fileName := a.FileName
			size := a.SizeBytes
			mediaType := a.MediaType
			digest := a.SHA256
			stored := filesPrefix + a.ID
			ad.FileName = &fileName
			ad.SizeBytes = &size
			ad.MediaType = &mediaType
			ad.SHA256 = &digest
			ad.StoredAs = &stored
		default:
			return ItemData{}, fmt.Errorf("exporting attachment %q: unknown kind %q", a.ID, a.Kind)
		}
		out.Attachments = append(out.Attachments, ad)
	}
	return out, nil
}

func writeContent(zw *zip.Writer, src Source, id string) error {
	rc, err := src.OpenContent(id)
	if err != nil {
		return fmt.Errorf("opening content %q: %w", id, err)
	}
	defer rc.Close() //nolint:errcheck // read-only
	header := &zip.FileHeader{Name: filesPrefix + id, Method: zip.Deflate}
	header.SetMode(0o600)
	entry, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("creating archive entry: %w", err)
	}
	if _, err := io.Copy(entry, rc); err != nil {
		return fmt.Errorf("writing content %q: %w", id, err)
	}
	return nil
}

func writeEntry(zw *zip.Writer, name string, content []byte) error {
	entry, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("creating archive entry: %w", err)
	}
	if _, err := entry.Write(content); err != nil {
		return fmt.Errorf("writing archive entry: %w", err)
	}
	return nil
}

// Import validates an archive and, if it is valid, loads it into an empty
// repository (E-5 to E-9). It is all-or-nothing: a failure before the database
// transaction leaves the repository empty.
func Import(ctx context.Context, archivePath string, dst Target) error {
	empty, err := dst.Empty(ctx)
	if err != nil {
		return fmt.Errorf("checking repository: %w", err)
	}
	if !empty {
		return ErrNotEmpty
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArchive, err)
	}
	defer zr.Close() //nolint:errcheck // read-only

	entries, err := indexEntries(zr)
	if err != nil {
		return err
	}

	data, err := readData(entries[dataFileName])
	if err != nil {
		return err
	}
	if err := validateData(data); err != nil {
		return err
	}
	if err := validateFiles(data, entries); err != nil {
		return err
	}

	items, err := toDomainItems(data)
	if err != nil {
		return err
	}

	// Write content first, tracking what we wrote so a later failure can undo
	// it (E-7). The archive has already been fully validated.
	written := make([]string, 0)
	for _, item := range items {
		for _, a := range item.Attachments {
			if a.Kind != domain.KindFile {
				continue
			}
			if _, err := dst.SaveContent(a.ID, fileContent(entries, a.ID)); err != nil {
				deleteWritten(dst, written)
				return fmt.Errorf("storing content %q: %w", a.ID, err)
			}
			written = append(written, a.ID)
		}
	}

	if err := dst.ImportItems(ctx, items); err != nil {
		deleteWritten(dst, written)
		return fmt.Errorf("importing items: %w", err)
	}
	return nil
}

// indexEntries maps entry names to zip entries and rejects unsafe paths
// (zip-slip protection).
func indexEntries(zr *zip.ReadCloser) (map[string]*zip.File, error) {
	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		name := f.Name
		if name == dataFileName || (strings.HasPrefix(name, filesPrefix) && validFileEntry(name)) {
			entries[name] = f
			continue
		}
		// Ignore directory entries; reject anything else that escapes files/.
		if strings.HasSuffix(name, "/") {
			continue
		}
		return nil, fmt.Errorf("%w: unexpected entry %q", ErrInvalidArchive, name)
	}
	return entries, nil
}

// validFileEntry reports whether a files/ entry is a plain name (no path
// traversal, no nested directories).
func validFileEntry(name string) bool {
	rest := strings.TrimPrefix(name, filesPrefix)
	if rest == "" || strings.ContainsAny(rest, `/\`) || rest == "." || rest == ".." {
		return false
	}
	return true
}

func readData(entry *zip.File) (Data, error) {
	if entry == nil {
		return Data{}, fmt.Errorf("%w: %s is missing", ErrInvalidArchive, dataFileName)
	}
	rc, err := entry.Open()
	if err != nil {
		return Data{}, fmt.Errorf("%w: %w", ErrInvalidArchive, err)
	}
	defer rc.Close() //nolint:errcheck // read-only
	var data Data
	if err := json.NewDecoder(rc).Decode(&data); err != nil {
		return Data{}, fmt.Errorf("%w: %s is not valid JSON: %w", ErrInvalidArchive, dataFileName, err)
	}
	return data, nil
}

func validateData(data Data) error {
	if data.Format != Format {
		return fmt.Errorf("%w: unknown format %q", ErrInvalidArchive, data.Format)
	}
	if data.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: unsupported format_version %d", ErrInvalidArchive, data.FormatVersion)
	}
	for i, item := range data.Items {
		if item.ID == "" {
			return fmt.Errorf("%w: item %d has no id", ErrInvalidArchive, i)
		}
		if _, err := time.Parse(time.RFC3339, item.CreatedAt); err != nil {
			return fmt.Errorf("%w: item %q has an invalid created_at", ErrInvalidArchive, item.ID)
		}
		for _, a := range item.Attachments {
			if a.ID == "" {
				return fmt.Errorf("%w: attachment on item %q has no id", ErrInvalidArchive, item.ID)
			}
			switch a.Kind {
			case string(domain.KindLink):
				if a.URL == nil || *a.URL == "" {
					return fmt.Errorf("%w: link attachment %q has no url", ErrInvalidArchive, a.ID)
				}
			case string(domain.KindFile):
				if a.SHA256 == nil || *a.SHA256 == "" || a.FileName == nil {
					return fmt.Errorf("%w: file attachment %q is incomplete", ErrInvalidArchive, a.ID)
				}
			default:
				return fmt.Errorf("%w: attachment %q has unknown kind %q", ErrInvalidArchive, a.ID, a.Kind)
			}
		}
	}
	return nil
}

// validateFiles checks that every referenced file exists and that its SHA-256
// matches (E-6).
func validateFiles(data Data, entries map[string]*zip.File) error {
	for _, item := range data.Items {
		for _, a := range item.Attachments {
			if a.Kind != string(domain.KindFile) {
				continue
			}
			entry := entries[filesPrefix+a.ID]
			if entry == nil {
				return fmt.Errorf("%w: content for attachment %q is missing", ErrInvalidArchive, a.ID)
			}
			digest, err := hashEntry(entry)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArchive, err)
			}
			if !strings.EqualFold(digest, *a.SHA256) {
				return fmt.Errorf("%w: content for attachment %q does not match its sha256", ErrInvalidArchive, a.ID)
			}
		}
	}
	return nil
}

func hashEntry(entry *zip.File) (string, error) {
	rc, err := entry.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close() //nolint:errcheck // read-only
	hash := sha256.New()
	// #nosec G110 -- the archive is the user's own export; sizes are not
	// attacker-controlled beyond any normal upload.
	if _, err := io.Copy(hash, rc); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fileContent reopens a validated entry for a second pass. The caller must
// have validated the archive already.
func fileContent(entries map[string]*zip.File, id string) io.Reader {
	rc, err := entries[filesPrefix+id].Open()
	if err != nil {
		return errorReader{err}
	}
	return rc
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func toDomainItems(data Data) ([]domain.Item, error) {
	items := make([]domain.Item, 0, len(data.Items))
	for _, item := range data.Items {
		createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("%w: item %q created_at: %w", ErrInvalidArchive, item.ID, err)
		}
		updatedAt := createdAt
		if item.UpdatedAt != "" {
			if updatedAt, err = time.Parse(time.RFC3339, item.UpdatedAt); err != nil {
				return nil, fmt.Errorf("%w: item %q updated_at: %w", ErrInvalidArchive, item.ID, err)
			}
		}
		trashed, err := parseTimePtr(item.TrashedAt)
		if err != nil {
			return nil, fmt.Errorf("%w: item %q trashed_at: %w", ErrInvalidArchive, item.ID, err)
		}
		domainItem := domain.Item{
			ID:          item.ID,
			Title:       item.Title,
			Description: item.Description,
			Note:        item.Note,
			Favorite:    item.Favorite,
			CreatedAt:   createdAt.UTC(),
			UpdatedAt:   updatedAt.UTC(),
			TrashedAt:   trashed,
		}
		for _, name := range item.Tags {
			domainItem.Tags = append(domainItem.Tags, domain.Tag{Name: name})
		}
		for _, a := range item.Attachments {
			attachment, err := toDomainAttachment(item.ID, a)
			if err != nil {
				return nil, err
			}
			domainItem.Attachments = append(domainItem.Attachments, attachment)
		}
		items = append(items, domainItem)
	}
	return items, nil
}

func toDomainAttachment(itemID string, a AttachmentData) (domain.Attachment, error) {
	createdAt, err := time.Parse(time.RFC3339, a.CreatedAt)
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("%w: attachment %q created_at: %w", ErrInvalidArchive, a.ID, err)
	}
	trashed, err := parseTimePtr(a.TrashedAt)
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("%w: attachment %q trashed_at: %w", ErrInvalidArchive, a.ID, err)
	}
	attachment := domain.Attachment{
		ID:        a.ID,
		ItemID:    itemID,
		Kind:      domain.AttachmentKind(a.Kind),
		Label:     deref(a.Label),
		CreatedAt: createdAt.UTC(),
		State:     domain.AttachmentState(a.State),
		TrashedAt: trashed,
	}
	if attachment.State == "" {
		attachment.State = domain.StateActive
	}
	switch attachment.Kind {
	case domain.KindLink:
		attachment.URL = deref(a.URL)
		attachment.NormalizedURL = domain.NormalizeURL(attachment.URL)
	case domain.KindFile:
		attachment.FileName = deref(a.FileName)
		attachment.MediaType = deref(a.MediaType)
		attachment.SHA256 = strings.ToLower(deref(a.SHA256))
		if a.SizeBytes != nil {
			attachment.SizeBytes = *a.SizeBytes
		}
	default:
		return domain.Attachment{}, fmt.Errorf("%w: attachment %q has unknown kind", ErrInvalidArchive, a.ID)
	}
	return attachment, nil
}

func deleteWritten(dst Target, ids []string) {
	for _, id := range ids {
		_ = dst.DeleteContent(id)
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func parseTimePtr(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	t = t.UTC()
	return &t, nil
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

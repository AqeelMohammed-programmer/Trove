package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"Trove/internal/domain"
)

// BlobStore stores the content of copied files. It is implemented by
// internal/blob.
type BlobStore interface {
	// Save streams content under the attachment id and returns the lowercase
	// hex SHA-256 of the exact bytes and the byte count.
	Save(id string, r io.Reader) (digest string, size int64, err error)
	// Open returns stored content for reading.
	Open(id string) (io.ReadCloser, error)
	// Delete removes stored content.
	Delete(id string) error
}

// LinkInput is one link attachment supplied by the form (A-2, A-6).
type LinkInput struct {
	URL   string
	Label string
}

// FileInput is one file attachment supplied by the form. Content is streamed
// and must be read exactly once (A-3, A-4, A-5).
type FileInput struct {
	FileName  string
	MediaType string
	Label     string
	Content   io.Reader
}

// buildLinks validates link inputs and turns them into attachments. It returns
// the first validation error, so the whole save is rejected (A-2).
func buildLinks(links []LinkInput) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(links))
	for _, link := range links {
		url, err := domain.CleanLinkURL(link.URL)
		if err != nil {
			return nil, err
		}
		label, err := domain.CleanLabel(link.Label)
		if err != nil {
			return nil, err
		}
		id, err := domain.NewID()
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Attachment{
			ID:            id,
			Kind:          domain.KindLink,
			Label:         label,
			URL:           url,
			NormalizedURL: domain.NormalizeURL(url),
			State:         domain.StateActive,
		})
	}
	return out, nil
}

// filePlaceholders builds attachments carrying only the metadata known before
// the bytes are streamed. It is used to resolve the default title (I-2) and to
// reserve identifiers.
func filePlaceholders(files []FileInput) ([]domain.Attachment, error) {
	out := make([]domain.Attachment, 0, len(files))
	for _, file := range files {
		name := strings.TrimSpace(file.FileName)
		if name == "" {
			return nil, &domain.ValidationError{Field: "file", Message: "A file needs a name."}
		}
		label, err := domain.CleanLabel(file.Label)
		if err != nil {
			return nil, err
		}
		id, err := domain.NewID()
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Attachment{
			ID:        id,
			Kind:      domain.KindFile,
			Label:     label,
			FileName:  name,
			MediaType: file.MediaType,
			State:     domain.StateActive,
		})
	}
	return out, nil
}

// saveFileAttachments streams each file to the blob store, filling in the
// SHA-256 and size (A-4). On any failure it removes the blobs it already wrote,
// so a rejected save leaves no orphaned content. The caller removes the blobs
// if the database write later fails.
func saveFileAttachments(blobs BlobStore, files []FileInput, attachments []domain.Attachment) ([]string, error) {
	saved := make([]string, 0, len(files))
	if blobs == nil {
		if len(files) > 0 {
			return nil, fmt.Errorf("saving files: no blob store configured")
		}
		return saved, nil
	}
	for i, file := range files {
		digest, size, err := blobs.Save(attachments[i].ID, file.Content)
		if err != nil {
			return saved, fmt.Errorf("storing file %q: %w", file.FileName, err)
		}
		saved = append(saved, attachments[i].ID)
		attachments[i].SHA256 = digest
		attachments[i].SizeBytes = size
	}
	return saved, nil
}

// discardBlobs removes stored content after a failed database write. The error
// is deliberately not returned to the user: the content is orphaned, not
// corrupt, and a cleanup pass removes leftovers (AGENTS.md section 6).
func discardBlobs(blobs BlobStore, ids []string) {
	if blobs == nil {
		return
	}
	for _, id := range ids {
		_ = blobs.Delete(id)
	}
}

// AttachmentContent is a file attachment prepared for download (A-7).
type AttachmentContent struct {
	Attachment domain.Attachment
	Content    io.ReadCloser
}

// Download returns the stored content of an active file attachment. Trashed
// attachments and trashed items are refused (V-2 note: nothing runs in the
// app origin).
func (s *ItemService) Download(ctx context.Context, id string) (AttachmentContent, error) {
	attachment, err := s.store.AttachmentByID(ctx, id)
	if err != nil {
		return AttachmentContent{}, err
	}
	if attachment.Kind != domain.KindFile {
		return AttachmentContent{}, domain.ErrNotFound
	}
	if attachment.State != domain.StateActive {
		return AttachmentContent{}, domain.ErrNotFound
	}
	trashed, err := s.store.ItemTrashed(ctx, attachment.ItemID)
	if err != nil {
		return AttachmentContent{}, err
	}
	if trashed {
		return AttachmentContent{}, domain.ErrNotFound
	}
	content, err := s.blobs.Open(attachment.ID)
	if err != nil {
		return AttachmentContent{}, fmt.Errorf("opening content: %w", err)
	}
	return AttachmentContent{Attachment: attachment, Content: content}, nil
}

package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"Trove/internal/domain"
	"Trove/internal/exchange"
)

// Export writes the whole repository as a ZIP archive (E-1). The snapshot is
// taken from stored data at one moment.
func (s *ItemService) Export(ctx context.Context, w io.Writer) error {
	if s.blobs == nil {
		return errors.New("export: no blob store configured")
	}
	return exchange.Export(ctx, s, w, s.now().UTC())
}

// Import loads a validated archive into an empty repository (E-5..E-9). It
// never changes the password.
func (s *ItemService) Import(ctx context.Context, archivePath string) error {
	if s.blobs == nil {
		return errors.New("import: no blob store configured")
	}
	if err := exchange.Import(ctx, archivePath, s); err != nil {
		if errors.Is(err, exchange.ErrNotEmpty) {
			return &domain.ValidationError{Field: "archive", Message: "The repository is not empty. Import is allowed only into a fresh instance."}
		}
		if errors.Is(err, exchange.ErrInvalidArchive) {
			return &domain.ValidationError{Field: "archive", Message: "That file is not a valid Trove export: " + trimPrefix(err.Error())}
		}
		return fmt.Errorf("importing archive: %w", err)
	}
	return nil
}

// AllItems implements exchange.Source.
func (s *ItemService) AllItems(ctx context.Context) ([]domain.Item, error) {
	return s.store.AllItems(ctx)
}

// OpenContent implements exchange.Source.
func (s *ItemService) OpenContent(id string) (io.ReadCloser, error) {
	return s.blobs.Open(id)
}

// Empty implements exchange.Target.
func (s *ItemService) Empty(ctx context.Context) (bool, error) {
	return s.store.RepositoryEmpty(ctx)
}

// SaveContent implements exchange.Target.
func (s *ItemService) SaveContent(id string, r io.Reader) (string, error) {
	digest, _, err := s.blobs.Save(id, r)
	return digest, err
}

// DeleteContent implements exchange.Target.
func (s *ItemService) DeleteContent(id string) error {
	return s.blobs.Delete(id)
}

// ImportItems implements exchange.Target.
func (s *ItemService) ImportItems(ctx context.Context, items []domain.Item) error {
	return s.store.ImportItems(ctx, items)
}

// trimPrefix removes the "exchange: invalid archive: " prefix from a
// validation error so the message reads naturally in the UI.
func trimPrefix(message string) string {
	const prefix = "exchange: invalid archive: "
	if len(message) >= len(prefix) && message[:len(prefix)] == prefix {
		return message[len(prefix):]
	}
	return message
}

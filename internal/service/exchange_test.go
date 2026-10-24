package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Trove/internal/blob"
	"Trove/internal/domain"
	"Trove/internal/store"
	"Trove/migrations"
)

// realService builds a service backed by a real SQLite database and blob store.
func realService(t *testing.T) *ItemService {
	t.Helper()
	dir := t.TempDir()

	st, err := store.Open(context.Background(), filepath.Join(dir, "repository.sqlite"))
	if err != nil {
		t.Fatalf("store.Open(): %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	migrator := &store.Migrator{DB: st.DB(), FS: migrations.FS, Dir: "."}
	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	blobs, err := blob.New(filepath.Join(dir, "files"))
	if err != nil {
		t.Fatalf("blob.New(): %v", err)
	}
	return NewItemService(st, blobs, time.Now)
}

func TestExportImport_RoundTrip_E10(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	source := realService(t)

	withFile, err := source.Create(ctx, ItemInput{
		Title:       "Has a file",
		Description: "summary",
		Note:        "# heading",
		Tags:        []string{"reading", "rust"},
		Favorite:    true,
		Links:       []LinkInput{{URL: "https://example.org/article", Label: "source"}},
		Files:       []FileInput{{FileName: "paper.pdf", MediaType: "application/pdf", Content: strings.NewReader("pdf bytes")}},
	})
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	trashed, err := source.Create(ctx, ItemInput{Title: "In the trash"})
	if err != nil {
		t.Fatalf("Create(trashed): %v", err)
	}
	if err := source.TrashItem(ctx, trashed.ID); err != nil {
		t.Fatalf("TrashItem(): %v", err)
	}

	archive := filepath.Join(t.TempDir(), "export.zip")
	f, err := os.Create(archive) // #nosec G304 -- test path
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if err := source.Export(ctx, f); err != nil {
		t.Fatalf("Export(): %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	target := realService(t)
	if err := target.Import(ctx, archive); err != nil {
		t.Fatalf("Import(): %v", err)
	}

	// The file attachment survives with its content and metadata.
	got, err := target.Get(ctx, withFile.ID)
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if got.Title != "Has a file" || !got.Favorite || got.Note != "# heading" {
		t.Errorf("item not preserved (E-8): %+v", got)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags not preserved (E-8): %v", got.Tags)
	}
	var fileID string
	for _, a := range got.Attachments {
		if a.Kind == domain.KindFile {
			fileID = a.ID
			if a.FileName != "paper.pdf" || a.MediaType != "application/pdf" {
				t.Errorf("file metadata not preserved: %+v", a)
			}
		}
	}
	content, err := target.Download(ctx, fileID)
	if err != nil {
		t.Fatalf("Download(): %v", err)
	}
	defer func() { _ = content.Content.Close() }()
	buf := make([]byte, 64)
	n, _ := content.Content.Read(buf)
	if string(buf[:n]) != "pdf bytes" {
		t.Errorf("file content not preserved (E-8): %q", buf[:n])
	}

	// The trashed item stays trashed.
	restored, err := target.Get(ctx, trashed.ID)
	if err != nil {
		t.Fatalf("Get(trashed): %v", err)
	}
	if restored.TrashedAt == nil {
		t.Errorf("trash state not preserved (E-8)")
	}

	// Search works on the imported data (S-6 on imported rows).
	result, err := target.List(ctx, domain.ListFilter{Search: "heading"})
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if result.Total != 1 {
		t.Errorf("imported item not searchable: total = %d", result.Total)
	}
}

func TestImport_RefusesNonEmptyRepository_E5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	source := realService(t)
	if _, err := source.Create(ctx, ItemInput{Title: "Existing"}); err != nil {
		t.Fatalf("Create(): %v", err)
	}
	archive := filepath.Join(t.TempDir(), "export.zip")
	f, err := os.Create(archive) // #nosec G304 -- test path
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if err := source.Export(ctx, f); err != nil {
		t.Fatalf("Export(): %v", err)
	}
	_ = f.Close()

	target := realService(t)
	if _, err := target.Create(ctx, ItemInput{Title: "Already here"}); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	err = target.Import(ctx, archive)
	if err == nil {
		t.Fatal("Import into a non-empty repository = nil, want an error (E-5)")
	}
	var validation *domain.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Import() error = %v, want *ValidationError", err)
	}
}

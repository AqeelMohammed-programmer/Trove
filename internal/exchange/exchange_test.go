package exchange

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Trove/internal/domain"
)

// --- fakes -----------------------------------------------------------------

type fakeSource struct {
	items   []domain.Item
	content map[string][]byte
}

func (f fakeSource) AllItems(context.Context) ([]domain.Item, error) { return f.items, nil }

func (f fakeSource) OpenContent(id string) (io.ReadCloser, error) {
	data, ok := f.content[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

type fakeTarget struct {
	items      []domain.Item
	content    map[string][]byte
	empty      bool
	failImport bool
}

func newFakeTarget() *fakeTarget {
	return &fakeTarget{content: map[string][]byte{}, empty: true}
}

func (f *fakeTarget) Empty(context.Context) (bool, error) { return f.empty, nil }

func (f *fakeTarget) SaveContent(id string, r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	f.content[id] = data
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (f *fakeTarget) DeleteContent(id string) error {
	delete(f.content, id)
	return nil
}

func (f *fakeTarget) ImportItems(_ context.Context, items []domain.Item) error {
	if f.failImport {
		return os.ErrInvalid
	}
	f.items = items
	return nil
}

func sampleExport() fakeSource {
	created := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 10, 2, 10, 15, 0, 0, time.UTC)
	content := []byte("file content")
	sum := sha256.Sum256(content)
	return fakeSource{
		items: []domain.Item{
			{
				ID: "item-1", Title: "Example", Description: "desc", Note: "# note",
				Favorite: true, CreatedAt: created, UpdatedAt: updated,
				Tags: []domain.Tag{{Name: "reading"}, {Name: "rust"}},
				Attachments: []domain.Attachment{
					{ID: "link-1", Kind: domain.KindLink, Label: "source", URL: "https://example.org/a", NormalizedURL: "https://example.org/a", CreatedAt: created, State: domain.StateActive},
					{ID: "file-1", Kind: domain.KindFile, FileName: "paper.pdf", SizeBytes: int64(len(content)), MediaType: "application/pdf", SHA256: hex.EncodeToString(sum[:]), CreatedAt: created, State: domain.StateTrashedIndividually},
				},
			},
			{
				ID: "item-2", Title: "Trashed", CreatedAt: created, UpdatedAt: created,
				TrashedAt: &created,
				Attachments: []domain.Attachment{
					{ID: "file-2", Kind: domain.KindFile, FileName: "b.bin", SizeBytes: 2, MediaType: "application/octet-stream", SHA256: hex.EncodeToString(sum[:]), CreatedAt: created, State: domain.StateTrashedWithItem},
				},
			},
		},
		content: map[string][]byte{"file-1": content, "file-2": content},
	}
}

func exportToFile(t *testing.T, src Source) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.zip")
	f, err := os.Create(path) // #nosec G304 -- test path
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	if err := Export(context.Background(), src, f, time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Export(): %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	return path
}

// --- tests -----------------------------------------------------------------

func TestExport_ContainsDataAndFiles_E2(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(): %v", err)
	}
	defer zr.Close() //nolint:errcheck // read-only

	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"data.json", "files/file-1", "files/file-2"} {
		if !names[want] {
			t.Errorf("archive missing %q (E-2): %v", want, names)
		}
	}
}

func TestExport_ExcludesSecrets_E3(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(): %v", err)
	}
	defer zr.Close() //nolint:errcheck // read-only

	for _, f := range zr.File {
		if f.Name != dataFileName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("Open(): %v", err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("ReadAll(): %v", err)
		}
		lower := strings.ToLower(string(data))
		for _, secret := range []string{"password", "argon2", "session", "token"} {
			if strings.Contains(lower, secret) {
				t.Errorf("archive metadata contains %q (E-3)", secret)
			}
		}
	}
}

func TestRoundTrip_E10(t *testing.T) {
	t.Parallel()

	src := sampleExport()
	path := exportToFile(t, src)

	target := newFakeTarget()
	if err := Import(context.Background(), path, target); err != nil {
		t.Fatalf("Import(): %v", err)
	}
	if len(target.items) != 2 {
		t.Fatalf("imported items = %d, want 2", len(target.items))
	}

	byID := map[string]domain.Item{}
	for _, item := range target.items {
		byID[item.ID] = item
	}
	got := byID["item-1"]
	if got.Title != "Example" || !got.Favorite || got.Note != "# note" {
		t.Errorf("item fields not preserved: %+v (E-8)", got)
	}
	if !got.CreatedAt.Equal(src.items[0].CreatedAt) || !got.UpdatedAt.Equal(src.items[0].UpdatedAt) {
		t.Error("timestamps not preserved (E-8)")
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v, want 2 (E-8)", got.Tags)
	}
	var link, file domain.Attachment
	for _, a := range got.Attachments {
		switch a.Kind {
		case domain.KindLink:
			link = a
		case domain.KindFile:
			file = a
		}
	}
	if link.URL != "https://example.org/a" || link.Label != "source" {
		t.Errorf("link not preserved: %+v", link)
	}
	if file.State != domain.StateTrashedIndividually || file.FileName != "paper.pdf" {
		t.Errorf("file attachment not preserved: %+v (E-8)", file)
	}
	if byID["item-2"].TrashedAt == nil {
		t.Error("trashed item lost its trash state (E-8)")
	}
	if !bytes.Equal(target.content["file-1"], src.content["file-1"]) {
		t.Error("file content not preserved (E-8)")
	}
}

func TestImport_RefusesNonEmpty_E5(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	target := newFakeTarget()
	target.empty = false

	if err := Import(context.Background(), path, target); !errors.Is(err, ErrNotEmpty) {
		t.Fatalf("Import(non-empty) = %v, want ErrNotEmpty (E-5)", err)
	}
}

func TestImport_RejectsBadChecksum_E6_E7(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	corrupted := rewriteArchive(t, path, func(data *Data, entries map[string][]byte) {
		// Change the declared digest so it no longer matches the content.
		data.Items[0].Attachments[1].SHA256 = ptr("0000000000000000000000000000000000000000000000000000000000000000")
	})

	target := newFakeTarget()
	err := Import(context.Background(), corrupted, target)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("Import(bad checksum) = %v, want a sha256 error (E-6)", err)
	}
	if len(target.items) != 0 || len(target.content) != 0 {
		t.Error("a failed import changed the repository (E-7)")
	}
}

func TestImport_RejectsMissingFile_E6(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	corrupted := rewriteArchive(t, path, func(_ *Data, entries map[string][]byte) {
		delete(entries, "files/file-1")
	})

	target := newFakeTarget()
	if err := Import(context.Background(), corrupted, target); err == nil {
		t.Fatal("Import(missing file) = nil, want an error (E-6)")
	}
	if len(target.items) != 0 {
		t.Error("a failed import changed the repository (E-7)")
	}
}

func TestImport_RejectsUnknownFormat_E6(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	corrupted := rewriteArchive(t, path, func(data *Data, _ map[string][]byte) {
		data.Format = "something-else"
	})

	target := newFakeTarget()
	if err := Import(context.Background(), corrupted, target); err == nil {
		t.Fatal("Import(unknown format) = nil, want an error (E-6)")
	}
}

func TestImport_RollsBackContentOnDBFailure_E7(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	target := newFakeTarget()
	target.failImport = true

	if err := Import(context.Background(), path, target); err == nil {
		t.Fatal("Import() = nil, want an error")
	}
	if len(target.content) != 0 {
		t.Errorf("content left behind after a failed import (E-7): %v", target.content)
	}
}

func TestImport_RejectsPathTraversal_E6(t *testing.T) {
	t.Parallel()

	path := exportToFile(t, sampleExport())
	corrupted := rewriteArchive(t, path, func(_ *Data, entries map[string][]byte) {
		entries["../evil"] = []byte("x")
	})

	target := newFakeTarget()
	if err := Import(context.Background(), corrupted, target); err == nil {
		t.Fatal("Import(path traversal entry) = nil, want an error")
	}
}

// rewriteArchive rebuilds an archive, letting the test mutate the metadata and
// the file entries.
func rewriteArchive(t *testing.T, path string, mutate func(*Data, map[string][]byte)) string {
	t.Helper()

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(): %v", err)
	}
	defer zr.Close() //nolint:errcheck // read-only

	entries := map[string][]byte{}
	var data Data
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("Open(): %v", err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("ReadAll(): %v", err)
		}
		if f.Name == dataFileName {
			if err := json.Unmarshal(content, &data); err != nil {
				t.Fatalf("Unmarshal(): %v", err)
			}
			continue
		}
		entries[f.Name] = content
	}
	mutate(&data, entries)

	out := filepath.Join(t.TempDir(), "modified.zip")
	file, err := os.Create(out) // #nosec G304 -- test path
	if err != nil {
		t.Fatalf("Create(): %v", err)
	}
	zw := zip.NewWriter(file)
	write := func(name string, content []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%s): %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("Write(%s): %v", name, err)
		}
	}
	for name, content := range entries {
		write(name, content)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal(): %v", err)
	}
	write(dataFileName, encoded)
	if err := zw.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	return out
}

func ptr(s string) *string { return &s }

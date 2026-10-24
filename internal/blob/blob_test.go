package blob

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testID = "0123456789abcdef0123456789abcdef"

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "files"))
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return s
}

func TestSave_ComputesSHA256AndSize_A4(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	content := []byte("the exact bytes of the file")

	digest, size, err := s.Save(testID, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save(): %v", err)
	}
	want := sha256.Sum256(content)
	if digest != hex.EncodeToString(want[:]) {
		t.Fatalf("digest = %q, want %q", digest, hex.EncodeToString(want[:]))
	}
	if size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
}

func TestSave_StoresByIDNotName_A3(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	if _, _, err := s.Save(testID, strings.NewReader("payload")); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), testID)); err != nil {
		t.Fatalf("content not stored under its id: %v", err)
	}
	// The stored file must be private.
	info, err := os.Stat(filepath.Join(s.Dir(), testID))
	if err != nil {
		t.Fatalf("Stat(): %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("content permissions = %04o, want 0600", perm)
	}
}

func TestOpen_RoundTrips(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	content := []byte("round trip")
	if _, _, err := s.Save(testID, bytes.NewReader(content)); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	rc, err := s.Open(testID)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll(): %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestOpen_MissingIsNotFound(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	if _, err := s.Open(testID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open(missing) error = %v, want ErrNotFound", err)
	}
}

func TestDelete_IsIdempotent_X9(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	if _, _, err := s.Save(testID, strings.NewReader("x")); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	if err := s.Delete(testID); err != nil {
		t.Fatalf("Delete(): %v", err)
	}
	if err := s.Delete(testID); err != nil {
		t.Fatalf("second Delete() = %v, want nil (X-9 retry safety)", err)
	}
	if exists, err := s.Exists(testID); err != nil || exists {
		t.Fatalf("Exists() = %v, %v, want false, nil", exists, err)
	}
}

func TestSave_RejectsPathTraversalName(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	for _, id := range []string{
		"../../etc/passwd",
		"..",
		".",
		"abc/def",
		"0123456789abcdef0123456789abcdeZ", // non-hex
		"short",
	} {
		if _, _, err := s.Save(id, strings.NewReader("x")); err == nil {
			t.Errorf("Save(%q) error = nil, want rejection", id)
		}
	}
}

func TestList_IgnoresTemporaryUploads(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	if _, _, err := s.Save(testID, strings.NewReader("x")); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	// Simulate an interrupted upload.
	if err := os.WriteFile(filepath.Join(s.Dir(), ".upload-123"), []byte("partial"), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	ids, err := s.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(ids) != 1 || ids[0] != testID {
		t.Fatalf("List() = %v, want just %q", ids, testID)
	}
}

func TestSave_LeavesNoTemporaryFileOnSuccess(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	if _, _, err := s.Save(testID, strings.NewReader("x")); err != nil {
		t.Fatalf("Save(): %v", err)
	}
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatalf("ReadDir(): %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

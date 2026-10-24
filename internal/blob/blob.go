// Package blob stores copies of user-provided files on disk. Content is
// addressed by attachment id, never by a user-supplied file name, and is
// written through a temporary file so a crash never leaves a half-written
// attachment (AGENTS.md section 6).
package blob

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrNotFound means no content is stored for an id.
var ErrNotFound = errors.New("blob: content not found")

// Store keeps file content under a single directory. It is safe for concurrent
// use: every operation uses its own temporary file and id-named destination.
type Store struct {
	// dir is the absolute directory holding the content files.
	dir string
}

// New creates the content directory (mode 0700) and returns a store rooted at
// it.
func New(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("blob: empty content directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving content directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("creating content directory %q: %w", abs, err)
	}
	return &Store{dir: abs}, nil
}

// Dir returns the absolute content directory.
func (s *Store) Dir() string { return s.dir }

// Save streams r to disk under the given attachment id, computing the SHA-256
// and byte count as it goes. It writes to a temporary file in the same
// directory and renames it into place, so the destination is either absent or
// complete. No more than a small buffer is held in memory.
func (s *Store) Save(id string, r io.Reader) (digest string, size int64, err error) {
	if !validID(id) {
		return "", 0, errors.New("blob: invalid attachment id")
	}

	tmp, err := os.CreateTemp(s.dir, ".upload-*")
	if err != nil {
		return "", 0, fmt.Errorf("creating temporary file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup; a successful rename removes the temp path.
	defer func() {
		if err != nil {
			if rmErr := os.Remove(tmpName); rmErr != nil && !os.IsNotExist(rmErr) {
				err = fmt.Errorf("%w (and removing temporary file: %w)", err, rmErr)
			}
		}
	}()

	hash := sha256.New()
	size, err = io.Copy(io.MultiWriter(tmp, hash), r)
	if err != nil {
		_ = tmp.Close()
		return "", 0, fmt.Errorf("writing content: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", 0, fmt.Errorf("setting content permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", 0, fmt.Errorf("closing content file: %w", err)
	}
	if err := os.Rename(tmpName, s.path(id)); err != nil {
		return "", 0, fmt.Errorf("moving content into place: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// Open returns the stored content for reading. The caller closes it.
func (s *Store) Open(id string) (io.ReadCloser, error) {
	if !validID(id) {
		return nil, errors.New("blob: invalid attachment id")
	}
	f, err := os.Open(s.path(id)) // #nosec G304 -- path is id-validated and never user-supplied
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("opening content: %w", err)
	}
	return f, nil
}

// Delete removes stored content. A missing file is not an error, because
// deletion must be safe to retry (X-9).
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return errors.New("blob: invalid attachment id")
	}
	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting content: %w", err)
	}
	return nil
}

// Exists reports whether content is present for an id.
func (s *Store) Exists(id string) (bool, error) {
	if !validID(id) {
		return false, errors.New("blob: invalid attachment id")
	}
	if _, err := os.Stat(s.path(id)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking content: %w", err)
	}
	return true, nil
}

// List returns the ids of every stored content file. It ignores temporary
// upload files so an interrupted save is not mistaken for real content.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("listing content directory: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !validID(name) {
			continue
		}
		ids = append(ids, name)
	}
	return ids, nil
}

func (s *Store) path(id string) string {
	// filepath.Join cleans the path; validID guarantees no separators, so the
	// result always stays inside the content directory.
	return filepath.Join(s.dir, id)
}

// validID accepts only lowercase hex ids of the length the application
// generates (128 bits, hex encoded). This is the path-traversal guard: a
// user-supplied file name is never part of a path.
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

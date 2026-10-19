// Package memorystorage is an in-memory FileStorage adapter for tests. It
// records stored and removed names so tests can assert byte-side effects
// without touching the filesystem.
package memorystorage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"sync"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// Storage keeps upload bytes in memory.
type Storage struct {
	mu      sync.Mutex
	files   map[string][]byte
	stored  []string
	removed []string
	failOn  map[string]error
}

// New creates an empty in-memory storage.
func New() *Storage {
	return &Storage{files: map[string][]byte{}, failOn: map[string]error{}}
}

// FailOnStore makes Store fail for names containing match.
func (s *Storage) FailOnStore(match string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failOn[match] = err
}

// Store buffers src in memory under a deterministic stored name, honoring
// the maxBytes ceiling the same way the local adapter does.
func (s *Storage) Store(_ context.Context, name, _ string, src io.Reader, maxBytes int64) (string, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for match, err := range s.failOn {
		if strings.Contains(name, match) {
			return "", 0, err
		}
	}
	data, err := io.ReadAll(io.LimitReader(src, maxBytes+1))
	if err != nil {
		return "", 0, err
	}
	if int64(len(data)) > maxBytes {
		return "", 0, apperr.NewBadRequest("invalid file size")
	}
	stored := "stored-" + name
	if _, exists := s.files[stored]; exists {
		return "", 0, errors.New("duplicate stored name")
	}
	s.files[stored] = data
	s.stored = append(s.stored, stored)
	return stored, int64(len(data)), nil
}

// Open is not implemented: read-path tests use the local adapter with a temp
// directory instead.
func (s *Storage) Open(context.Context, string) (fs.File, error) {
	return nil, errors.New("memorystorage: Open is not supported")
}

// Remove forgets one stored upload and records the removal.
func (s *Storage) Remove(_ context.Context, storedName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if storedName != "" {
		s.removed = append(s.removed, storedName)
	}
	delete(s.files, storedName)
	return nil
}

// UploadURL returns a pseudo upload URL for one stored name.
func (s *Storage) UploadURL(storedName string) string {
	return "/api/uploads/" + storedName
}

// IsUploadURL reports whether url carries this adapter's prefix.
func (s *Storage) IsUploadURL(url string) bool {
	return strings.HasPrefix(url, "/api/uploads/")
}

// StoredName extracts the stored name from an upload URL.
func (s *Storage) StoredName(url string) string {
	if !s.IsUploadURL(url) {
		return ""
	}
	return strings.TrimPrefix(url, "/api/uploads/")
}

// Removed returns the names passed to Remove.
func (s *Storage) Removed() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.removed...)
}

// Stored returns the names successfully stored.
func (s *Storage) Stored() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.stored...)
}

// Package localstorage stores uploaded file bytes on the local filesystem
// under one directory. Stored names are UUID-prefixed sanitized names, files
// are created exclusively to avoid overwrites, and uploads are served from
// the /api/uploads/ URL prefix owned by this adapter.
package localstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// uploadURLPrefix is the public URL prefix this adapter mints for stored files.
const uploadURLPrefix = "/api/uploads/"

// Storage stores upload bytes in one local directory.
type Storage struct {
	dir string
}

// New creates a local upload storage rooted at dir.
func New(dir string) *Storage {
	return &Storage{dir: dir}
}

// Store writes src under a fresh stored name and reports that name, the byte
// count, and the public upload URL. It reads at most maxBytes; larger inputs
// are rejected and their partial target removed so no oversized file remains.
func (s *Storage) Store(ctx context.Context, name, contentType string, src io.Reader, maxBytes int64) (storedName string, size int64, err error) {
	if ctx.Err() != nil {
		return "", 0, ctx.Err()
	}
	clean, nameErr := CleanUploadName(name)
	if nameErr != nil {
		return "", 0, nameErr
	}
	if mkdirErr := os.MkdirAll(s.dir, 0o755); mkdirErr != nil {
		return "", 0, fmt.Errorf("create upload dir: %w", mkdirErr)
	}
	stored := uuid.NewString() + "-" + clean
	target := filepath.Join(s.dir, stored)
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", 0, fmt.Errorf("create upload file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(src, maxBytes+1))
	if copyErr != nil {
		return "", 0, errors.Join(fmt.Errorf("write upload: %w", copyErr), file.Close(), os.Remove(target))
	}
	if written > maxBytes {
		if err := errors.Join(file.Close(), os.Remove(target)); err != nil {
			return "", 0, fmt.Errorf("cleanup oversized upload: %w", err)
		}
		return "", 0, apperr.NewBadRequest("invalid file size")
	}
	if err := file.Close(); err != nil {
		return "", 0, errors.Join(fmt.Errorf("close upload file: %w", err), os.Remove(target))
	}
	if ctx.Err() != nil {
		return "", 0, errors.Join(ctx.Err(), os.Remove(target))
	}
	return stored, written, nil
}

// Open returns one stored upload as an fs.File.
func (s *Storage) Open(ctx context.Context, storedName string) (fs.File, error) {
	if err := validateStoredName(storedName); err != nil {
		return nil, err
	}
	return os.DirFS(s.dir).Open(storedName)
}

// Remove deletes one stored upload; a missing file is not an error.
func (s *Storage) Remove(ctx context.Context, storedName string) error {
	if storedName == "" {
		return nil
	}
	if err := validateStoredName(storedName); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.dir, storedName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove upload file: %w", err)
	}
	return nil
}

// UploadURL returns the public URL for one stored upload.
func (s *Storage) UploadURL(storedName string) string {
	return uploadURLPrefix + storedName
}

// IsUploadURL reports whether url was minted by this adapter.
func (s *Storage) IsUploadURL(url string) bool {
	return strings.HasPrefix(url, uploadURLPrefix)
}

// StoredName extracts the stored name from an upload URL minted by this
// adapter. An empty name means the URL is not an upload URL.
func (s *Storage) StoredName(url string) string {
	if !s.IsUploadURL(url) {
		return ""
	}
	return strings.TrimPrefix(url, uploadURLPrefix)
}

// CleanUploadName reduces a client-supplied filename to a safe base name.
func CleanUploadName(name string) (string, error) {
	cleaned := filepath.Base(strings.TrimSpace(name))
	if cleaned == "." || cleaned == string(filepath.Separator) || cleaned == "" {
		return "", apperr.NewBadRequest("invalid file name")
	}
	if strings.Contains(cleaned, "/") || strings.Contains(cleaned, "\\") {
		return "", apperr.NewBadRequest("invalid file name")
	}
	return cleaned, nil
}

func validateStoredName(name string) error {
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return apperr.NewBadRequest("invalid upload file")
	}
	return nil
}

// Package domain contains uploaded file metadata rules.
package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// File validation errors.
var (
	ErrInvalidFileID       = errors.New("invalid file id")
	ErrInvalidFileName     = errors.New("invalid file name")
	ErrInvalidFileURL      = errors.New("invalid file url")
	ErrInvalidFileSize     = errors.New("invalid file size")
	ErrInvalidContentType  = errors.New("invalid content type")
	ErrInvalidCategoryID   = errors.New("invalid category id")
	ErrInvalidCategoryName = errors.New("invalid category name")
)

// FileObject is validated metadata for a file uploaded through the back office
// or registered from an external HTTP(S) URL.
type FileObject struct {
	ID          int64
	Name        string
	URL         string
	Size        int64
	ContentType string
	CategoryID  int64
	CreatedAt   time.Time
}

// Rename changes the display name while preserving the stored location.
func (f FileObject) Rename(name string) (FileObject, error) {
	return RestoreFileObject(f.ID, name, f.URL, f.Size, f.ContentType, f.CategoryID, f.CreatedAt)
}

// RestoreFileObject rebuilds an uploaded file record from a trusted store representation.
func RestoreFileObject(id int64, name, fileURL string, size int64, contentType string, categoryID int64, createdAt time.Time) (FileObject, error) {
	name = strings.TrimSpace(name)
	fileURL = strings.TrimSpace(fileURL)
	contentType = strings.TrimSpace(contentType)
	if id < 0 {
		return FileObject{}, ErrInvalidFileID
	}
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || len(name) > 180 {
		return FileObject{}, ErrInvalidFileName
	}
	if !validFileURL(fileURL) {
		return FileObject{}, ErrInvalidFileURL
	}
	if size < 0 {
		return FileObject{}, ErrInvalidFileSize
	}
	if contentType == "" || len(contentType) > 120 {
		return FileObject{}, ErrInvalidContentType
	}
	if categoryID < 0 {
		return FileObject{}, ErrInvalidCategoryID
	}
	return FileObject{ID: id, Name: name, URL: fileURL, Size: size, ContentType: contentType, CategoryID: categoryID, CreatedAt: createdAt}, nil
}

// FileCategory groups uploaded files into an operator-managed tree.
type FileCategory struct {
	ID        int64
	ParentID  int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RestoreFileCategory rebuilds a file category from a trusted store representation.
func RestoreFileCategory(id, parentID int64, name string, createdAt, updatedAt time.Time) (FileCategory, error) {
	name = strings.TrimSpace(name)
	if id < 0 || parentID < 0 || (id > 0 && id == parentID) {
		return FileCategory{}, ErrInvalidCategoryID
	}
	if name == "" || len(name) > 80 {
		return FileCategory{}, ErrInvalidCategoryName
	}
	return FileCategory{ID: id, ParentID: parentID, Name: name, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func validFileURL(value string) bool {
	if value == "" || len(value) > 2048 {
		return false
	}
	if strings.HasPrefix(value, "/") {
		return !strings.HasPrefix(value, "//")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

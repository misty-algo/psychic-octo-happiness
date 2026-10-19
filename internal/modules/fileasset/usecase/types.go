// Package usecase coordinates uploaded file metadata workflows.
package usecase

import (
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

const (

	// defaultMaxUploadBytes bounds one uploaded file's byte size.
	defaultMaxUploadBytes = 10 << 20
)

// Store persists uploaded file metadata and its operator-managed categories.
type Store interface {
	CreateFile(context.Context, domain.FileObject) (domain.FileObject, error)
	FindFileByID(context.Context, int64) (domain.FileObject, error)
	ListFiles(context.Context, ListFilter) ([]domain.FileObject, int, error)
	UpdateFile(context.Context, domain.FileObject) (domain.FileObject, error)
	DeleteFile(context.Context, int64) error
	CreateCategory(context.Context, domain.FileCategory) (domain.FileCategory, error)
	UpdateCategory(context.Context, domain.FileCategory) (domain.FileCategory, error)
	FindCategoryByID(context.Context, int64) (domain.FileCategory, error)
	ListCategories(context.Context) ([]domain.FileCategory, error)
	DeleteCategory(context.Context, int64) error
	CategoryNameExists(context.Context, string, int64, int64) (bool, error)
}

// FileStorage stores and retrieves uploaded file bytes. Stored names are
// opaque to callers; UploadURL/StoredName translate between stored names and
// the public URL space the adapter owns.
type FileStorage interface {
	Store(ctx context.Context, name, contentType string, src io.Reader, maxBytes int64) (storedName string, size int64, err error)
	Open(ctx context.Context, storedName string) (fs.File, error)
	Remove(ctx context.Context, storedName string) error
	UploadURL(storedName string) string
	StoredName(url string) string
}

// FileSource is one uploaded file's bytes with delivery metadata.
type FileSource struct {
	Name        string
	ContentType string
	Reader      io.Reader
}

// Usecase coordinates uploaded file metadata and byte-storage rules.
type Usecase struct {
	store         Store
	storage       FileStorage
	maxUploadSize int64
}

// Option customizes the file asset usecase.
type Option func(*Usecase)

// WithMaxUploadBytes replaces the default upload size ceiling; tests use it
// to exercise the limit without allocating default-sized inputs.
func WithMaxUploadBytes(max int64) Option {
	return func(u *Usecase) {
		if max > 0 {
			u.maxUploadSize = max
		}
	}
}

// New creates a file asset usecase over one metadata store and one byte
// storage adapter.
func New(store Store, storage FileStorage, opts ...Option) *Usecase {
	u := &Usecase{store: store, storage: storage, maxUploadSize: defaultMaxUploadBytes}
	for _, opt := range opts {
		if opt != nil {
			opt(u)
		}
	}
	return u
}

// FileInput carries an uploaded file record after the HTTP adapter stores bytes.
type FileInput struct {
	Name        string
	URL         string
	Size        int64
	ContentType string
	CategoryID  int64
}

// URLImportInput carries an external HTTP(S) URL to register as a file asset.
type URLImportInput struct {
	Name       string
	URL        string
	CategoryID int64
}

// RenameInput carries a display-name update for one file.
type RenameInput struct {
	ID   int64
	Name string
}

// ListInput carries pagination for file lists.
type ListInput struct {
	Page       int
	PageSize   int
	CategoryID int64
}

// ListFilter is the validated store-facing pagination window.
type ListFilter struct {
	pagination.Window
	CategoryID int64
}

// ListOutput is a paginated uploaded file result.
type ListOutput struct {
	Items    []FileObject
	Page     int
	PageSize int
	Total    int
}

// FileObject is the adapter-facing uploaded file DTO.
type FileObject struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CategoryID  int64     `json:"category_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// CategoryInput carries category fields for creation.
type CategoryInput struct {
	Name     string
	ParentID int64
}

// UpdateCategoryInput carries category fields for updates.
type UpdateCategoryInput struct {
	ID       int64
	Name     string
	ParentID int64
}

// Category is a file category tree node returned to HTTP adapters.
type Category struct {
	ID        int64      `json:"id"`
	ParentID  int64      `json:"parent_id"`
	Name      string     `json:"name"`
	Children  []Category `json:"children"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

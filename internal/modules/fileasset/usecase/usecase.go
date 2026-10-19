package usecase

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

// CreateFile stores uploaded bytes and their metadata as one unit: when
// metadata persistence fails after bytes were stored, the stored bytes are
// removed so no orphaned upload survives.
func (u *Usecase) CreateFile(ctx context.Context, input FileInput, source FileSource) (FileObject, error) {
	if err := u.ready(); err != nil {
		return FileObject{}, err
	}
	if source.Name == "" {
		source.Name = input.Name
	}
	storedName, size, err := u.storage.Store(ctx, source.Name, source.ContentType, source.Reader, u.maxUploadSize)
	if err != nil {
		return FileObject{}, err
	}
	file, buildErr := domain.RestoreFileObject(0, source.Name, u.storage.UploadURL(storedName), size, source.ContentType, input.CategoryID, time.Time{})
	if buildErr == nil {
		created, storeErr := u.store.CreateFile(ctx, file)
		if storeErr == nil {
			return fromFile(created), nil
		}
		buildErr = storeErr
	}
	if removeErr := u.storage.Remove(ctx, storedName); removeErr != nil {
		return FileObject{}, errors.Join(buildErr, removeErr)
	}
	return FileObject{}, buildErr
}

// ImportURL registers an external HTTP(S) URL as a file asset without fetching
// the remote body. The backend stores metadata only, so this path cannot become
// a server-side request forgery primitive.
func (u *Usecase) ImportURL(ctx context.Context, input URLImportInput) (FileObject, error) {
	if err := u.ready(); err != nil {
		return FileObject{}, err
	}
	fileURL, err := normalizeExternalURL(input.URL)
	if err != nil {
		return FileObject{}, err
	}
	if categoryErr := u.ensureCategory(ctx, input.CategoryID); categoryErr != nil {
		return FileObject{}, categoryErr
	}
	name := importedFileName(input.Name, fileURL)
	file, err := domain.RestoreFileObject(0, name, fileURL, 0, "external/url", input.CategoryID, time.Time{})
	if err != nil {
		return FileObject{}, mapDomainError(err)
	}
	created, err := u.store.CreateFile(ctx, file)
	if err != nil {
		return FileObject{}, err
	}
	return fromFile(created), nil
}

// ListFiles returns paginated uploaded file records.
func (u *Usecase) ListFiles(ctx context.Context, input ListInput) (ListOutput, error) {
	if err := u.ready(); err != nil {
		return ListOutput{}, err
	}
	filter, err := normalizeListInput(input)
	if err != nil {
		return ListOutput{}, err
	}
	files, total, err := u.store.ListFiles(ctx, filter)
	if err != nil {
		return ListOutput{}, err
	}
	return ListOutput{
		Items:    mapFiles(files),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    total,
	}, nil
}

// RenameFile updates the display name for one file metadata record.
func (u *Usecase) RenameFile(ctx context.Context, input RenameInput) (FileObject, error) {
	if err := u.ready(); err != nil {
		return FileObject{}, err
	}
	if input.ID <= 0 {
		return FileObject{}, apperr.NewBadRequest("invalid file id")
	}
	file, err := u.store.FindFileByID(ctx, input.ID)
	if err != nil {
		return FileObject{}, err
	}
	renamed, err := file.Rename(input.Name)
	if err != nil {
		return FileObject{}, mapDomainError(err)
	}
	saved, err := u.store.UpdateFile(ctx, renamed)
	if err != nil {
		return FileObject{}, err
	}
	return fromFile(saved), nil
}

// DeleteFile removes one file metadata record and returns the deleted snapshot.
func (u *Usecase) DeleteFile(ctx context.Context, id int64) (FileObject, error) {
	if err := u.ready(); err != nil {
		return FileObject{}, err
	}
	if id <= 0 {
		return FileObject{}, apperr.NewBadRequest("invalid file id")
	}
	file, err := u.store.FindFileByID(ctx, id)
	if err != nil {
		return FileObject{}, err
	}
	if err := u.store.DeleteFile(ctx, id); err != nil {
		return FileObject{}, err
	}
	if stored := u.storage.StoredName(file.URL); stored != "" {
		if err := u.storage.Remove(ctx, stored); err != nil {
			return FileObject{}, err
		}
	}
	return fromFile(file), nil
}

// OpenUpload returns one stored upload for delivery after its stored name
// has been validated.
func (u *Usecase) OpenUpload(ctx context.Context, storedName string) (fs.File, error) {
	if err := u.ready(); err != nil {
		return nil, err
	}
	return u.storage.Open(ctx, storedName)
}

func (u *Usecase) ready() error {
	if u == nil || u.store == nil {
		return apperr.New(apperr.ErrInternalServer, "file store is not configured")
	}
	return nil
}

func normalizeListInput(input ListInput) (ListFilter, error) {
	if input.CategoryID < 0 {
		return ListFilter{}, apperr.NewBadRequest("invalid category id")
	}
	window, err := pagination.Normalize(input.Page, input.PageSize, pagination.DefaultOptions)
	if err != nil {
		return ListFilter{}, apperr.NewBadRequest("invalid pagination")
	}
	return ListFilter{Window: window, CategoryID: input.CategoryID}, nil
}

func mapFiles(files []domain.FileObject) []FileObject {
	out := make([]FileObject, 0, len(files))
	for _, file := range files {
		out = append(out, fromFile(file))
	}
	return out
}

func fromFile(file domain.FileObject) FileObject {
	return FileObject{
		ID:          file.ID,
		Name:        file.Name,
		URL:         file.URL,
		Size:        file.Size,
		ContentType: file.ContentType,
		CategoryID:  file.CategoryID,
		CreatedAt:   file.CreatedAt,
	}
}

func (u *Usecase) ensureCategory(ctx context.Context, id int64) error {
	if id < 0 {
		return apperr.NewBadRequest("invalid category id")
	}
	if id == 0 {
		return nil
	}
	_, err := u.store.FindCategoryByID(ctx, id)
	return err
}

func normalizeExternalURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return "", apperr.NewBadRequest("invalid file url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", apperr.NewBadRequest("invalid file url")
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func importedFileName(name, fileURL string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return "external-file"
	}
	base := strings.TrimSpace(path.Base(parsed.Path))
	if base == "." || base == "/" || base == "" {
		return parsed.Host
	}
	return base
}

func mapDomainError(err error) error {
	for _, entry := range domainErrorMessages {
		if errors.Is(err, entry.err) {
			return apperr.NewBadRequest(entry.message)
		}
	}
	return err
}

var domainErrorMessages = []struct {
	err     error
	message string
}{
	{domain.ErrInvalidFileID, "invalid file id"},
	{domain.ErrInvalidFileName, "invalid file name"},
	{domain.ErrInvalidFileURL, "invalid file url"},
	{domain.ErrInvalidFileSize, "invalid file size"},
	{domain.ErrInvalidContentType, "invalid content type"},
	{domain.ErrInvalidCategoryID, "invalid category id"},
	{domain.ErrInvalidCategoryName, "invalid category name"},
}

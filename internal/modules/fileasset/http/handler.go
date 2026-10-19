// Package fileassethttp adapts file metadata HTTP requests to the file usecase.
package fileassethttp

import (
	"errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	"github.com/NSObjects/echo-admin/internal/modules/fileasset/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/logging"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

const (
	maxUploadBytes = 10 << 20
)

// Handler adapts file HTTP requests to the file usecase.
type Handler struct {
	usecase *usecase.Usecase
	audit   *oprec.Recorder
}

// New creates a file HTTP handler.
func New(uc *usecase.Usecase, audit *oprec.Recorder) *Handler {
	return &Handler{usecase: uc, audit: audit}
}

// Register mounts file routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.GET("/file-categories", handler.ListCategories)
	group.POST("/file-categories", handler.CreateCategory)
	group.PATCH("/file-categories/:id", handler.UpdateCategory)
	group.DELETE("/file-categories/:id", handler.DeleteCategory)
	group.GET("/files", handler.ListFiles)
	group.POST("/files", handler.UploadFile)
	group.POST("/files/import-url", handler.ImportURL)
	group.PATCH("/files/:id/name", handler.RenameFile)
	group.DELETE("/files/:id", handler.DeleteFile)
	group.GET("/uploads/*", handler.ServeUpload)
}

// ListCategories returns the category tree used by file management.
func (h *Handler) ListCategories(c *echo.Context) error {
	categories, err := h.usecase.ListCategories(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, categories)
}

// CreateCategory adds one file category.
func (h *Handler) CreateCategory(c *echo.Context) error {
	var req categoryRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	category, err := h.usecase.CreateCategory(c.Request().Context(), usecase.CategoryInput{
		Name:     req.Name,
		ParentID: req.ParentID,
	})
	if err != nil {
		return err
	}
	err = h.audit.Record(c, "create", "file_category", strconv.FormatInt(category.ID, 10), "created file category", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, category)
}

// UpdateCategory changes one file category.
func (h *Handler) UpdateCategory(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "file category")
	if err != nil {
		return err
	}
	var req categoryRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return bindErr
	}
	category, err := h.usecase.UpdateCategory(c.Request().Context(), usecase.UpdateCategoryInput{
		ID:       id,
		Name:     req.Name,
		ParentID: req.ParentID,
	})
	if err != nil {
		return err
	}
	err = h.audit.Record(c, "update", "file_category", strconv.FormatInt(category.ID, 10), "updated file category", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, category)
}

// DeleteCategory removes one file category without deleting files.
func (h *Handler) DeleteCategory(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "file category")
	if err != nil {
		return err
	}
	err = h.usecase.DeleteCategory(c.Request().Context(), id)
	err = h.audit.Record(c, "delete", "file_category", strconv.FormatInt(id, 10), "deleted file category", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, deletedResponse{ID: id})
}

// ListFiles returns uploaded file records.
func (h *Handler) ListFiles(c *echo.Context) error {
	input, err := listInput(c)
	if err != nil {
		return err
	}
	output, err := h.usecase.ListFiles(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, output.Page, output.PageSize, output.Total)
}

// UploadFile stores one uploaded file and records its metadata.
func (h *Handler) UploadFile(c *echo.Context) error {
	header, err := c.FormFile("file")
	if err != nil {
		return apperr.WrapBadRequest(err, "file is required")
	}
	categoryID, err := formInt64(c, "category_id")
	if err != nil {
		return err
	}
	source, closeUpload, sourceErr := multipartSource(header)
	if sourceErr != nil {
		return sourceErr
	}
	file, opErr := h.usecase.CreateFile(c.Request().Context(), usecase.FileInput{CategoryID: categoryID}, source)
	opErr = errors.Join(opErr, closeUpload())
	if err := h.audit.Record(c, "upload", "file", file.URL, "uploaded file", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.Created(c, file)
}

// ImportURL registers an external HTTP(S) URL as a file asset.
func (h *Handler) ImportURL(c *echo.Context) error {
	var req importURLRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	file, err := h.usecase.ImportURL(c.Request().Context(), usecase.URLImportInput{
		Name:       req.Name,
		URL:        req.URL,
		CategoryID: req.CategoryID,
	})
	err = h.audit.Record(c, "import_url", "file", file.URL, "imported file url", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, file)
}

// RenameFile updates one file display name.
func (h *Handler) RenameFile(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "file")
	if err != nil {
		return err
	}
	var req renameFileRequest
	err = httpreq.BindAndValidate(c, &req)
	if err != nil {
		return err
	}
	file, err := h.usecase.RenameFile(c.Request().Context(), usecase.RenameInput{
		ID:   id,
		Name: req.Name,
	})
	err = h.audit.Record(c, "rename", "file", strconv.FormatInt(file.ID, 10), "renamed file", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, file)
}

// DeleteFile removes one file metadata record and its stored bytes when present.
func (h *Handler) DeleteFile(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "file")
	if err != nil {
		return err
	}
	file, opErr := h.usecase.DeleteFile(c.Request().Context(), id)
	if err := h.audit.Record(c, "delete", "file", strconv.FormatInt(file.ID, 10), "deleted file", opErr); err != nil {
		return err
	}
	if opErr != nil {
		return opErr
	}
	return httpresp.OK(c, deletedResponse{ID: file.ID})
}

// ServeUpload returns one stored uploaded file after boot-level route
// authorization has accepted the request.
func (h *Handler) ServeUpload(c *echo.Context) error {
	storedName, err := cleanStoredUploadName(c.Param("*"))
	if err != nil {
		return err
	}
	file, err := h.usecase.OpenUpload(c.Request().Context(), storedName)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Read-path close failures only waste a descriptor; the response
			// is already committed, so log-and-continue is the right ceiling.
			logging.FromContext(c.Request().Context()).Warn().Err(closeErr).Msg("close upload file failed")
		}
	}()
	return c.FileFS(storedName, singleFileFS{file})
}

// singleFileFS adapts one opened fs.File to echo's FileFS, which needs an
// fs.FS rooted at the stored name.
type singleFileFS struct{ file fs.File }

func (f singleFileFS) Open(name string) (fs.File, error) { return f.file, nil }

func listInput(c *echo.Context) (usecase.ListInput, error) {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return usecase.ListInput{}, err
	}
	categoryID, err := httpreq.QueryInt64(c, "category_id", 0)
	if err != nil {
		return usecase.ListInput{}, err
	}
	return usecase.ListInput{Page: page, PageSize: pageSize, CategoryID: categoryID}, nil
}

func formInt64(c *echo.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.FormValue(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, apperr.NewBadRequest("invalid " + name)
	}
	return value, nil
}

func validateUploadHeader(header *multipart.FileHeader) error {
	if header == nil {
		return apperr.NewBadRequest("file is required")
	}
	if header.Size <= 0 || header.Size > maxUploadBytes {
		return apperr.NewBadRequest("invalid file size")
	}
	return nil
}

func cleanStoredUploadName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", apperr.NewBadRequest("invalid upload file")
	}
	return name, nil
}

func multipartSource(header *multipart.FileHeader) (usecase.FileSource, func() error, error) {
	if err := validateUploadHeader(header); err != nil {
		return usecase.FileSource{}, nil, err
	}
	opened, err := header.Open()
	if err != nil {
		return usecase.FileSource{}, nil, fmt.Errorf("open upload: %w", err)
	}
	return usecase.FileSource{
		Name:        header.Filename,
		ContentType: contentType(header),
		Reader:      opened,
	}, opened.Close, nil
}

type deletedResponse struct {
	ID int64 `json:"id"`
}

func contentType(header *multipart.FileHeader) string {
	if value := header.Header.Get(echo.HeaderContentType); value != "" {
		return value
	}
	return http.DetectContentType(nil)
}

package fileassethttp_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/fileasset/adapters/localstorage"
	filedomain "github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	filehttp "github.com/NSObjects/echo-admin/internal/modules/fileasset/http"
	fileusecase "github.com/NSObjects/echo-admin/internal/modules/fileasset/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

func TestUploadFileStoresBytesMetadataAndOperation(t *testing.T) {
	e, store, recorder, uploadDir := newFileEcho(t)

	rec := doMultipart(t, e, "/api/files", "hello.txt", "hello", 7)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", store.createCalls)
	}
	if store.created.Name != "hello.txt" {
		t.Fatalf("created name = %q, want hello.txt", store.created.Name)
	}
	storedName := strings.TrimPrefix(store.created.URL, "/api/uploads/")
	if storedName == store.created.URL || storedName == "" {
		t.Fatalf("created URL = %q, want /api/uploads/<file>", store.created.URL)
	}
	if _, err := os.Stat(filepath.Join(uploadDir, storedName)); err != nil {
		t.Fatalf("stat uploaded file: %v", err)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].ActorID; got != 7 {
		t.Fatalf("operation actor id = %d, want 7", got)
	}
}

func TestUploadFileAssignsCategory(t *testing.T) {
	e, store, _, _ := newFileEcho(t)
	store.categories = []filedomain.FileCategory{mustCategory(t, 5, 0, "合同")}

	rec := doMultipartWithFields(t, e, "/api/files", "hello.txt", "hello", 7, map[string]string{
		"category_id": "5",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.created.CategoryID != 5 {
		t.Fatalf("created category id = %d, want 5", store.created.CategoryID)
	}
}

func TestImportURLStoresMetadataAndOperation(t *testing.T) {
	e, store, recorder, _ := newFileEcho(t)

	rec := doJSON(t, e, http.MethodPost, "/api/files/import-url", `{"url":"https://cdn.example.com/manual.pdf?version=1"}`, 7)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import url status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", store.createCalls)
	}
	if store.created.Name != "manual.pdf" {
		t.Fatalf("created name = %q, want manual.pdf", store.created.Name)
	}
	if store.created.URL != "https://cdn.example.com/manual.pdf?version=1" {
		t.Fatalf("created URL = %q, want imported url", store.created.URL)
	}
	if store.created.Size != 0 {
		t.Fatalf("created size = %d, want 0 for metadata-only URL import", store.created.Size)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if recorder.records[0].Action != "import_url" {
		t.Fatalf("operation action = %q, want import_url", recorder.records[0].Action)
	}
}

func TestImportURLRejectsUnsafeURLBeforeStore(t *testing.T) {
	e, store, recorder, _ := newFileEcho(t)

	rec := doJSON(t, e, http.MethodPost, "/api/files/import-url", `{"url":"javascript:alert(1)"}`, 7)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("import unsafe url status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.createCalls != 0 {
		t.Fatalf("createCalls = %d, want 0", store.createCalls)
	}
	// Failed operations are audited too, marked unsuccessful.
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1 failed record", len(recorder.records))
	}
	if recorder.records[0].Success {
		t.Fatal("operation record Success = true, want false for rejected import")
	}
}

func TestListFilesPassesCategoryFilter(t *testing.T) {
	e, store, _, _ := newFileEcho(t)

	req := httptest.NewRequest(http.MethodGet, "/api/files?category_id=5", nil)
	req = req.WithContext(requestctx.WithUserID(req.Context(), 7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list files status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.listFilter.CategoryID != 5 {
		t.Fatalf("category filter = %d, want 5", store.listFilter.CategoryID)
	}
}

func TestServeUploadReturnsStoredFile(t *testing.T) {
	e, _, _, uploadDir := newFileEcho(t)
	if err := os.WriteFile(filepath.Join(uploadDir, "stored.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write uploaded file fixture: %v", err)
	}

	rec := doJSON(t, e, http.MethodGet, "/api/uploads/stored.txt", "", 7)
	if rec.Code != http.StatusOK {
		t.Fatalf("serve upload status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("served upload body = %q, want hello", rec.Body.String())
	}
}

func TestServeUploadRejectsNestedPath(t *testing.T) {
	e, _, _, _ := newFileEcho(t)

	rec := doJSON(t, e, http.MethodGet, "/api/uploads/nested/stored.txt", "", 7)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("serve upload status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRenameFileUpdatesMetadataAndRecordsOperation(t *testing.T) {
	e, store, recorder, _ := newFileEcho(t)
	store.file = mustFile(t, 9, "old.txt", "https://cdn.example.com/old.txt")

	rec := doJSON(t, e, http.MethodPatch, "/api/files/9/name", `{"name":"new.txt"}`, 7)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename file status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.updated.Name != "new.txt" {
		t.Fatalf("updated name = %q, want new.txt", store.updated.Name)
	}
	if got := onlyOperation(t, recorder).Action; got != "rename" {
		t.Fatalf("operation action = %q, want rename", got)
	}
}

func TestDeleteFileRemovesMetadataLocalUploadAndRecordsOperation(t *testing.T) {
	e, store, recorder, uploadDir := newFileEcho(t)
	storedName := "stored.txt"
	target := filepath.Join(uploadDir, storedName)
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write uploaded file fixture: %v", err)
	}
	store.file = mustFile(t, 9, "hello.txt", "/api/uploads/"+storedName)

	rec := doJSON(t, e, http.MethodDelete, "/api/files/9", "", 7)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete file status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.deletedID != 9 {
		t.Fatalf("deleted id = %d, want 9", store.deletedID)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("stat deleted upload error = %v, want not exist", err)
	}
	if got := onlyOperation(t, recorder).Action; got != "delete" {
		t.Fatalf("operation action = %q, want delete", got)
	}
}

func TestCreateCategoryStoresMetadataAndRecordsOperation(t *testing.T) {
	e, store, recorder, _ := newFileEcho(t)

	rec := doJSON(t, e, http.MethodPost, "/api/file-categories", `{"name":"合同"}`, 7)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createdCategory.Name != "合同" {
		t.Fatalf("created category name = %q, want 合同", store.createdCategory.Name)
	}
	if got := onlyOperation(t, recorder).Resource; got != "file_category" {
		t.Fatalf("operation resource = %q, want file_category", got)
	}
}

func TestDeleteCategoryRejectsParentWithChildren(t *testing.T) {
	e, store, recorder, _ := newFileEcho(t)
	store.categories = []filedomain.FileCategory{
		mustCategory(t, 1, 0, "合同"),
		mustCategory(t, 2, 1, "采购合同"),
	}

	rec := doJSON(t, e, http.MethodDelete, "/api/file-categories/1", "", 7)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete category status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.deletedCategoryID != 0 {
		t.Fatalf("deleted category id = %d, want 0", store.deletedCategoryID)
	}
	// Failed operations are audited too, marked unsuccessful.
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1 failed record", len(recorder.records))
	}
	if recorder.records[0].Success {
		t.Fatal("operation record Success = true, want false for rejected delete")
	}
}

func newFileEcho(t *testing.T) (*echo.Echo, *fileStore, *operationRecorder, string) {
	t.Helper()
	store := &fileStore{}
	uploadDir := t.TempDir()
	uc := fileusecase.New(store, localstorage.New(uploadDir))
	recorder := &operationRecorder{}
	handler := filehttp.New(uc, oprec.New(recorder))

	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	filehttp.Register(e.Group("/api"), handler)
	return e, store, recorder, uploadDir
}

func doMultipart(t *testing.T, e *echo.Echo, path, filename, body string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	return doMultipartWithFields(t, e, path, filename, body, userID, nil)
}

func doMultipartWithFields(t *testing.T, e *echo.Echo, path, filename, body string, userID int64, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte(body)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write multipart field %s: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &payload)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	req = req.WithContext(requestctx.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(requestctx.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

type fileStore struct {
	createCalls       int
	created           filedomain.FileObject
	file              filedomain.FileObject
	updated           filedomain.FileObject
	deletedID         int64
	listFilter        fileusecase.ListFilter
	categories        []filedomain.FileCategory
	createdCategory   filedomain.FileCategory
	updatedCategory   filedomain.FileCategory
	deletedCategoryID int64
	categoryNameTaken bool
}

func (s *fileStore) CreateFile(ctx context.Context, file filedomain.FileObject) (filedomain.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return filedomain.FileObject{}, err
	}
	s.createCalls++
	s.created = file
	return filedomain.RestoreFileObject(1, file.Name, file.URL, file.Size, file.ContentType, file.CategoryID, time.Unix(1_800_000_000, 0).UTC())
}

func (s *fileStore) ListFiles(ctx context.Context, filter fileusecase.ListFilter) ([]filedomain.FileObject, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.listFilter = filter
	return nil, 0, nil
}

func (s *fileStore) FindFileByID(ctx context.Context, _ int64) (filedomain.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return filedomain.FileObject{}, err
	}
	return s.file, nil
}

func (s *fileStore) UpdateFile(ctx context.Context, file filedomain.FileObject) (filedomain.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return filedomain.FileObject{}, err
	}
	s.updated = file
	return file, nil
}

func (s *fileStore) DeleteFile(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedID = id
	return nil
}

func (s *fileStore) CreateCategory(ctx context.Context, category filedomain.FileCategory) (filedomain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return filedomain.FileCategory{}, err
	}
	s.createdCategory = category
	return filedomain.RestoreFileCategory(11, category.ParentID, category.Name, time.Unix(1_800_000_000, 0).UTC(), time.Unix(1_800_000_000, 0).UTC())
}

func (s *fileStore) UpdateCategory(ctx context.Context, category filedomain.FileCategory) (filedomain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return filedomain.FileCategory{}, err
	}
	s.updatedCategory = category
	return category, nil
}

func (s *fileStore) FindCategoryByID(ctx context.Context, id int64) (filedomain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return filedomain.FileCategory{}, err
	}
	for _, category := range s.categories {
		if category.ID == id {
			return category, nil
		}
	}
	return filedomain.FileCategory{}, apperr.NewNotFound("file category")
}

func (s *fileStore) ListCategories(ctx context.Context) ([]filedomain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.categories, nil
}

func (s *fileStore) DeleteCategory(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedCategoryID = id
	return nil
}

func (s *fileStore) CategoryNameExists(ctx context.Context, _ string, _, _ int64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return s.categoryNameTaken, nil
}

type operationRecorder struct {
	records []auditusecase.OperationInput
}

func (r *operationRecorder) RecordOperation(_ context.Context, input auditusecase.OperationInput) (auditusecase.OperationLog, error) {
	r.records = append(r.records, input)
	return auditusecase.OperationLog{}, nil
}

func mustFile(t *testing.T, id int64, name, url string) filedomain.FileObject {
	t.Helper()
	file, err := filedomain.RestoreFileObject(id, name, url, 5, "text/plain", 0, time.Unix(1_800_000_000, 0).UTC())
	if err != nil {
		t.Fatalf("RestoreFileObject() error = %v", err)
	}
	return file
}

func mustCategory(t *testing.T, id, parentID int64, name string) filedomain.FileCategory {
	t.Helper()
	category, err := filedomain.RestoreFileCategory(id, parentID, name, time.Unix(1_800_000_000, 0).UTC(), time.Unix(1_800_000_000, 0).UTC())
	if err != nil {
		t.Fatalf("RestoreFileCategory() error = %v", err)
	}
	return category
}

func onlyOperation(t *testing.T, recorder *operationRecorder) auditusecase.OperationInput {
	t.Helper()
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	return recorder.records[0]
}

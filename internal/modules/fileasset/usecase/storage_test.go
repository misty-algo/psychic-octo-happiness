package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NSObjects/echo-admin/internal/modules/fileasset/adapters/memorystorage"
	filedomain "github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	"github.com/NSObjects/echo-admin/internal/modules/fileasset/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

type fileStorageStore struct {
	existing   []filedomain.FileObject
	nextID     int64
	created    []filedomain.FileObject
	deletedIDs []int64
	failCreate bool
}

func (s *fileStorageStore) CreateFile(_ context.Context, file filedomain.FileObject) (filedomain.FileObject, error) {
	if s.failCreate {
		return filedomain.FileObject{}, apperr.NewConflict("file already exists")
	}
	s.nextID++
	file.ID = s.nextID
	s.created = append(s.created, file)
	s.existing = append(s.existing, file)
	return file, nil
}

func (s *fileStorageStore) FindFileByID(_ context.Context, id int64) (filedomain.FileObject, error) {
	for _, file := range s.existing {
		if file.ID == id {
			return file, nil
		}
	}
	return filedomain.FileObject{}, apperr.NewNotFound("file")
}

func (s *fileStorageStore) ListFiles(context.Context, usecase.ListFilter) ([]filedomain.FileObject, int, error) {
	return nil, 0, nil
}

func (s *fileStorageStore) UpdateFile(_ context.Context, file filedomain.FileObject) (filedomain.FileObject, error) {
	return file, nil
}

func (s *fileStorageStore) DeleteFile(_ context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	for i, file := range s.existing {
		if file.ID == id {
			s.existing = append(s.existing[:i], s.existing[i+1:]...)
			break
		}
	}
	return nil
}

func (s *fileStorageStore) CreateCategory(context.Context, filedomain.FileCategory) (filedomain.FileCategory, error) {
	return filedomain.FileCategory{}, nil
}

func (s *fileStorageStore) UpdateCategory(context.Context, filedomain.FileCategory) (filedomain.FileCategory, error) {
	return filedomain.FileCategory{}, nil
}

func (s *fileStorageStore) FindCategoryByID(context.Context, int64) (filedomain.FileCategory, error) {
	return filedomain.FileCategory{}, nil
}

func (s *fileStorageStore) ListCategories(context.Context) ([]filedomain.FileCategory, error) {
	return nil, nil
}

func (s *fileStorageStore) DeleteCategory(context.Context, int64) error { return nil }

func (s *fileStorageStore) CategoryNameExists(context.Context, string, int64, int64) (bool, error) {
	return false, nil
}

func TestCreateFileStoresBytesAndMetadata(t *testing.T) {
	store := &fileStorageStore{}
	storage := memorystorage.New()
	uc := usecase.New(store, storage)

	file, err := uc.CreateFile(context.Background(), usecase.FileInput{},
		usecase.FileSource{Name: "合同.pdf", ContentType: "application/pdf", Reader: strings.NewReader("bytes")})
	if err != nil {
		t.Fatalf("CreateFile() error = %v, want nil", err)
	}
	if file.URL == "" || file.Size != 5 {
		t.Fatalf("file = %+v, want stored URL and size 5", file)
	}
	if got := storage.Stored(); len(got) != 1 {
		t.Fatalf("stored names = %v, want one entry", got)
	}
	if len(store.created) != 1 {
		t.Fatalf("metadata records = %d, want 1", len(store.created))
	}
	if storage.Removed() != nil {
		t.Fatalf("removed names = %v, want none on success", storage.Removed())
	}
}

func TestCreateFileRemovesBytesWhenMetadataFails(t *testing.T) {
	store := &fileStorageStore{failCreate: true}
	storage := memorystorage.New()
	uc := usecase.New(store, storage)

	_, err := uc.CreateFile(context.Background(), usecase.FileInput{},
		usecase.FileSource{Name: "合同.pdf", ContentType: "application/pdf", Reader: strings.NewReader("bytes")})
	if err == nil {
		t.Fatal("CreateFile() error = nil, want metadata failure")
	}
	if got := storage.Removed(); len(got) != 1 || got[0] != "stored-合同.pdf" {
		t.Fatalf("removed names = %v, want [stored-合同.pdf] so no orphaned bytes survive", got)
	}
}

func TestCreateFileLeavesNoMetadataWhenBytesFail(t *testing.T) {
	store := &fileStorageStore{}
	storage := memorystorage.New()
	storage.FailOnStore("毒", errors.New("disk full"))
	uc := usecase.New(store, storage)

	_, err := uc.CreateFile(context.Background(), usecase.FileInput{},
		usecase.FileSource{Name: "毒.pdf", ContentType: "application/pdf", Reader: strings.NewReader("bytes")})
	if err == nil {
		t.Fatal("CreateFile() error = nil, want byte-store failure")
	}
	if len(store.created) != 0 {
		t.Fatalf("metadata records = %d, want 0 when bytes fail", len(store.created))
	}
	if storage.Removed() != nil {
		t.Fatalf("removed names = %v, want none when Store failed before writing", storage.Removed())
	}
}

func TestDeleteFileRemovesStoredBytes(t *testing.T) {
	created := filedomain.FileObject{ID: 3, Name: "a", URL: "/api/uploads/stored-a", Size: 1}
	store := &fileStorageStore{existing: []filedomain.FileObject{created}, nextID: 3}
	storage := memorystorage.New()
	uc := usecase.New(store, storage)

	if _, err := uc.DeleteFile(context.Background(), 3); err != nil {
		t.Fatalf("DeleteFile() error = %v, want nil", err)
	}
	if got := storage.Removed(); len(got) != 1 || got[0] != "stored-a" {
		t.Fatalf("removed names = %v, want [stored-a]", got)
	}
}

func TestDeleteFileKeepsExternalURLsUntouched(t *testing.T) {
	created := filedomain.FileObject{ID: 4, Name: "b", URL: "https://cdn.example.com/b.png"}
	store := &fileStorageStore{existing: []filedomain.FileObject{created}, nextID: 4}
	storage := memorystorage.New()
	uc := usecase.New(store, storage)

	if _, err := uc.DeleteFile(context.Background(), 4); err != nil {
		t.Fatalf("DeleteFile() error = %v, want nil", err)
	}
	if got := storage.Removed(); got != nil {
		t.Fatalf("removed names = %v, want none for external URLs", got)
	}
}

func TestCreateFileRejectsOversizedUpload(t *testing.T) {
	store := &fileStorageStore{}
	storage := memorystorage.New()
	uc := usecase.New(store, storage, usecase.WithMaxUploadBytes(2))

	_, err := uc.CreateFile(context.Background(), usecase.FileInput{},
		usecase.FileSource{Name: "big.bin", ContentType: "application/octet-stream", Reader: strings.NewReader("too many bytes")})
	if def, ok := apperr.ParseRegistered(err); !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("CreateFile(oversized) error = %v, want bad request", err)
	}
	if len(store.created) != 0 {
		t.Fatalf("metadata records = %d, want 0 for oversized upload", len(store.created))
	}
}

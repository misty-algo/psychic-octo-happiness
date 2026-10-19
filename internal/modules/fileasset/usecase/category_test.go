package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/fileasset/adapters/memorystorage"
	"github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

func TestCreateCategoryRejectsDuplicateNameUnderSameParent(t *testing.T) {
	store := &categoryStore{categoryNameTaken: true}
	uc := New(store, memorystorage.New())

	_, err := uc.CreateCategory(context.Background(), CategoryInput{Name: "合同"})
	if code := appCode(t, err); code != apperr.ErrConflict {
		t.Fatalf("CreateCategory() code = %d, want %d", code, apperr.ErrConflict)
	}
	if store.createdCategory.ID != 0 {
		t.Fatalf("created category id = %d, want 0", store.createdCategory.ID)
	}
}

func TestUpdateCategoryRejectsCycle(t *testing.T) {
	store := &categoryStore{categories: []domain.FileCategory{
		mustUsecaseCategory(t, 1, 0, "合同"),
		mustUsecaseCategory(t, 2, 1, "采购合同"),
	}}
	uc := New(store, memorystorage.New())

	_, err := uc.UpdateCategory(context.Background(), UpdateCategoryInput{ID: 1, Name: "合同", ParentID: 2})
	if code := appCode(t, err); code != apperr.ErrBadRequest {
		t.Fatalf("UpdateCategory() code = %d, want %d", code, apperr.ErrBadRequest)
	}
	if store.updatedCategory.ID != 0 {
		t.Fatalf("updated category id = %d, want 0", store.updatedCategory.ID)
	}
}

func TestDeleteCategoryRejectsParentWithChildren(t *testing.T) {
	store := &categoryStore{categories: []domain.FileCategory{
		mustUsecaseCategory(t, 1, 0, "合同"),
		mustUsecaseCategory(t, 2, 1, "采购合同"),
	}}
	uc := New(store, memorystorage.New())

	err := uc.DeleteCategory(context.Background(), 1)
	if code := appCode(t, err); code != apperr.ErrBadRequest {
		t.Fatalf("DeleteCategory() code = %d, want %d", code, apperr.ErrBadRequest)
	}
	if store.deletedCategoryID != 0 {
		t.Fatalf("deleted category id = %d, want 0", store.deletedCategoryID)
	}
}

type categoryStore struct {
	categoryNameTaken bool
	categories        []domain.FileCategory
	createdCategory   domain.FileCategory
	updatedCategory   domain.FileCategory
	deletedCategoryID int64
}

func (s *categoryStore) CreateFile(context.Context, domain.FileObject) (domain.FileObject, error) {
	return domain.FileObject{}, nil
}

func (s *categoryStore) FindFileByID(context.Context, int64) (domain.FileObject, error) {
	return domain.FileObject{}, nil
}

func (s *categoryStore) ListFiles(context.Context, ListFilter) ([]domain.FileObject, int, error) {
	return nil, 0, nil
}

func (s *categoryStore) UpdateFile(context.Context, domain.FileObject) (domain.FileObject, error) {
	return domain.FileObject{}, nil
}

func (s *categoryStore) DeleteFile(context.Context, int64) error {
	return nil
}

func (s *categoryStore) CreateCategory(_ context.Context, category domain.FileCategory) (domain.FileCategory, error) {
	s.createdCategory = category
	return category, nil
}

func (s *categoryStore) UpdateCategory(_ context.Context, category domain.FileCategory) (domain.FileCategory, error) {
	s.updatedCategory = category
	return category, nil
}

func (s *categoryStore) FindCategoryByID(_ context.Context, id int64) (domain.FileCategory, error) {
	for _, category := range s.categories {
		if category.ID == id {
			return category, nil
		}
	}
	return domain.FileCategory{}, apperr.NewNotFound("file category")
}

func (s *categoryStore) ListCategories(context.Context) ([]domain.FileCategory, error) {
	return s.categories, nil
}

func (s *categoryStore) DeleteCategory(_ context.Context, id int64) error {
	s.deletedCategoryID = id
	return nil
}

func (s *categoryStore) CategoryNameExists(context.Context, string, int64, int64) (bool, error) {
	return s.categoryNameTaken, nil
}

func mustUsecaseCategory(t *testing.T, id, parentID int64, name string) domain.FileCategory {
	t.Helper()
	category, err := domain.RestoreFileCategory(id, parentID, name, time.Unix(1_800_000_000, 0).UTC(), time.Unix(1_800_000_000, 0).UTC())
	if err != nil {
		t.Fatalf("RestoreFileCategory() error = %v", err)
	}
	return category
}

func appCode(t *testing.T, err error) int {
	t.Helper()
	appErr, ok := apperr.Parse(err)
	if !ok {
		t.Fatalf("error = %v, want app error", err)
	}
	return appErr.Code()
}

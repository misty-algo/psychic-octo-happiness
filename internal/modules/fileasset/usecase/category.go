package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// ListCategories returns the operator-managed category tree.
func (u *Usecase) ListCategories(ctx context.Context) ([]Category, error) {
	if err := u.ready(); err != nil {
		return nil, err
	}
	categories, err := u.store.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	return buildCategoryTree(categories), nil
}

// CreateCategory creates one file category under the requested parent.
func (u *Usecase) CreateCategory(ctx context.Context, input CategoryInput) (Category, error) {
	if err := u.ready(); err != nil {
		return Category{}, err
	}
	category, err := domain.RestoreFileCategory(0, input.ParentID, input.Name, time.Time{}, time.Time{})
	if err != nil {
		return Category{}, mapDomainError(err)
	}
	if parentErr := u.validateCategoryParent(ctx, category.ParentID); parentErr != nil {
		return Category{}, parentErr
	}
	if duplicateErr := u.ensureCategoryNameAvailable(ctx, category.Name, category.ParentID, 0); duplicateErr != nil {
		return Category{}, duplicateErr
	}
	created, err := u.store.CreateCategory(ctx, category)
	if err != nil {
		return Category{}, err
	}
	return fromCategory(created), nil
}

// UpdateCategory updates one category name or parent without creating cycles.
func (u *Usecase) UpdateCategory(ctx context.Context, input UpdateCategoryInput) (Category, error) {
	if err := u.ready(); err != nil {
		return Category{}, err
	}
	if input.ID <= 0 {
		return Category{}, apperr.NewBadRequest("invalid category id")
	}
	existing, err := u.store.FindCategoryByID(ctx, input.ID)
	if err != nil {
		return Category{}, err
	}
	category, err := domain.RestoreFileCategory(existing.ID, input.ParentID, input.Name, existing.CreatedAt, existing.UpdatedAt)
	if err != nil {
		return Category{}, mapDomainError(err)
	}
	if moveErr := u.validateCategoryMove(ctx, category.ID, category.ParentID); moveErr != nil {
		return Category{}, moveErr
	}
	if duplicateErr := u.ensureCategoryNameAvailable(ctx, category.Name, category.ParentID, category.ID); duplicateErr != nil {
		return Category{}, duplicateErr
	}
	updated, err := u.store.UpdateCategory(ctx, category)
	if err != nil {
		return Category{}, err
	}
	return fromCategory(updated), nil
}

// DeleteCategory removes one category while preserving files in that category.
func (u *Usecase) DeleteCategory(ctx context.Context, id int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	if id <= 0 {
		return apperr.NewBadRequest("invalid category id")
	}
	categories, err := u.store.ListCategories(ctx)
	if err != nil {
		return err
	}
	if !categoryExists(categories, id) {
		return apperr.NewNotFound("file category")
	}
	if categoryHasChildren(categories, id) {
		return apperr.NewBadRequest("category has children")
	}
	return u.store.DeleteCategory(ctx, id)
}

func (u *Usecase) validateCategoryParent(ctx context.Context, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	_, err := u.store.FindCategoryByID(ctx, parentID)
	return err
}

func (u *Usecase) validateCategoryMove(ctx context.Context, id, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	if parentID == id {
		return apperr.NewBadRequest("category cannot be its own parent")
	}
	categories, err := u.store.ListCategories(ctx)
	if err != nil {
		return err
	}
	if !categoryExists(categories, parentID) {
		return apperr.NewNotFound("file category")
	}
	if categoryParentCreatesCycle(categories, id, parentID) {
		return apperr.NewBadRequest("category parent creates a cycle")
	}
	return nil
}

func (u *Usecase) ensureCategoryNameAvailable(ctx context.Context, name string, parentID, excludeID int64) error {
	exists, err := u.store.CategoryNameExists(ctx, name, parentID, excludeID)
	if err != nil {
		return err
	}
	if exists {
		return apperr.NewConflict("file category name already exists")
	}
	return nil
}

func buildCategoryTree(categories []domain.FileCategory) []Category {
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].ParentID == categories[j].ParentID {
			return categories[i].ID < categories[j].ID
		}
		return categories[i].ParentID < categories[j].ParentID
	})
	childrenByParent := make(map[int64][]domain.FileCategory, len(categories))
	for _, category := range categories {
		childrenByParent[category.ParentID] = append(childrenByParent[category.ParentID], category)
	}
	return categoryChildren(childrenByParent, 0)
}

func categoryChildren(childrenByParent map[int64][]domain.FileCategory, parentID int64) []Category {
	children := childrenByParent[parentID]
	out := make([]Category, 0, len(children))
	for _, child := range children {
		item := fromCategory(child)
		item.Children = categoryChildren(childrenByParent, child.ID)
		out = append(out, item)
	}
	return out
}

func fromCategory(category domain.FileCategory) Category {
	return Category{
		ID:        category.ID,
		ParentID:  category.ParentID,
		Name:      category.Name,
		CreatedAt: category.CreatedAt,
		UpdatedAt: category.UpdatedAt,
	}
}

func categoryExists(categories []domain.FileCategory, id int64) bool {
	for _, category := range categories {
		if category.ID == id {
			return true
		}
	}
	return false
}

func categoryHasChildren(categories []domain.FileCategory, id int64) bool {
	for _, category := range categories {
		if category.ParentID == id {
			return true
		}
	}
	return false
}

func categoryParentCreatesCycle(categories []domain.FileCategory, id, parentID int64) bool {
	current := parentID
	for current != 0 {
		if current == id {
			return true
		}
		next := int64(0)
		for _, category := range categories {
			if category.ID == current {
				next = category.ParentID
				break
			}
		}
		current = next
	}
	return false
}

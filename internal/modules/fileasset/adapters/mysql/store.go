// Package mysql persists uploaded file metadata in MySQL.
package mysql

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/modules/fileasset/domain"
	"github.com/NSObjects/echo-admin/internal/modules/fileasset/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// Store persists uploaded file metadata in MySQL.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the MySQL file metadata and category tables.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create file store: nil context")
	}
	if db == nil {
		return nil, errors.New("create file store: nil db")
	}
	if err := db.WithContext(ctx).AutoMigrate(&categoryModel{}, &fileModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate file tables")
	}
	return &Store{db: db}, nil
}

// CreateFile inserts uploaded file metadata.
func (s *Store) CreateFile(ctx context.Context, file domain.FileObject) (domain.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return domain.FileObject{}, err
	}
	model := fileModelFromDomain(file, time.Now().UTC())
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.FileObject{}, apperr.WrapDatabase(err, "create file")
	}
	return model.toDomain()
}

// FindFileByID returns one file metadata record.
func (s *Store) FindFileByID(ctx context.Context, id int64) (domain.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return domain.FileObject{}, err
	}
	var model fileModel
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.FileObject{}, apperr.NewNotFound("file")
		}
		return domain.FileObject{}, apperr.WrapDatabase(err, "find file")
	}
	return model.toDomain()
}

// ListFiles returns uploaded file metadata ordered by id descending.
func (s *Store) ListFiles(ctx context.Context, filter usecase.ListFilter) ([]domain.FileObject, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	var total int64
	query := s.db.WithContext(ctx).Model(&fileModel{})
	if filter.CategoryID > 0 {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperr.WrapDatabase(err, "count files")
	}
	var models []fileModel
	err := query.Order("id DESC").Offset(filter.Offset).Limit(filter.Limit).Find(&models).Error
	if err != nil {
		return nil, 0, apperr.WrapDatabase(err, "list files")
	}
	files := make([]domain.FileObject, 0, len(models))
	for _, model := range models {
		file, err := model.toDomain()
		if err != nil {
			return nil, 0, err
		}
		files = append(files, file)
	}
	return files, int(total), nil
}

// UpdateFile replaces mutable file metadata fields.
func (s *Store) UpdateFile(ctx context.Context, file domain.FileObject) (domain.FileObject, error) {
	if err := ctx.Err(); err != nil {
		return domain.FileObject{}, err
	}
	model := fileModelFromDomain(file, time.Now().UTC())
	result := s.db.WithContext(ctx).Save(&model)
	if result.Error != nil {
		return domain.FileObject{}, apperr.WrapDatabase(result.Error, "update file")
	}
	if result.RowsAffected == 0 {
		return domain.FileObject{}, apperr.NewNotFound("file")
	}
	return model.toDomain()
}

// DeleteFile removes one file metadata record.
func (s *Store) DeleteFile(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Delete(&fileModel{}, "id = ?", id)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete file")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("file")
	}
	return nil
}

// CreateCategory inserts one file category.
func (s *Store) CreateCategory(ctx context.Context, category domain.FileCategory) (domain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return domain.FileCategory{}, err
	}
	model := categoryModelFromDomain(category, time.Now().UTC())
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.FileCategory{}, apperr.WrapDatabase(err, "create file category")
	}
	return model.toDomain()
}

// UpdateCategory updates one file category.
func (s *Store) UpdateCategory(ctx context.Context, category domain.FileCategory) (domain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return domain.FileCategory{}, err
	}
	var existing categoryModel
	if err := s.db.WithContext(ctx).First(&existing, "id = ?", category.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.FileCategory{}, apperr.NewNotFound("file category")
		}
		return domain.FileCategory{}, apperr.WrapDatabase(err, "find file category")
	}
	result := s.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"name":      category.Name,
		"parent_id": category.ParentID,
	})
	if result.Error != nil {
		return domain.FileCategory{}, apperr.WrapDatabase(result.Error, "update file category")
	}
	return s.FindCategoryByID(ctx, category.ID)
}

// FindCategoryByID returns one file category.
func (s *Store) FindCategoryByID(ctx context.Context, id int64) (domain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return domain.FileCategory{}, err
	}
	var model categoryModel
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.FileCategory{}, apperr.NewNotFound("file category")
		}
		return domain.FileCategory{}, apperr.WrapDatabase(err, "find file category")
	}
	return model.toDomain()
}

// ListCategories returns all file categories ordered for stable tree assembly.
func (s *Store) ListCategories(ctx context.Context) ([]domain.FileCategory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []categoryModel
	err := s.db.WithContext(ctx).Order("parent_id ASC, id ASC").Find(&models).Error
	if err != nil {
		return nil, apperr.WrapDatabase(err, "list file categories")
	}
	categories := make([]domain.FileCategory, 0, len(models))
	for _, model := range models {
		category, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, nil
}

// DeleteCategory removes one category. Files stay available and become
// uncategorized, so deleting taxonomy cannot delete uploaded assets.
func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&fileModel{}).Where("category_id = ?", id).Update("category_id", 0).Error; err != nil {
			return err
		}
		result := tx.Delete(&categoryModel{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return apperr.NewNotFound("file category")
		}
		return nil
	})
	if err != nil {
		appErr, ok := apperr.Parse(err)
		if ok && appErr != nil {
			return err
		}
		return apperr.WrapDatabase(err, "delete file category")
	}
	return nil
}

// CategoryNameExists reports duplicate names under the same parent.
func (s *Store) CategoryNameExists(ctx context.Context, name string, parentID, excludeID int64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	query := s.db.WithContext(ctx).Model(&categoryModel{}).Where("name = ? AND parent_id = ?", strings.TrimSpace(name), parentID)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return false, apperr.WrapDatabase(err, "check file category name")
	}
	return total > 0, nil
}

type fileModel struct {
	ID          int64     `gorm:"primaryKey"`
	Name        string    `gorm:"type:varchar(180);not null"`
	URL         string    `gorm:"type:varchar(2048);not null"`
	Size        int64     `gorm:"not null"`
	ContentType string    `gorm:"type:varchar(120);not null"`
	CategoryID  int64     `gorm:"not null;default:0;index"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (fileModel) TableName() string {
	return "file_assets"
}

func fileModelFromDomain(file domain.FileObject, createdAt time.Time) fileModel {
	return fileModel{
		ID:          file.ID,
		Name:        file.Name,
		URL:         file.URL,
		Size:        file.Size,
		ContentType: file.ContentType,
		CategoryID:  file.CategoryID,
		CreatedAt:   coalesceTime(file.CreatedAt, createdAt),
	}
}

func (m fileModel) toDomain() (domain.FileObject, error) {
	return domain.RestoreFileObject(m.ID, m.Name, m.URL, m.Size, m.ContentType, m.CategoryID, m.CreatedAt)
}

type categoryModel struct {
	ID        int64     `gorm:"primaryKey"`
	ParentID  int64     `gorm:"not null;default:0;index;uniqueIndex:uniq_file_categories_parent_name"`
	Name      string    `gorm:"type:varchar(80);not null;uniqueIndex:uniq_file_categories_parent_name"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (categoryModel) TableName() string {
	return "file_categories"
}

func categoryModelFromDomain(category domain.FileCategory, now time.Time) categoryModel {
	return categoryModel{
		ID:        category.ID,
		ParentID:  category.ParentID,
		Name:      category.Name,
		CreatedAt: coalesceTime(category.CreatedAt, now),
		UpdatedAt: coalesceTime(category.UpdatedAt, now),
	}
}

func (m categoryModel) toDomain() (domain.FileCategory, error) {
	return domain.RestoreFileCategory(m.ID, m.ParentID, m.Name, m.CreatedAt, m.UpdatedAt)
}

func coalesceTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

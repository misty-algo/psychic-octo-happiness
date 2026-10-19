// Package mysql persists API token metadata in MySQL.
package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/modules/apitoken/domain"
	"github.com/NSObjects/echo-admin/internal/modules/apitoken/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	inframysql "github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysql"
)

// Store persists API tokens in MySQL.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the API token table.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create api token store: nil context")
	}
	if db == nil {
		return nil, errors.New("create api token store: nil db")
	}
	if err := db.WithContext(ctx).AutoMigrate(&tokenModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate api token table")
	}
	return &Store{db: db}, nil
}

// ListTokens returns token metadata ordered by creation time.
func (s *Store) ListTokens(ctx context.Context, filter usecase.ListFilter) ([]domain.APIToken, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	var total int64
	base := tokenQuery(s.db.WithContext(ctx).Model(&tokenModel{}), filter)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, apperr.WrapDatabase(err, "count api tokens")
	}
	var models []tokenModel
	err := tokenQuery(s.db.WithContext(ctx).Model(&tokenModel{}), filter).
		Order("created_at DESC, id DESC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&models).Error
	if err != nil {
		return nil, 0, apperr.WrapDatabase(err, "list api tokens")
	}
	tokens := make([]domain.APIToken, 0, len(models))
	for _, model := range models {
		token, err := model.toDomain()
		if err != nil {
			return nil, 0, err
		}
		tokens = append(tokens, token)
	}
	return tokens, int(total), nil
}

// FindTokenByID returns one token by id.
func (s *Store) FindTokenByID(ctx context.Context, id int64) (domain.APIToken, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIToken{}, err
	}
	var model tokenModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return domain.APIToken{}, inframysql.MapReadError(err, "api token", "find api token")
	}
	return model.toDomain()
}

// FindTokenByHash returns one token by its secret hash.
func (s *Store) FindTokenByHash(ctx context.Context, hash string) (domain.APIToken, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIToken{}, err
	}
	var model tokenModel
	if err := s.db.WithContext(ctx).First(&model, "secret_hash = ?", hash).Error; err != nil {
		return domain.APIToken{}, inframysql.MapReadError(err, "api token", "find api token by hash")
	}
	return model.toDomain()
}

// CreateToken inserts one token record.
func (s *Store) CreateToken(ctx context.Context, token domain.APIToken) (domain.APIToken, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIToken{}, err
	}
	now := time.Now().UTC()
	model := tokenModelFromDomain(token, now)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.APIToken{}, inframysql.MapWriteError(err, "api token already exists", "create api token")
	}
	return model.toDomain()
}

// UpdateToken replaces mutable token metadata while preserving the secret hash.
func (s *Store) UpdateToken(ctx context.Context, token domain.APIToken) (domain.APIToken, error) {
	if err := ctx.Err(); err != nil {
		return domain.APIToken{}, err
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&tokenModel{}).
		Where("id = ?", token.ID).
		Updates(map[string]interface{}{
			"name":        token.Name,
			"description": token.Description,
			"active":      token.Active,
			"expires_at":  token.ExpiresAt,
			"updated_at":  now,
		})
	if result.Error != nil {
		return domain.APIToken{}, inframysql.MapWriteError(result.Error, "api token already exists", "update api token")
	}
	if result.RowsAffected == 0 {
		return domain.APIToken{}, apperr.NewNotFound("api token")
	}
	return s.FindTokenByID(ctx, token.ID)
}

// DeleteToken revokes one token by id.
func (s *Store) DeleteToken(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&tokenModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"active":     false,
			"updated_at": now,
		})
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "revoke api token")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("api token")
	}
	return nil
}

func tokenQuery(db *gorm.DB, filter usecase.ListFilter) *gorm.DB {
	if filter.AdminID > 0 {
		db = db.Where("admin_id = ?", filter.AdminID)
	}
	if filter.Active != nil {
		db = db.Where("active = ?", *filter.Active)
	}
	return db
}

// TouchLastUsed updates the last-used timestamp after successful token authentication.
func (s *Store) TouchLastUsed(ctx context.Context, id int64, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Model(&tokenModel{}).
		Where("id = ?", id).
		Update("last_used_at", at.UTC())
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "touch api token last used")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("api token")
	}
	return nil
}

type tokenModel struct {
	ID          int64      `gorm:"primaryKey"`
	AdminID     int64      `gorm:"not null;index"`
	RoleID      int64      `gorm:"not null;index"`
	Name        string     `gorm:"type:varchar(80);not null"`
	Description string     `gorm:"type:varchar(240);not null"`
	Prefix      string     `gorm:"type:varchar(32);not null;index"`
	SecretHash  string     `gorm:"type:char(64);not null;uniqueIndex"`
	Active      bool       `gorm:"not null;index"`
	ExpiresAt   *time.Time `gorm:"index"`
	LastUsedAt  *time.Time
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (tokenModel) TableName() string {
	return "api_tokens"
}

func tokenModelFromDomain(token domain.APIToken, now time.Time) tokenModel {
	return tokenModel{
		ID:          token.ID,
		AdminID:     token.AdminID,
		RoleID:      token.RoleID,
		Name:        token.Name,
		Description: token.Description,
		Prefix:      token.Prefix,
		SecretHash:  token.SecretHash,
		Active:      token.Active,
		ExpiresAt:   copyTimePtr(token.ExpiresAt),
		LastUsedAt:  copyTimePtr(token.LastUsedAt),
		CreatedAt:   coalesceTime(token.CreatedAt, now),
		UpdatedAt:   coalesceTime(token.UpdatedAt, now),
	}
}

func (m tokenModel) toDomain() (domain.APIToken, error) {
	return domain.RestoreAPIToken(m.ID, m.AdminID, m.RoleID, m.Name, m.Description, m.Prefix, m.SecretHash, m.Active, m.ExpiresAt, m.LastUsedAt, m.CreatedAt, m.UpdatedAt)
}

func coalesceTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}

func copyTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

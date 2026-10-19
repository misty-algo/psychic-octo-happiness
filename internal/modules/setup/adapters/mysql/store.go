// Package mysql persists setup installation state in MySQL.
package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	inframysql "github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysql"
)

const installationStateKey = "system"

// Store persists setup installation state.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the setup installation-state table.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create setup store: nil context")
	}
	if db == nil {
		return nil, errors.New("create setup store: nil db")
	}
	if err := db.WithContext(ctx).AutoMigrate(&installationStateModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate setup tables")
	}
	return &Store{db: db}, nil
}

// WithDB returns a store bound to db for transaction-scoped setup operations.
func (s *Store) WithDB(db *gorm.DB) *Store {
	return &Store{db: db}
}

// Initialized reports whether system first initialization has completed.
func (s *Store) Initialized(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var model installationStateModel
	err := s.db.WithContext(ctx).First(&model, "`key` = ?", installationStateKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, apperr.WrapDatabase(err, "read installation state")
	}
	return model.CompletedAt != nil, nil
}

// RequireOpenInstallation locks or creates the singleton installation state and
// rejects setup when initialization has already completed.
func (s *Store) RequireOpenInstallation(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state, err := s.lockInstallationState(ctx)
	if err != nil {
		return err
	}
	if state.CompletedAt != nil {
		return apperr.NewConflict("system is already initialized")
	}
	return nil
}

// CompleteInstallation marks setup as completed.
func (s *Store) CompleteInstallation(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).
		Model(&installationStateModel{}).
		Where("`key` = ? AND completed_at IS NULL", installationStateKey).
		Updates(map[string]interface{}{
			"completed_at": &now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "complete installation state")
	}
	if result.RowsAffected == 0 {
		return apperr.NewConflict("system is already initialized")
	}
	return nil
}

func (s *Store) lockInstallationState(ctx context.Context) (installationStateModel, error) {
	var model installationStateModel
	err := s.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&model, "`key` = ?", installationStateKey).Error
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return installationStateModel{}, apperr.WrapDatabase(err, "lock installation state")
	}
	model = installationStateModel{
		Key:       installationStateKey,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if createErr := s.db.WithContext(ctx).Create(&model).Error; createErr != nil {
		if inframysql.IsDuplicateKey(createErr) {
			return s.lockInstallationState(ctx)
		}
		return installationStateModel{}, apperr.WrapDatabase(createErr, "create installation state")
	}
	return model, nil
}

type installationStateModel struct {
	Key         string     `gorm:"primaryKey;type:varchar(32)"`
	CompletedAt *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
}

func (installationStateModel) TableName() string {
	return "setup_installation_state"
}

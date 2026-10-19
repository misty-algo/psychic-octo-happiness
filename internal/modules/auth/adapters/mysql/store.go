// Package mysql persists authentication runtime state in MySQL.
package mysql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	authdomain "github.com/NSObjects/echo-admin/internal/modules/auth/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	inframysql "github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysql"
)

const (
	loginAttemptWindow      = 15 * time.Minute
	loginAttemptLockout     = 15 * time.Minute
	maxFailedLoginAttempts  = 5
	loginAttemptKeyMaxBytes = 128
)

// Store persists auth-owned runtime data such as login sessions and failed
// login attempt counters.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the auth runtime tables.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create auth store: nil context")
	}
	if db == nil {
		return nil, errors.New("create auth store: nil db")
	}
	if err := db.WithContext(ctx).AutoMigrate(&loginSessionModel{}, &loginAttemptModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate auth tables")
	}
	return &Store{db: db}, nil
}

// CreateLoginSession stores a new browser login session.
func (s *Store) CreateLoginSession(ctx context.Context, session authdomain.LoginSession) (authdomain.LoginSession, error) {
	if err := ctx.Err(); err != nil {
		return authdomain.LoginSession{}, err
	}
	model := loginSessionModelFromDomain(session)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return authdomain.LoginSession{}, apperr.WrapDatabase(err, "create login session")
	}
	return model.toDomain()
}

// FindLoginSessionByTokenHash reads an active or historical session by token hash.
func (s *Store) FindLoginSessionByTokenHash(ctx context.Context, tokenHash string) (authdomain.LoginSession, bool, error) {
	if err := ctx.Err(); err != nil {
		return authdomain.LoginSession{}, false, err
	}
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return authdomain.LoginSession{}, false, nil
	}
	var model loginSessionModel
	err := s.db.WithContext(ctx).First(&model, "token_hash = ?", tokenHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authdomain.LoginSession{}, false, nil
	}
	if err != nil {
		return authdomain.LoginSession{}, false, apperr.WrapDatabase(err, "find login session")
	}
	session, err := model.toDomain()
	if err != nil {
		return authdomain.LoginSession{}, false, err
	}
	return session, true, nil
}

// RefreshLoginSession updates last-seen metadata for a still-active session.
func (s *Store) RefreshLoginSession(ctx context.Context, session authdomain.LoginSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.db.WithContext(ctx).
		Model(&loginSessionModel{}).
		Where("id = ? AND revoked_at IS NULL", session.ID).
		Updates(map[string]any{
			"last_seen_at":    session.LastSeenAt,
			"idle_expires_at": session.IdleExpiresAt,
			"updated_at":      session.UpdatedAt,
		}).Error
	if err != nil {
		return apperr.WrapDatabase(err, "refresh login session")
	}
	return nil
}

// UpdateLoginSessionRole changes the active role for the current browser login.
func (s *Store) UpdateLoginSessionRole(ctx context.Context, sessionID, roleID int64, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).
		Model(&loginSessionModel{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Updates(map[string]any{
			"active_role_id": roleID,
			"updated_at":     now.UTC(),
		})
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "update login session role")
	}
	if result.RowsAffected == 0 {
		return apperr.NewUnauthorized()
	}
	return nil
}

// RevokeLoginSession revokes one login session.
func (s *Store) RevokeLoginSession(ctx context.Context, sessionID int64, reason string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.revokeSessions(ctx, now, reason, "id = ? AND revoked_at IS NULL", sessionID)
	if err != nil {
		return apperr.WrapDatabase(err, "revoke login session")
	}
	return nil
}

// RevokeOtherLoginSessions revokes every login session except the current one.
func (s *Store) RevokeOtherLoginSessions(ctx context.Context, adminID, keepSessionID int64, reason string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.revokeSessions(ctx, now, reason, "admin_id = ? AND id <> ? AND revoked_at IS NULL", adminID, keepSessionID)
	if err != nil {
		return apperr.WrapDatabase(err, "revoke other login sessions")
	}
	return nil
}

// RevokeLoginSessions revokes all login sessions for one administrator.
func (s *Store) RevokeLoginSessions(ctx context.Context, adminID int64, reason string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.revokeSessions(ctx, now, reason, "admin_id = ? AND revoked_at IS NULL", adminID)
	if err != nil {
		return apperr.WrapDatabase(err, "revoke login sessions")
	}
	return nil
}

// CheckLoginAttempt rejects a login when the hashed username/client key is
// still locked by previous failures.
func (s *Store) CheckLoginAttempt(ctx context.Context, key string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := normalizeLoginAttemptKey(key)
	if err != nil {
		return err
	}
	attempt, found, err := s.findLoginAttempt(ctx, key)
	if err != nil || !found {
		return err
	}
	now = now.UTC()
	if attempt.LockedUntil != nil && attempt.LockedUntil.After(now) {
		return apperr.New(apperr.ErrTooManyAttempts, "尝试次数过多，请稍后再试")
	}
	if loginAttemptExpired(attempt, now) {
		return s.ResetLoginAttempts(ctx, key)
	}
	return nil
}

// RecordLoginFailure increments the failure counter for one hashed login key.
func (s *Store) RecordLoginFailure(ctx context.Context, key string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := normalizeLoginAttemptKey(key)
	if err != nil {
		return err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.recordLoginFailure(ctx, tx, key, now.UTC())
	})
	if err != nil {
		return apperr.WrapDatabase(err, "record login attempt")
	}
	return nil
}

// ResetLoginAttempts clears failed-attempt state after a successful login.
func (s *Store) ResetLoginAttempts(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := normalizeLoginAttemptKey(key)
	if err != nil {
		return err
	}
	err = s.db.WithContext(ctx).Where("attempt_key = ?", key).Delete(&loginAttemptModel{}).Error
	if err != nil {
		return apperr.WrapDatabase(err, "reset login attempts")
	}
	return nil
}

type loginAttemptModel struct {
	AttemptKey    string     `gorm:"primaryKey;type:varchar(128)"`
	Failures      int        `gorm:"not null"`
	FirstFailedAt time.Time  `gorm:"not null"`
	LastFailedAt  time.Time  `gorm:"not null"`
	LockedUntil   *time.Time `gorm:"index"`
	CreatedAt     time.Time  `gorm:"not null"`
	UpdatedAt     time.Time  `gorm:"not null"`
}

func (loginAttemptModel) TableName() string {
	return "auth_login_attempts"
}

type loginSessionModel struct {
	ID                int64      `gorm:"primaryKey"`
	AdminID           int64      `gorm:"not null;index"`
	ActiveRoleID      int64      `gorm:"not null"`
	TokenHash         string     `gorm:"type:char(64);not null;uniqueIndex"`
	IP                string     `gorm:"type:varchar(64);not null"`
	UserAgent         string     `gorm:"type:varchar(512);not null"`
	CreatedAt         time.Time  `gorm:"not null"`
	LastSeenAt        time.Time  `gorm:"not null"`
	IdleExpiresAt     time.Time  `gorm:"not null;index"`
	AbsoluteExpiresAt time.Time  `gorm:"not null;index"`
	RevokedAt         *time.Time `gorm:"index"`
	RevokedReason     string     `gorm:"type:varchar(64);not null"`
	UpdatedAt         time.Time  `gorm:"not null"`
}

func (loginSessionModel) TableName() string {
	return "login_sessions"
}

func loginSessionModelFromDomain(session authdomain.LoginSession) loginSessionModel {
	return loginSessionModel{
		ID:                session.ID,
		AdminID:           session.AdminID,
		ActiveRoleID:      session.ActiveRoleID,
		TokenHash:         session.TokenHash,
		IP:                truncate(session.IP, 64),
		UserAgent:         truncate(session.UserAgent, 512),
		CreatedAt:         session.CreatedAt,
		LastSeenAt:        session.LastSeenAt,
		IdleExpiresAt:     session.IdleExpiresAt,
		AbsoluteExpiresAt: session.AbsoluteExpiresAt,
		RevokedAt:         session.RevokedAt,
		RevokedReason:     truncate(session.RevokedReason, 64),
		UpdatedAt:         session.UpdatedAt,
	}
}

func (m loginSessionModel) toDomain() (authdomain.LoginSession, error) {
	return authdomain.RestoreLoginSession(
		m.ID,
		m.AdminID,
		m.ActiveRoleID,
		m.TokenHash,
		m.IP,
		m.UserAgent,
		m.CreatedAt,
		m.LastSeenAt,
		m.IdleExpiresAt,
		m.AbsoluteExpiresAt,
		m.RevokedAt,
		m.RevokedReason,
		m.UpdatedAt,
	)
}

func (s *Store) findLoginAttempt(ctx context.Context, key string) (loginAttemptModel, bool, error) {
	var attempt loginAttemptModel
	err := s.db.WithContext(ctx).First(&attempt, "attempt_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return loginAttemptModel{}, false, nil
	}
	if err != nil {
		return loginAttemptModel{}, false, apperr.WrapDatabase(err, "find login attempt")
	}
	return attempt, true, nil
}

func (s *Store) recordLoginFailure(ctx context.Context, tx *gorm.DB, key string, now time.Time) error {
	var attempt loginAttemptModel
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "attempt_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		attempt = newLoginAttempt(key, now)
		if createErr := tx.WithContext(ctx).Create(&attempt).Error; createErr != nil {
			if inframysql.IsDuplicateKey(createErr) {
				return s.incrementExistingLoginAttempt(ctx, tx, key, now)
			}
			return createErr
		}
		return nil
	}
	if err != nil {
		return err
	}
	incrementLoginAttempt(&attempt, now)
	return tx.WithContext(ctx).Save(&attempt).Error
}

func (s *Store) incrementExistingLoginAttempt(ctx context.Context, tx *gorm.DB, key string, now time.Time) error {
	var attempt loginAttemptModel
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&attempt, "attempt_key = ?", key).Error
	if err != nil {
		return err
	}
	incrementLoginAttempt(&attempt, now)
	return tx.WithContext(ctx).Save(&attempt).Error
}

func newLoginAttempt(key string, now time.Time) loginAttemptModel {
	attempt := loginAttemptModel{
		AttemptKey:    key,
		Failures:      1,
		FirstFailedAt: now,
		LastFailedAt:  now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return attempt
}

func incrementLoginAttempt(attempt *loginAttemptModel, now time.Time) {
	if loginAttemptExpired(*attempt, now) {
		attempt.Failures = 0
		attempt.FirstFailedAt = now
		attempt.LockedUntil = nil
	}
	attempt.Failures++
	attempt.LastFailedAt = now
	attempt.UpdatedAt = now
	if attempt.Failures >= maxFailedLoginAttempts {
		lockedUntil := now.Add(loginAttemptLockout)
		attempt.LockedUntil = &lockedUntil
	}
}

func loginAttemptExpired(attempt loginAttemptModel, now time.Time) bool {
	return attempt.LastFailedAt.Before(now.Add(-loginAttemptWindow))
}

func normalizeLoginAttemptKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > loginAttemptKeyMaxBytes {
		return "", fmt.Errorf("invalid login attempt key")
	}
	return key, nil
}

func (s *Store) revokeSessions(ctx context.Context, now time.Time, reason, where string, args ...any) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "revoked"
	}
	return s.db.WithContext(ctx).
		Model(&loginSessionModel{}).
		Where(where, args...).
		Updates(map[string]any{
			"revoked_at":     now.UTC(),
			"revoked_reason": truncate(reason, 64),
			"updated_at":     now.UTC(),
		}).Error
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

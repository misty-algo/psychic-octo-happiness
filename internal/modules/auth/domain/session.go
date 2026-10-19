// Package domain defines authentication-owned business state.
package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidLoginSession indicates malformed session state.
	ErrInvalidLoginSession = errors.New("invalid login session")
	// ErrLoginSessionExpired indicates a session that passed its idle or
	// absolute expiry.
	ErrLoginSessionExpired = errors.New("login session expired")
	// ErrLoginSessionRevoked indicates a session explicitly signed out or
	// revoked by a security event.
	ErrLoginSessionRevoked = errors.New("login session revoked")
)

// LoginSession is one browser login state for an administrator. It stores the
// active role for that browser, but authorization still reads current role
// grants from the access module.
type LoginSession struct {
	ID                int64
	AdminID           int64
	ActiveRoleID      int64
	TokenHash         string
	IP                string
	UserAgent         string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	RevokedReason     string
	UpdatedAt         time.Time
}

// LoginSessionInput carries the data needed to create a login session.
type LoginSessionInput struct {
	AdminID           int64
	ActiveRoleID      int64
	TokenHash         string
	IP                string
	UserAgent         string
	CreatedAt         time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// NewLoginSession creates an active login session.
func NewLoginSession(input LoginSessionInput) (LoginSession, error) {
	session := LoginSession{
		AdminID:           input.AdminID,
		ActiveRoleID:      input.ActiveRoleID,
		TokenHash:         strings.TrimSpace(input.TokenHash),
		IP:                strings.TrimSpace(input.IP),
		UserAgent:         strings.TrimSpace(input.UserAgent),
		CreatedAt:         input.CreatedAt.UTC(),
		LastSeenAt:        input.CreatedAt.UTC(),
		IdleExpiresAt:     input.IdleExpiresAt.UTC(),
		AbsoluteExpiresAt: input.AbsoluteExpiresAt.UTC(),
		UpdatedAt:         input.CreatedAt.UTC(),
	}
	if err := session.validate(); err != nil {
		return LoginSession{}, err
	}
	return session, nil
}

// RestoreLoginSession restores persisted session state.
func RestoreLoginSession(id, adminID, activeRoleID int64, tokenHash, ip, userAgent string, createdAt, lastSeenAt, idleExpiresAt, absoluteExpiresAt time.Time, revokedAt *time.Time, revokedReason string, updatedAt time.Time) (LoginSession, error) {
	session := LoginSession{
		ID:                id,
		AdminID:           adminID,
		ActiveRoleID:      activeRoleID,
		TokenHash:         strings.TrimSpace(tokenHash),
		IP:                strings.TrimSpace(ip),
		UserAgent:         strings.TrimSpace(userAgent),
		CreatedAt:         createdAt.UTC(),
		LastSeenAt:        lastSeenAt.UTC(),
		IdleExpiresAt:     idleExpiresAt.UTC(),
		AbsoluteExpiresAt: absoluteExpiresAt.UTC(),
		RevokedAt:         utcPtr(revokedAt),
		RevokedReason:     strings.TrimSpace(revokedReason),
		UpdatedAt:         updatedAt.UTC(),
	}
	if err := session.validate(); err != nil {
		return LoginSession{}, err
	}
	return session, nil
}

// AvailabilityError returns the reason a session cannot authenticate a request.
func (s LoginSession) AvailabilityError(now time.Time) error {
	if s.RevokedAt != nil {
		return ErrLoginSessionRevoked
	}
	now = now.UTC()
	if !s.IdleExpiresAt.After(now) || !s.AbsoluteExpiresAt.After(now) {
		return ErrLoginSessionExpired
	}
	return nil
}

// NeedsRefresh reports whether last-seen metadata should be refreshed. The
// interval prevents hot admin pages from turning session reads into write load.
func (s LoginSession) NeedsRefresh(now time.Time, interval time.Duration) bool {
	if interval <= 0 {
		return true
	}
	return !s.LastSeenAt.After(now.UTC().Add(-interval))
}

// Refreshed returns a session with renewed idle expiry capped by absolute
// lifetime.
func (s LoginSession) Refreshed(now time.Time, idleTTL time.Duration) LoginSession {
	now = now.UTC()
	next := s
	next.LastSeenAt = now
	next.IdleExpiresAt = minTime(now.Add(idleTTL), s.AbsoluteExpiresAt)
	next.UpdatedAt = now
	return next
}

func (s LoginSession) validate() error {
	if s.AdminID <= 0 || s.ActiveRoleID <= 0 || strings.TrimSpace(s.TokenHash) == "" {
		return ErrInvalidLoginSession
	}
	if s.CreatedAt.IsZero() || s.LastSeenAt.IsZero() || s.IdleExpiresAt.IsZero() || s.AbsoluteExpiresAt.IsZero() || s.UpdatedAt.IsZero() {
		return ErrInvalidLoginSession
	}
	if !s.AbsoluteExpiresAt.After(s.CreatedAt) || !s.IdleExpiresAt.After(s.CreatedAt) {
		return ErrInvalidLoginSession
	}
	return nil
}

func utcPtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

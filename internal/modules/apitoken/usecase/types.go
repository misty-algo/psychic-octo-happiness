// Package usecase coordinates API token management and authentication.
package usecase

import (
	"context"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/apitoken/domain"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

const (
	maxTokenDays = 365
)

// Store persists API tokens and supports hashed-secret lookup.
type Store interface {
	ListTokens(context.Context, ListFilter) ([]domain.APIToken, int, error)
	FindTokenByID(context.Context, int64) (domain.APIToken, error)
	FindTokenByHash(context.Context, string) (domain.APIToken, error)
	CreateToken(context.Context, domain.APIToken) (domain.APIToken, error)
	UpdateToken(context.Context, domain.APIToken) (domain.APIToken, error)
	DeleteToken(context.Context, int64) error
	TouchLastUsed(context.Context, int64, time.Time) error
}

// AdminReader reads the minimal administrator state needed for token issuing.
type AdminReader interface {
	AdminSnapshot(context.Context, int64) (AdminSnapshot, error)
}

// RoleView is the access-owned role state that token policy decisions need.
type RoleView struct {
	IsSuper bool
}

// RolePolicy reads the active role's policy state for token issuance and
// visibility decisions. It stays a structured projection instead of a bare
// bool so future policy fields extend it without reshaping the bridge.
type RolePolicy interface {
	RoleView(context.Context, int64) (RoleView, error)
}

// SecretSource generates a raw API token secret.
type SecretSource func() (string, error)

// Usecase coordinates API token rules.
type Usecase struct {
	store        Store
	admins       AdminReader
	roles        RolePolicy
	now          func() time.Time
	secretSource SecretSource
}

// Option customizes the API token usecase.
type Option func(*Usecase)

// WithClock replaces the clock used for expiration and last-used timestamps.
func WithClock(now func() time.Time) Option {
	return func(u *Usecase) {
		if now != nil {
			u.now = now
		}
	}
}

// WithSecretSource replaces secure random token generation for deterministic tests.
func WithSecretSource(source SecretSource) Option {
	return func(u *Usecase) {
		if source != nil {
			u.secretSource = source
		}
	}
}

// New creates an API token usecase.
func New(store Store, admins AdminReader, roles RolePolicy, opts ...Option) *Usecase {
	u := &Usecase{
		store:        store,
		admins:       admins,
		roles:        roles,
		now:          func() time.Time { return time.Now().UTC() },
		secretSource: generateSecret,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(u)
		}
	}
	return u
}

// AdminSnapshot is the validated admin state used by token issuing rules.
type AdminSnapshot struct {
	RoleIDs []int64
	Active  bool
}

// TokenInput carries mutable API token fields.
type TokenInput struct {
	AdminID     int64
	RoleID      int64
	Name        string
	Description string
	Active      bool
	Days        int
	ExpiresAt   *time.Time
}

// UpdateTokenInput carries a full API token update.
type UpdateTokenInput struct {
	ID          int64
	Name        string
	Description string
	Active      bool
	ExpiresAt   *time.Time
}

// ListInput carries pagination for token lists.
type ListInput struct {
	Page     int
	PageSize int
	AdminID  int64
	Active   *bool
}

// ListFilter is the validated store-facing pagination window.
type ListFilter struct {
	pagination.Window
	AdminID int64
	Active  *bool
}

// TokenListOutput is a paginated API token result.
type TokenListOutput struct {
	Items    []APIToken
	Page     int
	PageSize int
	Total    int
}

// CreatedToken is returned once after creating an API token.
type CreatedToken struct {
	Token  APIToken `json:"token"`
	Secret string   `json:"secret"`
}

// TokenIdentity is the authenticated identity carried by a valid API token.
type TokenIdentity struct {
	AdminID int64
	RoleID  int64
}

// APIToken is the adapter-facing API token DTO.
type APIToken struct {
	ID          int64      `json:"id"`
	AdminID     int64      `json:"admin_id"`
	RoleID      int64      `json:"role_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Prefix      string     `json:"prefix"`
	Active      bool       `json:"active"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

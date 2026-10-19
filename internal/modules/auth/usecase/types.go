// Package usecase coordinates administrator authentication workflows.
package usecase

import (
	"context"
	"time"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	authdomain "github.com/NSObjects/echo-admin/internal/modules/auth/domain"
	identitydomain "github.com/NSObjects/echo-admin/internal/modules/identity/domain"
)

// AdminReader reads administrator credentials and profiles for authentication.
type AdminReader interface {
	FindByUsername(context.Context, string) (identitydomain.Admin, error)
	FindByID(context.Context, int64) (identitydomain.Admin, error)
	Update(context.Context, identitydomain.Admin) (identitydomain.Admin, error)
}

// AuthorizationReader reads current Administration Authorization state. The
// subject and view shapes are owned by the access module — Administration
// Authorization is its domain — so auth composes the view into current-user
// responses instead of keeping a mirrored copy.
type AuthorizationReader interface {
	CurrentAuthorization(context.Context, accessusecase.AuthorizationSubject) (accessusecase.AuthorizationView, error)
}

// LoginRecorder stores sign-in attempts without exposing audit storage details.
type LoginRecorder interface {
	RecordLogin(context.Context, LoginRecord) error
}

// LoginSessionStore persists browser login sessions without storing the raw
// cookie credential.
type LoginSessionStore interface {
	CreateLoginSession(context.Context, authdomain.LoginSession) (authdomain.LoginSession, error)
	FindLoginSessionByTokenHash(context.Context, string) (authdomain.LoginSession, bool, error)
	RefreshLoginSession(context.Context, authdomain.LoginSession) error
	UpdateLoginSessionRole(context.Context, int64, int64, time.Time) error
	RevokeLoginSession(context.Context, int64, string, time.Time) error
	RevokeOtherLoginSessions(context.Context, int64, int64, string, time.Time) error
	RevokeLoginSessions(context.Context, int64, string, time.Time) error
}

// LoginAttemptLimiter blocks repeated failed sign-in attempts across app
// instances. The key is already hashed by the usecase so stores do not need to
// persist raw usernames or client addresses.
type LoginAttemptLimiter interface {
	CheckLoginAttempt(context.Context, string, time.Time) error
	RecordLoginFailure(context.Context, string, time.Time) error
	ResetLoginAttempts(context.Context, string) error
}

// Usecase coordinates sign-in, login sessions, and current-user workflows.
type Usecase struct {
	admins        AdminReader
	authorization AuthorizationReader
	logins        LoginRecorder
	sessions      LoginSessionStore
	loginLimiter  LoginAttemptLimiter
	now           func() time.Time
}

// Option customizes the auth usecase.
type Option func(*Usecase)

// WithClock replaces the clock used for tokens and records.
func WithClock(now func() time.Time) Option {
	return func(u *Usecase) {
		if now != nil {
			u.now = now
		}
	}
}

// New creates an auth usecase with its required readers and login-session
// store.
func New(admins AdminReader, authorization AuthorizationReader, sessions LoginSessionStore, loginLimiter LoginAttemptLimiter, logins LoginRecorder, opts ...Option) *Usecase {
	u := &Usecase{
		admins:        admins,
		authorization: authorization,
		logins:        logins,
		sessions:      sessions,
		loginLimiter:  loginLimiter,
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(u)
		}
	}
	return u
}

// LoginInput carries administrator credentials from a delivery adapter.
type LoginInput struct {
	Username  string
	Password  string
	IP        string
	UserAgent string
}

// LoginOutput is returned after successful authentication.
type LoginOutput struct {
	SessionToken     string      `json:"-"`
	SessionExpiresAt time.Time   `json:"-"`
	User             CurrentUser `json:"user"`
}

// RoleSwitchInput carries the requested active role.
type RoleSwitchInput struct {
	RoleID int64
}

// ChangePasswordInput carries current-user password rotation data.
type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

// UpdateProfileInput carries current-user profile fields.
type UpdateProfileInput struct {
	DisplayName string
	Email       string
}

// RoleSwitchOutput is returned after the active role changes.
type RoleSwitchOutput struct {
	User CurrentUser `json:"user"`
}

// LoginRecord carries a safe sign-in audit event.
type LoginRecord struct {
	AdminID   int64
	Username  string
	IP        string
	UserAgent string
	Success   bool
	Reason    string
}

// LoginSessionIdentity is the authenticated browser login identity stored on
// request context by HTTP middleware.
type LoginSessionIdentity struct {
	SessionID int64
	AdminID   int64
	RoleID    int64
}

// CurrentUser is the adapter-facing current-user snapshot: administrator
// summary fields plus the access-owned authorization view. The view is
// embedded so its fields flatten into the /auth/me JSON contract; adding a
// field to accessusecase.AuthorizationView intentionally extends that
// response without any auth-side copy.
type CurrentUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	ActiveRoleID int64  `json:"active_role_id"`
	accessusecase.AuthorizationView
}

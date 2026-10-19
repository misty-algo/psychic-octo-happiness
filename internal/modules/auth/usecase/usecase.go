package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	authdomain "github.com/NSObjects/echo-admin/internal/modules/auth/domain"
	identitydomain "github.com/NSObjects/echo-admin/internal/modules/identity/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/logging"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

const (
	loginSessionTokenBytes      = 32
	loginSessionIdleTTL         = 2 * time.Hour
	loginSessionAbsoluteTTL     = 12 * time.Hour
	loginSessionRefreshInterval = 5 * time.Minute

	loginSessionRevokedLogout         = "logout"
	loginSessionRevokedLogoutOthers   = "logout_others"
	loginSessionRevokedPasswordChange = "password_changed"
	loginSessionRevokedSecurityEvent  = "security_event"
)

// Login authenticates an administrator and creates a browser login session.
func (u *Usecase) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	if err := u.ready(); err != nil {
		return LoginOutput{}, err
	}
	username := strings.ToLower(strings.TrimSpace(input.Username))
	now := u.now()
	attemptKey := loginAttemptKey(username, input.IP)
	if err := u.ensureLoginAttemptAllowed(ctx, attemptKey, now, username, input); err != nil {
		return LoginOutput{}, err
	}
	admin, err := u.admins.FindByUsername(ctx, username)
	if err != nil {
		record := loginRecordFromInput(0, username, input, false, "invalid credentials")
		return u.rejectLogin(ctx, attemptKey, now, record, apperr.New(apperr.ErrUnauthorized, "用户名或密码错误"))
	}
	if !admin.Active {
		record := loginRecordFromInput(admin.ID, username, input, false, "account disabled")
		return u.rejectLogin(ctx, attemptKey, now, record, apperr.New(apperr.ErrAccountDisabled, "账号已停用，请联系管理员"))
	}
	if compareErr := bcrypt.CompareHashAndPassword(admin.PasswordHash, []byte(input.Password)); compareErr != nil {
		record := loginRecordFromInput(admin.ID, username, input, false, "invalid credentials")
		return u.rejectLogin(ctx, attemptKey, now, record, apperr.New(apperr.ErrUnauthorized, "用户名或密码错误"))
	}

	if resetErr := u.loginLimiter.ResetLoginAttempts(ctx, attemptKey); resetErr != nil {
		return LoginOutput{}, resetErr
	}
	user, err := u.userSnapshot(ctx, admin, admin.ActiveRoleID)
	if err != nil {
		return LoginOutput{}, err
	}
	rawToken, tokenHash, err := newLoginSessionToken()
	if err != nil {
		return LoginOutput{}, err
	}
	session, err := authdomain.NewLoginSession(authdomain.LoginSessionInput{
		AdminID:           admin.ID,
		ActiveRoleID:      user.ActiveRoleID,
		TokenHash:         tokenHash,
		IP:                input.IP,
		UserAgent:         input.UserAgent,
		CreatedAt:         now,
		IdleExpiresAt:     now.Add(loginSessionIdleTTL),
		AbsoluteExpiresAt: now.Add(loginSessionAbsoluteTTL),
	})
	if err != nil {
		return LoginOutput{}, err
	}
	created, err := u.sessions.CreateLoginSession(ctx, session)
	if err != nil {
		return LoginOutput{}, err
	}
	if err := u.recordLogin(ctx, loginRecordFromInput(admin.ID, username, input, true, "login succeeded")); err != nil {
		return LoginOutput{}, err
	}
	return LoginOutput{SessionToken: rawToken, SessionExpiresAt: created.AbsoluteExpiresAt, User: user}, nil
}

// CurrentUser returns the authenticated administrator profile and active-role grants.
func (u *Usecase) CurrentUser(ctx context.Context) (CurrentUser, error) {
	if err := u.ready(); err != nil {
		return CurrentUser{}, err
	}
	admin, activeRoleID, err := u.currentAdminAndRole(ctx)
	if err != nil {
		return CurrentUser{}, err
	}
	return u.userSnapshot(ctx, admin, activeRoleID)
}

// UpdateProfile changes the current administrator's display fields only.
func (u *Usecase) UpdateProfile(ctx context.Context, input UpdateProfileInput) (CurrentUser, error) {
	if err := u.ready(); err != nil {
		return CurrentUser{}, err
	}
	admin, activeRoleID, err := u.currentAdminAndRole(ctx)
	if err != nil {
		return CurrentUser{}, err
	}
	if !admin.Active {
		return CurrentUser{}, apperr.New(apperr.ErrAccountDisabled, "账号已停用，请联系管理员")
	}
	updated, err := admin.UpdateProfile(input.DisplayName, input.Email, admin.RoleIDs, admin.ActiveRoleID, admin.Active)
	if err != nil {
		return CurrentUser{}, apperr.NewBadRequest("invalid profile")
	}
	saved, err := u.admins.Update(ctx, updated)
	if err != nil {
		return CurrentUser{}, err
	}
	return u.userSnapshot(ctx, saved, activeRoleID)
}

// SwitchRole changes the active role for the current login session.
func (u *Usecase) SwitchRole(ctx context.Context, input RoleSwitchInput) (RoleSwitchOutput, error) {
	if err := u.ready(); err != nil {
		return RoleSwitchOutput{}, err
	}
	sessionID, err := requestctx.RequireLoginSessionID(ctx)
	if err != nil {
		return RoleSwitchOutput{}, err
	}
	adminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return RoleSwitchOutput{}, err
	}
	admin, err := u.admins.FindByID(ctx, adminID)
	if err != nil {
		return RoleSwitchOutput{}, err
	}
	if !admin.Active {
		return RoleSwitchOutput{}, apperr.New(apperr.ErrAccountDisabled, "账号已停用，请联系管理员")
	}
	switched, err := admin.SwitchActiveRole(input.RoleID)
	if err != nil {
		return RoleSwitchOutput{}, apperr.NewBadRequest("active role is not assigned")
	}
	if err := u.sessions.UpdateLoginSessionRole(ctx, sessionID, switched.ActiveRoleID, u.now()); err != nil {
		return RoleSwitchOutput{}, err
	}
	user, err := u.userSnapshot(ctx, admin, input.RoleID)
	if err != nil {
		return RoleSwitchOutput{}, err
	}
	return RoleSwitchOutput{User: user}, nil
}

// ChangePassword verifies the current password, stores the new hash, and
// revokes other login sessions for the same administrator.
func (u *Usecase) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	if err := u.ready(); err != nil {
		return err
	}
	sessionID, err := requestctx.RequireLoginSessionID(ctx)
	if err != nil {
		return err
	}
	adminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	admin, err := u.admins.FindByID(ctx, adminID)
	if err != nil {
		return err
	}
	if !admin.Active {
		return apperr.New(apperr.ErrAccountDisabled, "账号已停用，请联系管理员")
	}
	if compareErr := bcrypt.CompareHashAndPassword(admin.PasswordHash, []byte(input.CurrentPassword)); compareErr != nil {
		return apperr.New(apperr.ErrUnauthorized, "当前密码不正确")
	}
	hash, err := hashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	updated, err := admin.ReplacePassword(hash)
	if err != nil {
		return apperr.NewBadRequest("invalid password")
	}
	if _, err := u.admins.Update(ctx, updated); err != nil {
		return err
	}
	return u.sessions.RevokeOtherLoginSessions(ctx, adminID, sessionID, loginSessionRevokedPasswordChange, u.now())
}

// Logout revokes the current login session.
func (u *Usecase) Logout(ctx context.Context) error {
	if err := u.ready(); err != nil {
		return err
	}
	sessionID, err := requestctx.RequireLoginSessionID(ctx)
	if err != nil {
		return err
	}
	return u.sessions.RevokeLoginSession(ctx, sessionID, loginSessionRevokedLogout, u.now())
}

// LogoutOthers revokes every other login session for the current administrator.
func (u *Usecase) LogoutOthers(ctx context.Context) error {
	if err := u.ready(); err != nil {
		return err
	}
	sessionID, err := requestctx.RequireLoginSessionID(ctx)
	if err != nil {
		return err
	}
	adminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return u.sessions.RevokeOtherLoginSessions(ctx, adminID, sessionID, loginSessionRevokedLogoutOthers, u.now())
}

// AuthenticateLoginSession validates a raw browser session token and returns
// the identity that middleware should attach to request context.
func (u *Usecase) AuthenticateLoginSession(ctx context.Context, rawToken string) (LoginSessionIdentity, error) {
	if err := u.ready(); err != nil {
		return LoginSessionIdentity{}, err
	}
	tokenHash := loginSessionTokenHash(rawToken)
	if tokenHash == "" {
		return LoginSessionIdentity{}, apperr.NewUnauthorized()
	}
	session, found, err := u.sessions.FindLoginSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return LoginSessionIdentity{}, err
	}
	if !found {
		return LoginSessionIdentity{}, apperr.NewUnauthorized()
	}
	now := u.now()
	if err := session.AvailabilityError(now); err != nil {
		return LoginSessionIdentity{}, mapSessionAvailabilityError(err)
	}
	admin, err := u.admins.FindByID(ctx, session.AdminID)
	if err != nil {
		return LoginSessionIdentity{}, apperr.NewUnauthorized()
	}
	if !admin.Active || !admin.HasRole(session.ActiveRoleID) {
		return LoginSessionIdentity{}, apperr.NewUnauthorized()
	}
	if session.NeedsRefresh(now, loginSessionRefreshInterval) {
		refreshed := session.Refreshed(now, loginSessionIdleTTL)
		if err := u.sessions.RefreshLoginSession(ctx, refreshed); err != nil {
			logging.FromContext(ctx).Warn().Err(err).Int64("session_id", session.ID).Msg("refresh login session")
		}
	}
	return LoginSessionIdentity{
		SessionID: session.ID,
		AdminID:   session.AdminID,
		RoleID:    session.ActiveRoleID,
	}, nil
}

// RevokeLoginSessions revokes all login sessions for one administrator after a
// security event such as disabling or deleting the account.
func (u *Usecase) RevokeLoginSessions(ctx context.Context, adminID int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	if adminID <= 0 {
		return apperr.NewBadRequest("invalid admin id")
	}
	return u.sessions.RevokeLoginSessions(ctx, adminID, loginSessionRevokedSecurityEvent, u.now())
}

func (u *Usecase) ready() error {
	if u == nil || u.admins == nil || u.authorization == nil || u.logins == nil || u.sessions == nil || u.loginLimiter == nil {
		return apperr.New(apperr.ErrInternalServer, "auth dependencies are not configured")
	}
	return nil
}

func (u *Usecase) currentAdminAndRole(ctx context.Context) (identitydomain.Admin, int64, error) {
	adminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return identitydomain.Admin{}, 0, err
	}
	admin, err := u.admins.FindByID(ctx, adminID)
	if err != nil {
		return identitydomain.Admin{}, 0, err
	}
	if !admin.Active {
		return identitydomain.Admin{}, 0, apperr.New(apperr.ErrAccountDisabled, "账号已停用，请联系管理员")
	}
	activeRoleID := admin.ActiveRoleID
	if roleID, roleErr := requestctx.RequireRoleID(ctx); roleErr == nil && roleID > 0 {
		activeRoleID = roleID
	}
	if !admin.HasRole(activeRoleID) {
		return identitydomain.Admin{}, 0, apperr.NewUnauthorized()
	}
	return admin, activeRoleID, nil
}

func (u *Usecase) userSnapshot(ctx context.Context, admin identitydomain.Admin, activeRoleID int64) (CurrentUser, error) {
	authorization, err := u.authorization.CurrentAuthorization(ctx, accessusecase.AuthorizationSubject{
		AdminID:      admin.ID,
		ActiveRoleID: activeRoleID,
	})
	if err != nil {
		return CurrentUser{}, err
	}
	return CurrentUser{
		ID:                admin.ID,
		Username:          admin.Username,
		DisplayName:       admin.DisplayName,
		Email:             admin.Email,
		ActiveRoleID:      authorization.ActiveRole.ID,
		AuthorizationView: authorization,
	}, nil
}

func (u *Usecase) recordLogin(ctx context.Context, record LoginRecord) error {
	return u.logins.RecordLogin(ctx, record)
}

func (u *Usecase) ensureLoginAttemptAllowed(ctx context.Context, attemptKey string, now time.Time, username string, input LoginInput) error {
	err := u.loginLimiter.CheckLoginAttempt(ctx, attemptKey, now)
	if err == nil {
		return nil
	}
	if !isTooManyLoginAttempts(err) {
		return err
	}
	record := loginRecordFromInput(0, username, input, false, "too many login attempts")
	if recordErr := u.recordLogin(ctx, record); recordErr != nil {
		return recordErr
	}
	return err
}

func (u *Usecase) rejectLogin(ctx context.Context, attemptKey string, now time.Time, record LoginRecord, loginErr error) (LoginOutput, error) {
	if recordErr := u.recordLogin(ctx, record); recordErr != nil {
		return LoginOutput{}, recordErr
	}
	if limitErr := u.loginLimiter.RecordLoginFailure(ctx, attemptKey, now); limitErr != nil {
		return LoginOutput{}, limitErr
	}
	return LoginOutput{}, loginErr
}

func hashPassword(password string) ([]byte, error) {
	if len(password) < 8 || len(password) > 72 {
		return nil, apperr.NewBadRequest("invalid password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

func newLoginSessionToken() (string, string, error) {
	token := make([]byte, loginSessionTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return "", "", fmt.Errorf("generate login session token: %w", err)
	}
	rawToken := base64.RawURLEncoding.EncodeToString(token)
	return rawToken, loginSessionTokenHash(rawToken), nil
}

func loginSessionTokenHash(rawToken string) string {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func mapSessionAvailabilityError(err error) error {
	if errors.Is(err, authdomain.ErrLoginSessionExpired) || errors.Is(err, authdomain.ErrLoginSessionRevoked) {
		return apperr.NewUnauthorized()
	}
	return err
}

func isTooManyLoginAttempts(err error) bool {
	appErr, ok := apperr.Parse(err)
	return ok && appErr.Code() == apperr.ErrTooManyAttempts
}

func loginAttemptKey(username, ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "unknown"
	}
	sum := sha256.Sum256([]byte(username + "\x00" + ip))
	return hex.EncodeToString(sum[:])
}

func loginRecordFromInput(adminID int64, username string, input LoginInput, success bool, reason string) LoginRecord {
	if username == "" {
		username = "unknown"
	}
	return LoginRecord{
		AdminID:   adminID,
		Username:  username,
		IP:        input.IP,
		UserAgent: input.UserAgent,
		Success:   success,
		Reason:    reason,
	}
}

package usecase_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	accessdomain "github.com/NSObjects/echo-admin/internal/modules/access/domain"
	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	authdomain "github.com/NSObjects/echo-admin/internal/modules/auth/domain"
	authusecase "github.com/NSObjects/echo-admin/internal/modules/auth/usecase"
	identitydomain "github.com/NSObjects/echo-admin/internal/modules/identity/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

func TestLoginCreatesSessionAndReturnsCurrentUserGrants(t *testing.T) {
	uc, recorder, store := newUsecaseWithStore(t)
	output, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	assertSuccessfulLogin(t, output, recorder, store)

	ctx := requestctx.WithUserID(context.Background(), 1)
	current, err := uc.CurrentUser(ctx)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	assertCurrentUserGrants(t, current)
}

func assertSuccessfulLogin(t *testing.T, output authusecase.LoginOutput, recorder *loginRecorder, store *authStore) {
	t.Helper()
	if output.SessionToken == "" {
		t.Fatal("Login() session token is empty, want opaque credential")
	}
	if output.SessionExpiresAt.IsZero() {
		t.Fatal("Login() session expiry is zero")
	}
	if output.User.ActiveRoleID != 1 {
		t.Fatalf("Login().User.ActiveRoleID = %d, want 1", output.User.ActiveRoleID)
	}
	if len(store.sessions) != 1 {
		t.Fatalf("created sessions = %d, want 1", len(store.sessions))
	}
	if len(recorder.records) != 1 || !recorder.records[0].Success {
		t.Fatalf("login records = %#v, want one success record", recorder.records)
	}
	if len(store.resetLoginKeys) != 1 {
		t.Fatalf("resetLoginKeys = %d, want 1 after successful login", len(store.resetLoginKeys))
	}
}

func assertCurrentUserGrants(t *testing.T, current authusecase.CurrentUser) {
	t.Helper()
	if current.Username != "admin" {
		t.Fatalf("CurrentUser().Username = %q, want admin", current.Username)
	}
	if !contains(current.Permissions, accessdomain.PermissionAdminRead) {
		t.Fatalf("permissions = %v, want %s", current.Permissions, accessdomain.PermissionAdminRead)
	}
	if len(current.Menus) == 0 {
		t.Fatal("CurrentUser().Menus is empty, want visible menus")
	}
	if current.Menus[0].Component != "./Admins" {
		t.Fatalf("CurrentUser().Menus[0].Component = %q, want ./Admins", current.Menus[0].Component)
	}
	if len(current.Menus[0].Buttons) != 2 {
		t.Fatalf("CurrentUser().Menus[0].Buttons = %d, want 2 for super admin", len(current.Menus[0].Buttons))
	}
}

func TestFailedLoginRecordsLoginLog(t *testing.T) {
	uc, recorder, store := newUsecaseWithStore(t)
	_, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "wrong-password",
	})
	if err == nil {
		t.Fatal("Login() error = nil, want invalid credentials")
	}
	if len(recorder.records) != 1 {
		t.Fatalf("login record count = %d, want 1", len(recorder.records))
	}
	if recorder.records[0].Success {
		t.Fatal("login record success = true, want false")
	}
	if len(store.recordedFailureKeys) != 1 {
		t.Fatalf("recordedFailureKeys = %d, want 1", len(store.recordedFailureKeys))
	}
}

func TestLoginRejectsLockedAttemptBeforeCredentialLookup(t *testing.T) {
	uc, recorder, store := newUsecaseWithStore(t)
	store.loginBlocked = true

	_, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "123456",
		IP:       "127.0.0.1",
	})
	if err == nil {
		t.Fatal("Login(locked) error = nil, want too many attempts")
	}
	if store.findByUsernameCalls != 0 {
		t.Fatalf("FindByUsername calls = %d, want 0 while locked", store.findByUsernameCalls)
	}
	if len(store.recordedFailureKeys) != 0 {
		t.Fatalf("recordedFailureKeys = %d, want 0 while locked", len(store.recordedFailureKeys))
	}
	if len(recorder.records) != 1 || recorder.records[0].Reason != "too many login attempts" {
		t.Fatalf("login records = %#v, want locked attempt audit", recorder.records)
	}
}

func TestSwitchRolePersistsActiveRoleAndScopesGrants(t *testing.T) {
	uc, _, store := newUsecaseWithStore(t)
	login, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	ctx := withSession(requestctx.WithUserID(context.Background(), 1), store.lastSessionID)

	output, err := uc.SwitchRole(ctx, authusecase.RoleSwitchInput{RoleID: 2})
	if err != nil {
		t.Fatalf("SwitchRole() error = %v", err)
	}
	if login.SessionToken == "" {
		t.Fatal("Login() session token is empty")
	}
	if output.User.ActiveRoleID != 2 {
		t.Fatalf("SwitchRole().User.ActiveRoleID = %d, want 2", output.User.ActiveRoleID)
	}
	if got := store.sessions[store.lastSessionID].ActiveRoleID; got != 2 {
		t.Fatalf("session active role = %d, want 2", got)
	}
	if !contains(output.User.Permissions, accessdomain.PermissionRoleRead) {
		t.Fatalf("permissions = %v, want %s", output.User.Permissions, accessdomain.PermissionRoleRead)
	}
	if contains(output.User.Permissions, accessdomain.PermissionAdminRead) {
		t.Fatalf("permissions = %v, want active role only", output.User.Permissions)
	}
	if len(output.User.Menus) != 1 {
		t.Fatalf("switched menus = %d, want 1", len(output.User.Menus))
	}
	if len(output.User.Menus[0].Buttons) != 1 || output.User.Menus[0].Buttons[0].Name != "update" {
		t.Fatalf("switched menu buttons = %#v, want only update", output.User.Menus[0].Buttons)
	}
}

func TestLogoutRevokesCurrentLoginSession(t *testing.T) {
	uc, _, store := newUsecaseWithStore(t)
	_, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	ctx := withSession(requestctx.WithUserID(context.Background(), 1), store.lastSessionID)
	if logoutErr := uc.Logout(ctx); logoutErr != nil {
		t.Fatalf("Logout() error = %v", logoutErr)
	}
	if store.sessions[store.lastSessionID].RevokedAt == nil {
		t.Fatal("session revoked_at is nil, want revoked after logout")
	}
}

func TestLogoutRejectsMissingLoginSessionContext(t *testing.T) {
	uc, _ := newUsecase(t)

	ctx := requestctx.WithUserID(context.Background(), 1)
	if err := uc.Logout(ctx); err == nil {
		t.Fatal("Logout(missing session) error = nil, want unauthorized")
	}
}

func TestChangePasswordRotatesPasswordAndRevokesOtherSessions(t *testing.T) {
	uc, _, store := newUsecaseWithStore(t)
	_, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	currentSessionID := store.lastSessionID
	if _, err := uc.Login(context.Background(), authusecase.LoginInput{Username: "admin", Password: "123456"}); err != nil {
		t.Fatalf("Login(second session) error = %v", err)
	}
	otherSessionID := store.lastSessionID
	ctx := withSession(requestctx.WithUserID(context.Background(), 1), currentSessionID)

	err = uc.ChangePassword(ctx, authusecase.ChangePasswordInput{
		CurrentPassword: "123456",
		NewPassword:     "changed123",
	})
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if store.sessions[currentSessionID].RevokedAt != nil {
		t.Fatal("current session revoked, want it to remain active")
	}
	if store.sessions[otherSessionID].RevokedAt == nil {
		t.Fatal("other session revoked_at is nil, want revoked")
	}
	if _, err := uc.Login(context.Background(), authusecase.LoginInput{Username: "admin", Password: "123456"}); err == nil {
		t.Fatal("Login(old password) error = nil, want invalid credentials")
	}
	if _, err := uc.Login(context.Background(), authusecase.LoginInput{Username: "admin", Password: "changed123"}); err != nil {
		t.Fatalf("Login(new password) error = %v", err)
	}
}

func TestChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	uc, _, store := newUsecaseWithStore(t)
	_, err := uc.Login(context.Background(), authusecase.LoginInput{
		Username: "admin",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	ctx := withSession(requestctx.WithUserID(context.Background(), 1), store.lastSessionID)

	err = uc.ChangePassword(ctx, authusecase.ChangePasswordInput{
		CurrentPassword: "wrong-password",
		NewPassword:     "changed123",
	})
	if err == nil {
		t.Fatal("ChangePassword(wrong current password) error = nil, want unauthorized")
	}
	if _, err := uc.Login(context.Background(), authusecase.LoginInput{Username: "admin", Password: "123456"}); err != nil {
		t.Fatalf("Login(original password) error = %v", err)
	}
}

func newUsecase(t *testing.T) (*authusecase.Usecase, *loginRecorder) {
	uc, recorder, _ := newUsecaseWithStore(t)
	return uc, recorder
}

func newUsecaseWithStore(t *testing.T) (*authusecase.Usecase, *loginRecorder, *authStore) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	admin, err := identitydomain.RestoreAdmin(1, "admin", "系统管理员", "admin@example.com", hash, []int64{1, 2}, 1, true, now, now)
	if err != nil {
		t.Fatalf("RestoreAdmin() error = %v", err)
	}
	store := &authStore{
		admin:         admin,
		nextSessionID: 1,
		sessions:      map[int64]authdomain.LoginSession{},
		sessionByHash: map[string]int64{},
	}
	authorization := &authorizationReader{views: authorizationViews(now)}
	recorder := &loginRecorder{}
	uc := authusecase.New(store, authorization, store, store, recorder, authusecase.WithClock(func() time.Time {
		return now
	}))
	return uc, recorder, store
}

func authorizationViews(now time.Time) map[int64]accessusecase.AuthorizationView {
	rootRole := accessusecase.Role{ID: 1, Code: accessdomain.RoleCodeSuperAdmin, Name: "超级管理员", Permissions: []string{accessdomain.PermissionAdminRead}, MenuIDs: []int64{1}, APIIDs: []int64{1}, DefaultPath: accessdomain.DefaultRolePath, Active: true, CreatedAt: now, UpdatedAt: now}
	operatorRole := accessusecase.Role{ID: 2, ParentID: 1, Code: "operator", Name: "运营", Permissions: []string{accessdomain.PermissionRoleRead}, MenuIDs: []int64{2}, APIIDs: []int64{2}, ButtonIDs: []int64{22}, DefaultPath: "/roles", Active: true, CreatedAt: now, UpdatedAt: now}
	adminMenu := accessusecase.Menu{ID: 1, Name: "管理员管理", Path: "/admins", Component: "./Admins", Permission: accessdomain.PermissionAdminRead, Active: true, Buttons: []accessusecase.Button{{ID: 11, MenuID: 1, Name: "create"}, {ID: 12, MenuID: 1, Name: "delete"}}}
	roleMenu := accessusecase.Menu{ID: 2, Name: "角色权限", Path: "/roles", Component: "./Roles", Permission: accessdomain.PermissionRoleRead, Active: true, Buttons: []accessusecase.Button{{ID: 22, MenuID: 2, Name: "update"}}}
	roles := []accessusecase.Role{rootRole, operatorRole}
	return map[int64]accessusecase.AuthorizationView{
		1: {ActiveRole: rootRole, Roles: roles, Permissions: rootRole.Permissions, Menus: []accessusecase.Menu{adminMenu}, DefaultPath: rootRole.DefaultPath},
		2: {ActiveRole: operatorRole, Roles: roles, Permissions: operatorRole.Permissions, Menus: []accessusecase.Menu{roleMenu}, DefaultPath: operatorRole.DefaultPath},
	}
}

type authorizationReader struct {
	views map[int64]accessusecase.AuthorizationView
}

func (r *authorizationReader) CurrentAuthorization(_ context.Context, subject accessusecase.AuthorizationSubject) (accessusecase.AuthorizationView, error) {
	view, ok := r.views[subject.ActiveRoleID]
	if !ok {
		return accessusecase.AuthorizationView{}, apperr.NewPermissionDenied("role", strconv.FormatInt(subject.ActiveRoleID, 10))
	}
	return view, nil
}

type authStore struct {
	admin               identitydomain.Admin
	nextSessionID       int64
	lastSessionID       int64
	sessions            map[int64]authdomain.LoginSession
	sessionByHash       map[string]int64
	loginBlocked        bool
	findByUsernameCalls int
	checkedLoginKeys    []string
	recordedFailureKeys []string
	resetLoginKeys      []string
}

func (s *authStore) FindByUsername(context.Context, string) (identitydomain.Admin, error) {
	s.findByUsernameCalls++
	return s.admin, nil
}

func (s *authStore) FindByID(context.Context, int64) (identitydomain.Admin, error) {
	return s.admin, nil
}

func (s *authStore) Update(_ context.Context, admin identitydomain.Admin) (identitydomain.Admin, error) {
	s.admin = admin
	return admin, nil
}

func (s *authStore) CreateLoginSession(_ context.Context, session authdomain.LoginSession) (authdomain.LoginSession, error) {
	session.ID = s.nextSessionID
	s.nextSessionID++
	s.lastSessionID = session.ID
	s.sessions[session.ID] = session
	s.sessionByHash[session.TokenHash] = session.ID
	return session, nil
}

func (s *authStore) FindLoginSessionByTokenHash(_ context.Context, tokenHash string) (authdomain.LoginSession, bool, error) {
	id, ok := s.sessionByHash[tokenHash]
	if !ok {
		return authdomain.LoginSession{}, false, nil
	}
	return s.sessions[id], true, nil
}

func (s *authStore) RefreshLoginSession(_ context.Context, session authdomain.LoginSession) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *authStore) UpdateLoginSessionRole(_ context.Context, sessionID, roleID int64, now time.Time) error {
	session := s.sessions[sessionID]
	session.ActiveRoleID = roleID
	session.UpdatedAt = now
	s.sessions[sessionID] = session
	return nil
}

func (s *authStore) RevokeLoginSession(_ context.Context, sessionID int64, reason string, now time.Time) error {
	session := s.sessions[sessionID]
	session.RevokedAt = &now
	session.RevokedReason = reason
	session.UpdatedAt = now
	s.sessions[sessionID] = session
	return nil
}

func (s *authStore) RevokeOtherLoginSessions(_ context.Context, adminID, keepSessionID int64, reason string, now time.Time) error {
	for id, session := range s.sessions {
		if session.AdminID != adminID || id == keepSessionID || session.RevokedAt != nil {
			continue
		}
		session.RevokedAt = &now
		session.RevokedReason = reason
		session.UpdatedAt = now
		s.sessions[id] = session
	}
	return nil
}

func (s *authStore) RevokeLoginSessions(_ context.Context, adminID int64, reason string, now time.Time) error {
	for id, session := range s.sessions {
		if session.AdminID != adminID || session.RevokedAt != nil {
			continue
		}
		session.RevokedAt = &now
		session.RevokedReason = reason
		session.UpdatedAt = now
		s.sessions[id] = session
	}
	return nil
}

func (s *authStore) CheckLoginAttempt(_ context.Context, key string, _ time.Time) error {
	s.checkedLoginKeys = append(s.checkedLoginKeys, key)
	if s.loginBlocked {
		return apperr.New(apperr.ErrTooManyAttempts, "too many login attempts")
	}
	return nil
}

func (s *authStore) RecordLoginFailure(_ context.Context, key string, _ time.Time) error {
	s.recordedFailureKeys = append(s.recordedFailureKeys, key)
	return nil
}

func (s *authStore) ResetLoginAttempts(_ context.Context, key string) error {
	s.resetLoginKeys = append(s.resetLoginKeys, key)
	return nil
}

type loginRecorder struct {
	records []authusecase.LoginRecord
}

func (r *loginRecorder) RecordLogin(_ context.Context, record authusecase.LoginRecord) error {
	r.records = append(r.records, record)
	return nil
}

func withSession(ctx context.Context, sessionID int64) context.Context {
	return requestctx.WithLoginSessionID(ctx, sessionID)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

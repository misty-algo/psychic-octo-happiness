package authhttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"

	accessdomain "github.com/NSObjects/echo-admin/internal/modules/access/domain"
	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	authdomain "github.com/NSObjects/echo-admin/internal/modules/auth/domain"
	authhttp "github.com/NSObjects/echo-admin/internal/modules/auth/http"
	authusecase "github.com/NSObjects/echo-admin/internal/modules/auth/usecase"
	identitydomain "github.com/NSObjects/echo-admin/internal/modules/identity/domain"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

func TestAuthFlowReturnsCurrentUser(t *testing.T) {
	e := newTestEcho(t)
	client := newSessionClient(e)

	login := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}
	loginBody := decodeLoginResponse(t, login)
	if loginBody.Data.User.Username != "admin" {
		t.Fatalf("login username = %q, want admin", loginBody.Data.User.Username)
	}
	assertLoginCookies(t, client)

	me := client.doJSON(t, http.MethodGet, "/api/auth/me", "")
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d: %s", me.Code, http.StatusOK, me.Body.String())
	}
	body := decodeCurrentUserResponse(t, me)
	if body.Data.Username != "admin" {
		t.Fatalf("me username = %q, want admin", body.Data.Username)
	}
	if len(body.Data.Menus) != 1 || body.Data.Menus[0].Component != "./Admins" {
		t.Fatalf("me menus = %#v, want one admins menu with component", body.Data.Menus)
	}
}

func TestLogoutRevokesCurrentLoginSession(t *testing.T) {
	e := newTestEcho(t)
	client := newSessionClient(e)
	login := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}

	logout := client.doJSON(t, http.MethodPost, "/api/auth/logout", "")
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d: %s", logout.Code, http.StatusOK, logout.Body.String())
	}
	me := client.doJSON(t, http.MethodGet, "/api/auth/me", "")
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("me status after logout = %d, want %d: %s", me.Code, http.StatusUnauthorized, me.Body.String())
	}
}

func TestLogoutOthersRevokesOtherLoginSessions(t *testing.T) {
	e := newTestEcho(t)
	current := newSessionClient(e)
	other := newSessionClient(e)
	if login := current.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`); login.Code != http.StatusOK {
		t.Fatalf("current login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}
	if login := other.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`); login.Code != http.StatusOK {
		t.Fatalf("other login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}

	logout := current.doJSON(t, http.MethodPost, "/api/auth/logout-others", "")
	if logout.Code != http.StatusOK {
		t.Fatalf("logout others status = %d, want %d: %s", logout.Code, http.StatusOK, logout.Body.String())
	}
	if me := current.doJSON(t, http.MethodGet, "/api/auth/me", ""); me.Code != http.StatusOK {
		t.Fatalf("current me status = %d, want %d: %s", me.Code, http.StatusOK, me.Body.String())
	}
	if me := other.doJSON(t, http.MethodGet, "/api/auth/me", ""); me.Code != http.StatusUnauthorized {
		t.Fatalf("other me status = %d, want %d: %s", me.Code, http.StatusUnauthorized, me.Body.String())
	}
}

func TestChangePasswordKeepsCurrentSession(t *testing.T) {
	e := newTestEcho(t)
	client := newSessionClient(e)
	login := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}

	change := client.doJSON(t, http.MethodPost, "/api/auth/password", `{"current_password":"123456","new_password":"changed123"}`)
	if change.Code != http.StatusOK {
		t.Fatalf("change password status = %d, want %d: %s", change.Code, http.StatusOK, change.Body.String())
	}
	me := client.doJSON(t, http.MethodGet, "/api/auth/me", "")
	if me.Code != http.StatusOK {
		t.Fatalf("me status after password change = %d, want %d: %s", me.Code, http.StatusOK, me.Body.String())
	}
	oldLogin := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`)
	if oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old login status = %d, want %d: %s", oldLogin.Code, http.StatusUnauthorized, oldLogin.Body.String())
	}
	newLogin := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"changed123"}`)
	if newLogin.Code != http.StatusOK {
		t.Fatalf("new login status = %d, want %d: %s", newLogin.Code, http.StatusOK, newLogin.Body.String())
	}
}

func TestUpdateProfileReturnsCurrentUser(t *testing.T) {
	e := newTestEcho(t)
	client := newSessionClient(e)
	login := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}

	update := client.doJSON(t, http.MethodPatch, "/api/auth/me", `{"display_name":"平台管理员","email":"ops@example.com"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("update profile status = %d, want %d: %s", update.Code, http.StatusOK, update.Body.String())
	}
	body := decodeCurrentUserResponse(t, update)
	if body.Data.DisplayName != "平台管理员" {
		t.Fatalf("display name = %q, want 平台管理员", body.Data.DisplayName)
	}
	if body.Data.Email != "ops@example.com" {
		t.Fatalf("email = %q, want ops@example.com", body.Data.Email)
	}
}

func newTestEcho(t *testing.T) *echo.Echo {
	t.Helper()
	store := newAuthStore(t)
	uc := authusecase.New(store, store, store, store, &loginRecorder{})
	handler := authhttp.New(uc, false)

	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	loginExemptions := []middlewares.RouteExemption{{Method: http.MethodPost, Path: "/api/auth/login"}}
	// Assemble through ApplyMiddlewares, the same interface production uses,
	// so the login-session/CSRF pairing under test cannot drift from real
	// installation.
	err := middlewares.ApplyMiddlewares(e, &middlewares.MiddlewareConfig{
		EnableRequestContext: true,
		LoginSession: &middlewares.LoginSessionConfig{
			CookieName:    authhttp.LoginSessionCookieName,
			Exemptions:    loginExemptions,
			Authenticator: sessionAuthenticator{auth: uc},
		},
		CSRF: authhttp.CSRFMiddlewareConfig(loginExemptions, false),
	})
	if err != nil {
		t.Fatalf("ApplyMiddlewares() error = %v", err)
	}
	authhttp.Register(e.Group("/api"), handler)
	return e
}

func newAuthStore(t *testing.T) *authStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	admin, err := identitydomain.RestoreAdmin(1, "admin", "系统管理员", "admin@example.com", hash, []int64{1}, 1, true, now, now)
	if err != nil {
		t.Fatalf("RestoreAdmin() error = %v", err)
	}
	role, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, nil, []int64{1}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole() error = %v", err)
	}
	menu, err := accessdomain.RestoreMenu(1, 0, "管理员管理", "/admins", "user", false, "./Admins", accessdomain.MenuMeta{}, accessdomain.PermissionAdminRead, 10, true, nil, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu() error = %v", err)
	}
	return &authStore{
		admin:         admin,
		role:          role,
		menu:          menu,
		nextSessionID: 1,
		sessions:      map[int64]authdomain.LoginSession{},
		sessionByHash: map[string]int64{},
	}
}

type authStore struct {
	admin         identitydomain.Admin
	role          accessdomain.Role
	menu          accessdomain.Menu
	nextSessionID int64
	sessions      map[int64]authdomain.LoginSession
	sessionByHash map[string]int64
}

func (s *authStore) FindByUsername(context.Context, string) (identitydomain.Admin, error) {
	return s.admin, nil
}

func (s *authStore) FindByID(context.Context, int64) (identitydomain.Admin, error) {
	return s.admin, nil
}

func (s *authStore) Update(_ context.Context, admin identitydomain.Admin) (identitydomain.Admin, error) {
	s.admin = admin
	return admin, nil
}

func (s *authStore) CurrentAuthorization(context.Context, accessusecase.AuthorizationSubject) (accessusecase.AuthorizationView, error) {
	role := accessusecase.Role{
		ID:          s.role.ID,
		Code:        s.role.Code,
		Name:        s.role.Name,
		Permissions: s.role.Permissions,
		MenuIDs:     s.role.MenuIDs,
		APIIDs:      s.role.APIIDs,
		DefaultPath: s.role.DefaultPath,
		Active:      s.role.Active,
	}
	menu := accessusecase.Menu{
		ID:         s.menu.ID,
		Name:       s.menu.Name,
		Path:       s.menu.Path,
		Component:  s.menu.Component,
		Permission: s.menu.Permission,
		Active:     s.menu.Active,
	}
	return accessusecase.AuthorizationView{
		ActiveRole:  role,
		Roles:       []accessusecase.Role{role},
		Permissions: role.Permissions,
		Menus:       []accessusecase.Menu{menu},
		DefaultPath: role.DefaultPath,
	}, nil
}

func (s *authStore) CreateLoginSession(_ context.Context, session authdomain.LoginSession) (authdomain.LoginSession, error) {
	session.ID = s.nextSessionID
	s.nextSessionID++
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

func (s *authStore) CheckLoginAttempt(context.Context, string, time.Time) error {
	return nil
}

func (s *authStore) RecordLoginFailure(context.Context, string, time.Time) error {
	return nil
}

func (s *authStore) ResetLoginAttempts(context.Context, string) error {
	return nil
}

type loginRecorder struct{}

func (r *loginRecorder) RecordLogin(context.Context, authusecase.LoginRecord) error {
	return nil
}

type sessionAuthenticator struct {
	auth *authusecase.Usecase
}

func (a sessionAuthenticator) AuthenticateLoginSession(ctx context.Context, token string) (middlewares.LoginSessionIdentity, error) {
	identity, err := a.auth.AuthenticateLoginSession(ctx, token)
	if err != nil {
		return middlewares.LoginSessionIdentity{}, err
	}
	return middlewares.LoginSessionIdentity{
		SessionID: identity.SessionID,
		UserID:    identity.AdminID,
		RoleID:    identity.RoleID,
	}, nil
}

type sessionClient struct {
	echo    *echo.Echo
	cookies map[string]*http.Cookie
}

func newSessionClient(e *echo.Echo) *sessionClient {
	return &sessionClient{echo: e, cookies: map[string]*http.Cookie{}}
}

func (c *sessionClient) doJSON(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for _, cookie := range c.cookies {
		if cookie.MaxAge < 0 {
			continue
		}
		req.AddCookie(cookie)
	}
	if csrf, ok := c.cookies[authhttp.CSRFCookieName]; ok && unsafeMethod(method) {
		req.Header.Set(echo.HeaderXCSRFToken, csrf.Value)
	}
	rec := httptest.NewRecorder()
	c.echo.ServeHTTP(rec, req)
	response := rec.Result()
	defer response.Body.Close()
	for _, cookie := range response.Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
			continue
		}
		c.cookies[cookie.Name] = cookie
	}
	return rec
}

func unsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func assertLoginCookies(t *testing.T, client *sessionClient) {
	t.Helper()
	sessionCookie, ok := client.cookies[authhttp.LoginSessionCookieName]
	if !ok {
		t.Fatalf("%s cookie missing", authhttp.LoginSessionCookieName)
	}
	if !sessionCookie.HttpOnly {
		t.Fatalf("%s HttpOnly = false, want true", authhttp.LoginSessionCookieName)
	}
	csrfCookie, ok := client.cookies[authhttp.CSRFCookieName]
	if !ok {
		t.Fatalf("%s cookie missing", authhttp.CSRFCookieName)
	}
	if csrfCookie.HttpOnly {
		t.Fatalf("%s HttpOnly = true, want false", authhttp.CSRFCookieName)
	}
}

type loginResponse struct {
	Data struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
	} `json:"data"`
}

func decodeLoginResponse(t *testing.T, rec *httptest.ResponseRecorder) loginResponse {
	t.Helper()
	var body loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login response: %v\n%s", err, rec.Body.String())
	}
	return body
}

type currentUserResponse struct {
	Data struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Menus       []struct {
			Component string `json:"component"`
		} `json:"menus"`
	} `json:"data"`
}

// TestCurrentUserJSONContractKeys 断言登录与 /auth/me 响应的原始 JSON key 契约。
// Go 的 json.Unmarshal 匹配 key 时大小写不敏感，上面的结构体解码测试无法发现
// key 大小写回归；而浏览器端 JS 属性访问是大小写敏感的，因此必须对响应里的
// 原始 key 做严格断言（AuthorizationView 缺失 json 标签时会把 key 序列化成
// ActiveRole/Menus 等大写形式并导致前端崩溃）。
func TestCurrentUserJSONContractKeys(t *testing.T) {
	e := newTestEcho(t)
	client := newSessionClient(e)

	login := client.doJSON(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"123456"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}
	assertLoginUserContractKeys(t, login.Body.Bytes())

	me := client.doJSON(t, http.MethodGet, "/api/auth/me", "")
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d: %s", me.Code, http.StatusOK, me.Body.String())
	}
	assertCurrentUserContractKeys(t, "me", me.Body.Bytes())
}

// assertLoginUserContractKeys 校验登录响应 data.user 内的当前用户 key。
func assertLoginUserContractKeys(t *testing.T, raw []byte) {
	t.Helper()
	var envelope struct {
		Data struct {
			User map[string]json.RawMessage `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("login: decode response envelope: %v\n%s", err, raw)
	}
	assertContractKeys(t, "login", envelope.Data.User)
}

// assertCurrentUserContractKeys 校验 /auth/me 响应 data 内的当前用户 key。
func assertCurrentUserContractKeys(t *testing.T, endpoint string, raw []byte) {
	t.Helper()
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("%s: decode response envelope: %v\n%s", endpoint, err, raw)
	}
	assertContractKeys(t, endpoint, envelope.Data)
}

// assertContractKeys 按 map 字面 key 匹配（大小写敏感）断言当前用户契约。
func assertContractKeys(t *testing.T, endpoint string, user map[string]json.RawMessage) {
	t.Helper()
	for _, key := range []string{"username", "display_name", "active_role", "roles", "permissions", "menus", "default_path"} {
		if _, ok := user[key]; !ok {
			t.Errorf("%s: current user key %q missing, got keys: %v", endpoint, key, mapKeys(user))
		}
	}
	for _, key := range []string{"ActiveRole", "Roles", "Permissions", "Menus", "DefaultPath"} {
		if _, ok := user[key]; ok {
			t.Errorf("%s: current user key %q must not appear, frontend reads snake_case only", endpoint, key)
		}
	}
}

func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func decodeCurrentUserResponse(t *testing.T, rec *httptest.ResponseRecorder) currentUserResponse {
	t.Helper()
	var body currentUserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode current user response: %v\n%s", err, rec.Body.String())
	}
	return body
}

package accesshttp_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	accessdomain "github.com/NSObjects/echo-admin/internal/modules/access/domain"
	accesshttp "github.com/NSObjects/echo-admin/internal/modules/access/http"
	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

func TestCreateRoleRecordsOperation(t *testing.T) {
	e, store, recorder := newAccessEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/roles", `{"code":"operator","name":"运营","permissions":["log:read"],"menu_ids":[1],"active":true}`, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create role status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createRoleCalls != 1 {
		t.Fatalf("createRoleCalls = %d, want 1", store.createRoleCalls)
	}
	if got := store.createdRole.Code; got != "operator" {
		t.Fatalf("created role code = %q, want operator", got)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].ActorID; got != 42 {
		t.Fatalf("operation actor id = %d, want 42", got)
	}
	if got := recorder.records[0].Resource; got != "role" {
		t.Fatalf("operation resource = %q, want role", got)
	}
}

func TestCopyRoleRecordsOperation(t *testing.T) {
	e, store, recorder := newAccessEcho()
	store.roles = twoRoles(t)

	rec := doJSON(t, e, http.MethodPost, "/api/roles/2/copy", `{"code":"operator_copy","name":"运营副本"}`, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("copy role status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createRoleCalls != 1 {
		t.Fatalf("createRoleCalls = %d, want 1", store.createRoleCalls)
	}
	if got := store.createdRole.Code; got != "operator_copy" {
		t.Fatalf("created role code = %q, want operator_copy", got)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].Action; got != "copy" {
		t.Fatalf("operation action = %q, want copy", got)
	}
}

func TestListPermissionsRequiresRoleReadPermission(t *testing.T) {
	e, _, _ := newAccessEcho()

	rec := doJSON(t, e, http.MethodGet, "/api/permissions", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("list permissions status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestCreateMenuAcceptsMetaAndButtons(t *testing.T) {
	e, store, recorder := newAccessEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/menus", `{"name":"菜单管理","path":"/menus","icon":"menu","component":"./Menus","meta":{"keep_alive":true},"permission":"menu:read","sort":40,"active":true,"buttons":[{"name":"create","description":"新增菜单"}]}`, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create menu status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createdMenu.Component != "./Menus" {
		t.Fatalf("created menu component = %q, want ./Menus", store.createdMenu.Component)
	}
	if !store.createdMenu.Meta.KeepAlive {
		t.Fatal("created menu KeepAlive = false, want true")
	}
	if len(store.createdMenu.Buttons) != 1 || store.createdMenu.Buttons[0].Name != "create" {
		t.Fatalf("created menu buttons = %#v, want create button", store.createdMenu.Buttons)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
}

func TestReadMenu(t *testing.T) {
	e, store, _ := newAccessEcho()
	menu, err := accessdomain.RestoreMenu(2, 0, "菜单管理", "/menus", "menu", false, "./Menus", accessdomain.MenuMeta{}, accessdomain.PermissionMenuRead, 20, true, nil, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreMenu() error = %v", err)
	}
	store.menus = []accessdomain.Menu{menu}

	rec := doJSON(t, e, http.MethodGet, "/api/menus/2", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("get menu status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestDeleteRoleRecordsOperation(t *testing.T) {
	e, store, recorder := newAccessEcho()
	store.roles = twoRoles(t)

	rec := doJSON(t, e, http.MethodDelete, "/api/roles/2", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete role status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.deletedRoleID != 2 {
		t.Fatalf("deletedRoleID = %d, want 2", store.deletedRoleID)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].Action; got != "delete" {
		t.Fatalf("operation action = %q, want delete", got)
	}
}

func TestDeleteMenuRecordsOperation(t *testing.T) {
	e, store, recorder := newAccessEcho()
	menu, err := accessdomain.RestoreMenu(2, 0, "临时菜单", "/scratch", "menu", false, "./Scratch", accessdomain.MenuMeta{}, "", 20, true, nil, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreMenu() error = %v", err)
	}
	store.menus = []accessdomain.Menu{menu}

	rec := doJSON(t, e, http.MethodDelete, "/api/menus/2", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete menu status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.deletedMenuID != 2 {
		t.Fatalf("deletedMenuID = %d, want 2", store.deletedMenuID)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].Resource; got != "menu" {
		t.Fatalf("operation resource = %q, want menu", got)
	}
}

func TestListAPIGroups(t *testing.T) {
	e, store, _ := newAccessEcho()
	firstAPI, err := accessdomain.RestoreAPI(1, "GET", "/api/a", "A", "admin", accessdomain.PermissionAdminRead, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPI(first) error = %v", err)
	}
	secondAPI, err := accessdomain.RestoreAPI(2, "GET", "/api/b", "B", "log", accessdomain.PermissionLogRead, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPI(second) error = %v", err)
	}
	store.apis = []accessdomain.API{secondAPI, firstAPI}

	rec := doJSON(t, e, http.MethodGet, "/api/apis/groups", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("list api groups status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestRegisterDoesNotExposeAPICatalogMutationRoutes(t *testing.T) {
	e, _, _ := newAccessEcho()
	registered := make(map[string]struct{})
	for _, route := range e.Router().Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}
	for _, forbidden := range []string{
		"POST /api/apis",
		"POST /api/apis/batch-delete",
		"PATCH /api/apis/:id",
		"DELETE /api/apis/:id",
	} {
		if _, ok := registered[forbidden]; ok {
			t.Fatalf("registered route %s, want deployment-owned catalog to be read-only", forbidden)
		}
	}
}

func TestReadAPI(t *testing.T) {
	e, store, _ := newAccessEcho()
	api, err := accessdomain.RestoreAPI(3, "GET", "/api/example", "示例API", "example", accessdomain.PermissionLogRead, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPI() error = %v", err)
	}
	store.apis = []accessdomain.API{api}

	rec := doJSON(t, e, http.MethodGet, "/api/apis/3", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("get api status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func newAccessEcho() (*echo.Echo, *accessStore, *operationRecorder) {
	store := &accessStore{}
	uc := accessusecase.New(store, accessAdminRoleReader{})
	recorder := &operationRecorder{}
	handler := accesshttp.New(uc, oprec.New(recorder))

	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	accesshttp.Register(e.Group("/api"), handler)
	return e, store, recorder
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	req = req.WithContext(requestctx.WithUserID(req.Context(), userID))
	req = req.WithContext(requestctx.WithRoleID(req.Context(), 1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

type accessStore struct {
	createRoleCalls int
	listMenuCalls   int
	createdRole     accessdomain.Role
	createdMenu     accessdomain.Menu
	updatedRoles    []accessdomain.Role
	roles           []accessdomain.Role
	menus           []accessdomain.Menu
	apis            []accessdomain.API
	deletedRoleID   int64
	deletedMenuID   int64
}

func (s *accessStore) FindRoleByID(_ context.Context, id int64) (accessdomain.Role, error) {
	for _, role := range s.roles {
		if role.ID == id {
			return role, nil
		}
	}
	return accessdomain.Role{}, apperr.NewNotFound("role")
}

func (s *accessStore) FindRoleByCode(context.Context, string) (accessdomain.Role, error) {
	return accessdomain.Role{}, apperr.NewNotFound("role")
}

func (s *accessStore) ListAllRoles(ctx context.Context) ([]accessdomain.Role, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(s.roles) > 0 {
		return s.roles, nil
	}
	role, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1}, []int64{1}, []int64{1}, []int64{1}, accessdomain.DefaultRolePath, true, fixedTime(), fixedTime())
	if err != nil {
		return nil, err
	}
	return []accessdomain.Role{role}, nil
}

func (s *accessStore) CreateRole(ctx context.Context, role accessdomain.Role) (accessdomain.Role, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.Role{}, err
	}
	s.createRoleCalls++
	s.createdRole = role
	return accessdomain.RestoreRole(10, role.ParentID, role.Code, role.Name, role.Permissions, role.MenuIDs, role.APIIDs, role.ButtonIDs, role.DataRoleIDs, role.DefaultPath, role.Active, fixedTime(), fixedTime())
}

func (s *accessStore) UpdateRole(ctx context.Context, role accessdomain.Role) (accessdomain.Role, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.Role{}, err
	}
	s.updatedRoles = append(s.updatedRoles, role)
	for index, existing := range s.roles {
		if existing.ID == role.ID {
			s.roles[index] = role
			return role, nil
		}
	}
	s.roles = append(s.roles, role)
	return role, nil
}

func (s *accessStore) DeleteRole(_ context.Context, id int64) error {
	s.deletedRoleID = id
	return nil
}

func (s *accessStore) FindAPIByID(ctx context.Context, id int64) (accessdomain.API, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.API{}, err
	}
	for _, api := range s.apis {
		if api.ID == id {
			return api, nil
		}
	}
	return accessdomain.RestoreAPI(id, "GET", "/api/existing", "Existing", "example", accessdomain.PermissionLogRead, fixedTime(), fixedTime())
}

func (s *accessStore) FindAPIByRoute(ctx context.Context, method, path string) (accessdomain.API, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.API{}, err
	}
	for _, api := range s.apis {
		if api.Method == method && api.Path == path {
			return api, nil
		}
	}
	return accessdomain.API{}, apperr.NewNotFound("api")
}

func (s *accessStore) ListAPIs(ctx context.Context) ([]accessdomain.API, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.apis, nil
}

func (s *accessStore) ListMenus(ctx context.Context) ([]accessdomain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.listMenuCalls++
	return s.menus, nil
}

func (s *accessStore) FindMenuByID(ctx context.Context, id int64) (accessdomain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.Menu{}, err
	}
	for _, menu := range s.menus {
		if menu.ID == id {
			return menu, nil
		}
	}
	return accessdomain.RestoreMenu(id, 0, "Existing", "/existing", "menu", false, "./Existing", accessdomain.MenuMeta{}, "log:read", 10, true, nil, fixedTime(), fixedTime())
}

func (s *accessStore) CreateMenu(ctx context.Context, menu accessdomain.Menu) (accessdomain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.Menu{}, err
	}
	s.createdMenu = menu
	return menu, nil
}

func (s *accessStore) UpdateMenu(ctx context.Context, menu accessdomain.Menu) (accessdomain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return accessdomain.Menu{}, err
	}
	return menu, nil
}

func (s *accessStore) DeleteMenu(_ context.Context, id int64) error {
	s.deletedMenuID = id
	return nil
}

type operationRecorder struct {
	records []auditusecase.OperationInput
}

func (r *operationRecorder) RecordOperation(_ context.Context, input auditusecase.OperationInput) (auditusecase.OperationLog, error) {
	r.records = append(r.records, input)
	return auditusecase.OperationLog{}, nil
}

type accessAdminRoleReader struct{}

func (accessAdminRoleReader) AdminRoleState(context.Context, int64) (accessusecase.AdminRoleState, error) {
	return accessusecase.AdminRoleState{RoleIDs: []int64{1}, ActiveRoleID: 1, Active: true}, nil
}

func (accessAdminRoleReader) RoleAssigned(context.Context, int64) (bool, error) {
	return false, nil
}

func twoRoles(t *testing.T) []accessdomain.Role {
	t.Helper()
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1}, []int64{1}, []int64{1}, []int64{1, 2}, accessdomain.DefaultRolePath, true, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	operator, err := accessdomain.RestoreRole(2, 1, "operator", "运营", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{2}, "/admins", true, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreRole(operator) error = %v", err)
	}
	return []accessdomain.Role{root, operator}
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}

package usecase_test

import (
	"context"
	"testing"
	"time"

	accessdomain "github.com/NSObjects/echo-admin/internal/modules/access/domain"
	"github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

func TestCurrentAuthorizationScopesActiveRoleGrants(t *testing.T) {
	store := authorizationStore(t)
	uc := usecase.New(store, adminRoleReaderSpy{state: usecase.AdminRoleState{
		RoleIDs:      []int64{1, 2, 3},
		ActiveRoleID: 2,
		Active:       true,
	}})

	view, err := uc.CurrentAuthorization(context.Background(), usecase.AuthorizationSubject{AdminID: 42, ActiveRoleID: 2})
	if err != nil {
		t.Fatalf("CurrentAuthorization() error = %v", err)
	}
	if view.ActiveRole.ID != 2 {
		t.Fatalf("ActiveRole.ID = %d, want 2", view.ActiveRole.ID)
	}
	if !sameStrings(view.Permissions, []string{accessdomain.PermissionRoleRead}) {
		t.Fatalf("Permissions = %v, want [%s]", view.Permissions, accessdomain.PermissionRoleRead)
	}
	if len(view.Roles) != 2 {
		t.Fatalf("Roles = %d, want 2 active assigned roles", len(view.Roles))
	}
	if len(view.Menus) != 1 || view.Menus[0].ID != 2 {
		t.Fatalf("Menus = %#v, want only role menu", view.Menus)
	}
	if len(view.Menus[0].Buttons) != 1 || view.Menus[0].Buttons[0].ID != 22 {
		t.Fatalf("Buttons = %#v, want only granted update button", view.Menus[0].Buttons)
	}
	if view.DefaultPath != "/roles" {
		t.Fatalf("DefaultPath = %q, want /roles", view.DefaultPath)
	}
}

func TestCurrentAuthorizationFailsClosedForAdministratorState(t *testing.T) {
	tests := []struct {
		name   string
		reader adminRoleReaderSpy
		code   int
	}{
		{
			name:   "missing administrator",
			reader: adminRoleReaderSpy{err: apperr.NewNotFound("admin")},
			code:   apperr.ErrUnauthorized,
		},
		{
			name: "inactive administrator",
			reader: adminRoleReaderSpy{state: usecase.AdminRoleState{
				RoleIDs: []int64{2}, ActiveRoleID: 2, Active: false,
			}},
			code: apperr.ErrAccountDisabled,
		},
		{
			name: "unassigned active role",
			reader: adminRoleReaderSpy{state: usecase.AdminRoleState{
				RoleIDs: []int64{1}, ActiveRoleID: 1, Active: true,
			}},
			code: apperr.ErrUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := usecase.New(authorizationStore(t), tt.reader)
			_, err := uc.CurrentAuthorization(context.Background(), usecase.AuthorizationSubject{AdminID: 42, ActiveRoleID: 2})
			assertAppErrorCode(t, err, tt.code)
		})
	}
}

func TestAuthorizeRouteRequiresExactActiveRoleGrant(t *testing.T) {
	uc := usecase.New(authorizationStore(t), adminRoleReaderSpy{state: usecase.AdminRoleState{
		RoleIDs: []int64{1, 2}, ActiveRoleID: 2, Active: true,
	}})
	subject := usecase.AuthorizationSubject{AdminID: 42, ActiveRoleID: 2}
	tests := []struct {
		name   string
		method string
		path   string
		code   int
	}{
		{name: "granted", method: "GET", path: "/api/roles"},
		{name: "normalized", method: " get ", path: " /api/roles "},
		{name: "ungranted", method: "GET", path: "/api/roles/:id", code: apperr.ErrPermissionDenied},
		{name: "missing", method: "GET", path: "/api/missing", code: apperr.ErrPermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uc.AuthorizeRoute(context.Background(), subject, tt.method, tt.path)
			if tt.code == 0 {
				if err != nil {
					t.Fatalf("AuthorizeRoute() error = %v, want nil", err)
				}
				return
			}
			assertAppErrorCode(t, err, tt.code)
		})
	}
}

func TestAuthorizeRouteRequiresExplicitRootGrant(t *testing.T) {
	uc := usecase.New(authorizationStore(t), adminRoleReaderSpy{state: usecase.AdminRoleState{
		RoleIDs: []int64{1}, ActiveRoleID: 1, Active: true,
	}})
	err := uc.AuthorizeRoute(context.Background(), usecase.AuthorizationSubject{AdminID: 42, ActiveRoleID: 1}, "DELETE", "/api/admins/:id")
	assertAppErrorCode(t, err, apperr.ErrPermissionDenied)
}

func TestAuthorizeRouteFailsClosedWhenActiveRoleUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]accessdomain.Role) []accessdomain.Role
	}{
		{
			name: "missing role",
			mutate: func(roles []accessdomain.Role) []accessdomain.Role {
				return roles[:1]
			},
		},
		{
			name: "inactive role",
			mutate: func(roles []accessdomain.Role) []accessdomain.Role {
				roles[1].Active = false
				return roles
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := authorizationStore(t)
			store.roles = tt.mutate(store.roles)
			uc := usecase.New(store, adminRoleReaderSpy{state: usecase.AdminRoleState{
				RoleIDs: []int64{1, 2}, ActiveRoleID: 2, Active: true,
			}})
			err := uc.AuthorizeRoute(context.Background(), usecase.AuthorizationSubject{AdminID: 42, ActiveRoleID: 2}, "GET", "/api/roles")
			assertAppErrorCode(t, err, apperr.ErrPermissionDenied)
		})
	}
}

func authorizationStore(t *testing.T) *storeSpy {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{11, 12}, []int64{1, 2}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	operator, err := accessdomain.RestoreRole(2, 1, "operator", "运营", []string{accessdomain.PermissionRoleRead}, []int64{1, 2, 3}, []int64{2}, []int64{22}, []int64{2}, "/roles", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(operator) error = %v", err)
	}
	inactive, err := accessdomain.RestoreRole(3, 1, "inactive", "停用角色", []string{accessdomain.PermissionLogRead}, []int64{3}, nil, nil, []int64{3}, "/logs", false, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(inactive) error = %v", err)
	}
	return &storeSpy{
		roles: []accessdomain.Role{root, operator, inactive},
		menus: authorizationMenus(t, now),
		apis:  authorizationAPIs(t, now),
	}
}

func authorizationMenus(t *testing.T, now time.Time) []accessdomain.Menu {
	t.Helper()
	adminCreate, err := accessdomain.RestoreMenuButton(11, 1, "create", "新增管理员", now, now)
	if err != nil {
		t.Fatalf("RestoreMenuButton(admin create) error = %v", err)
	}
	adminDelete, err := accessdomain.RestoreMenuButton(12, 1, "delete", "删除管理员", now, now)
	if err != nil {
		t.Fatalf("RestoreMenuButton(admin delete) error = %v", err)
	}
	roleCreate, err := accessdomain.RestoreMenuButton(21, 2, "create", "新增角色", now, now)
	if err != nil {
		t.Fatalf("RestoreMenuButton(role create) error = %v", err)
	}
	roleUpdate, err := accessdomain.RestoreMenuButton(22, 2, "update", "编辑角色", now, now)
	if err != nil {
		t.Fatalf("RestoreMenuButton(role update) error = %v", err)
	}
	adminMenu, err := accessdomain.RestoreMenu(1, 0, "管理员管理", "/admins", "user", false, "./Admins", accessdomain.MenuMeta{}, accessdomain.PermissionAdminRead, 10, true, []accessdomain.MenuButton{adminCreate, adminDelete}, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(admin) error = %v", err)
	}
	roleMenu, err := accessdomain.RestoreMenu(2, 0, "角色权限", "/roles", "safety", false, "./Roles", accessdomain.MenuMeta{}, accessdomain.PermissionRoleRead, 20, true, []accessdomain.MenuButton{roleCreate, roleUpdate}, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(role) error = %v", err)
	}
	inactiveMenu, err := accessdomain.RestoreMenu(3, 0, "停用菜单", "/inactive", "ban", false, "./Inactive", accessdomain.MenuMeta{}, accessdomain.PermissionRoleRead, 30, false, nil, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(inactive) error = %v", err)
	}
	return []accessdomain.Menu{adminMenu, roleMenu, inactiveMenu}
}

func authorizationAPIs(t *testing.T, now time.Time) []accessdomain.API {
	t.Helper()
	definitions := []struct {
		id          int64
		method      string
		path        string
		description string
	}{
		{id: 1, method: "GET", path: "/api/admins", description: "管理员列表"},
		{id: 2, method: "GET", path: "/api/roles", description: "角色列表"},
		{id: 3, method: "DELETE", path: "/api/admins/:id", description: "删除管理员"},
		{id: 4, method: "GET", path: "/api/roles/:id", description: "角色详情"},
	}
	apis := make([]accessdomain.API, 0, len(definitions))
	for _, definition := range definitions {
		api, err := accessdomain.RestoreAPI(definition.id, definition.method, definition.path, definition.description, "authorization", "", now, now)
		if err != nil {
			t.Fatalf("RestoreAPI(%s %s) error = %v", definition.method, definition.path, err)
		}
		apis = append(apis, api)
	}
	return apis
}

func assertAppErrorCode(t *testing.T, err error, want int) {
	t.Helper()
	appErr, ok := apperr.Parse(err)
	if !ok || appErr.Code() != want {
		t.Fatalf("error = %v, want code %d", err, want)
	}
}

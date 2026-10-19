package usecase_test

import (
	"context"
	"testing"
	"time"

	accessdomain "github.com/NSObjects/echo-admin/internal/modules/access/domain"
	"github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

func TestUpdateMenuPreservesCreatedAt(t *testing.T) {
	createdAt := time.Unix(1_700_000_000, 0).UTC()
	store := &storeSpy{menuCreatedAt: createdAt}
	uc := usecase.New(store, adminRoleReaderSpy{})

	_, err := uc.UpdateMenu(context.Background(), usecase.UpdateMenuInput{
		ID: 1,
		MenuInput: usecase.MenuInput{
			Name:       "Updated Menu",
			Path:       "/updated",
			Icon:       "menu",
			Component:  "./Updated",
			Permission: accessdomain.PermissionLogRead,
			Sort:       20,
			Active:     true,
		},
	})
	if err != nil {
		t.Fatalf("UpdateMenu() error = %v", err)
	}
	if got := store.updatedMenu.CreatedAt; !got.Equal(createdAt) {
		t.Fatalf("updated CreatedAt = %s, want %s", got, createdAt)
	}
}

func TestCreateRoleEnforcesActiveRoleGrantScope(t *testing.T) {
	uc, ctx := scopedManagerUsecase(t)

	created, err := uc.CreateRole(ctx, usecase.RoleInput{
		ParentID:    2,
		Code:        "operator",
		Name:        "运营",
		Permissions: []string{accessdomain.PermissionAdminRead},
		MenuIDs:     []int64{1},
		APIIDs:      []int64{1},
		ButtonIDs:   []int64{1},
		DefaultPath: "/admins",
		Active:      true,
	})
	if err != nil {
		t.Fatalf("CreateRole(allowed) error = %v", err)
	}
	if created.ParentID != 2 {
		t.Fatalf("created ParentID = %d, want 2", created.ParentID)
	}

	_, err = uc.CreateRole(ctx, usecase.RoleInput{
		ParentID:    2,
		Code:        "bad_operator",
		Name:        "越权运营",
		Permissions: []string{accessdomain.PermissionRoleRead},
		MenuIDs:     []int64{1},
		APIIDs:      []int64{1},
		ButtonIDs:   []int64{1},
		DefaultPath: "/roles",
		Active:      true,
	})
	if err == nil {
		t.Fatal("CreateRole(extra permission) error = nil, want permission denied")
	}

	_, err = uc.CreateRole(ctx, usecase.RoleInput{
		ParentID:    2,
		Code:        "bad_api_operator",
		Name:        "越权API运营",
		Permissions: []string{accessdomain.PermissionAdminRead},
		MenuIDs:     []int64{1},
		APIIDs:      []int64{2},
		ButtonIDs:   []int64{1},
		DefaultPath: "/admins",
		Active:      true,
	})
	if err == nil {
		t.Fatal("CreateRole(extra api) error = nil, want permission denied")
	}
}

func TestCreateRoleRejectsButtonGrantOutsideActiveRoleScope(t *testing.T) {
	uc, ctx := scopedManagerUsecase(t)

	_, err := uc.CreateRole(ctx, usecase.RoleInput{
		ParentID:    2,
		Code:        "bad_button_operator",
		Name:        "越权按钮运营",
		Permissions: []string{accessdomain.PermissionAdminRead},
		MenuIDs:     []int64{1},
		APIIDs:      []int64{1},
		ButtonIDs:   []int64{2},
		DefaultPath: "/admins",
		Active:      true,
	})
	if err == nil {
		t.Fatal("CreateRole(extra button) error = nil, want permission denied")
	}
}

func scopedManagerUsecase(t *testing.T) (*usecase.Usecase, context.Context) {
	uc, ctx, _ := scopedManagerUsecaseWithStore(t)
	return uc, ctx
}

func scopedManagerUsecaseWithStore(t *testing.T) (*usecase.Usecase, context.Context, *storeSpy) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	manager, err := accessdomain.RestoreRole(2, 1, "manager", "经理", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{2}, "/admins", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(manager) error = %v", err)
	}
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1, 2}, []int64{1, 2}, []int64{1, 2}, []int64{1, 2}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	store := &storeSpy{roles: []accessdomain.Role{root, manager}}
	uc := usecase.New(store, adminRoleReaderSpy{
		state: usecase.AdminRoleState{RoleIDs: []int64{2}, ActiveRoleID: 2},
	})
	ctx := requestctx.WithUserID(context.Background(), 42)
	return uc, requestctx.WithRoleID(ctx, 2), store
}

func TestVisibleRoleIDsUsesDataAuthorityInsideDelegationScope(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1}, []int64{1}, []int64{1}, []int64{1, 2, 3, 4}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	manager, err := accessdomain.RestoreRole(2, 1, "manager", "经理", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{3, 4}, "/admins", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(manager) error = %v", err)
	}
	operator, err := accessdomain.RestoreRole(3, 2, "operator", "运营", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{3}, "/admins", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(operator) error = %v", err)
	}
	sibling, err := accessdomain.RestoreRole(4, 1, "sibling", "同级", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{4}, "/admins", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(sibling) error = %v", err)
	}
	uc := usecase.New(&storeSpy{roles: []accessdomain.Role{root, manager, operator, sibling}}, adminRoleReaderSpy{
		state: usecase.AdminRoleState{RoleIDs: []int64{2}, ActiveRoleID: 2},
	})
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 42), 2)

	ids, err := uc.VisibleRoleIDs(ctx)
	if err != nil {
		t.Fatalf("VisibleRoleIDs() error = %v", err)
	}
	if !sameInt64s(ids, []int64{3}) {
		t.Fatalf("VisibleRoleIDs() = %v, want [3]", ids)
	}
}

func TestCopyRoleCopiesSourceGrants(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1, 2}, []int64{1, 2}, []int64{1, 2}, []int64{1, 2}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	source, err := accessdomain.RestoreRole(2, 1, "operator", "运营", []string{accessdomain.PermissionAdminRead, accessdomain.PermissionLogRead}, []int64{1, 2}, []int64{1, 2}, []int64{1, 2}, []int64{2}, "/admins", false, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(source) error = %v", err)
	}
	store := &storeSpy{roles: []accessdomain.Role{root, source}}
	uc := usecase.New(store, adminRoleReaderSpy{})

	copied, err := uc.CopyRole(superAdminContext(), usecase.CopyRoleInput{
		SourceID: 2,
		Code:     "operator_copy",
		Name:     "运营副本",
	})
	if err != nil {
		t.Fatalf("CopyRole() error = %v", err)
	}
	if copied.Code != "operator_copy" {
		t.Fatalf("copied Code = %q, want operator_copy", copied.Code)
	}
	assertCopiedRole(t, store.createdRole, source)
}

func assertCopiedRole(t *testing.T, gotRole, wantRole accessdomain.Role) {
	t.Helper()
	if gotRole.ParentID != wantRole.ParentID {
		t.Fatalf("created ParentID = %d, want %d", gotRole.ParentID, wantRole.ParentID)
	}
	if gotRole.DefaultPath != wantRole.DefaultPath {
		t.Fatalf("created DefaultPath = %q, want %q", gotRole.DefaultPath, wantRole.DefaultPath)
	}
	if gotRole.Active != wantRole.Active {
		t.Fatalf("created Active = %v, want %v", gotRole.Active, wantRole.Active)
	}
	if got, want := gotRole.Permissions, wantRole.Permissions; !sameStrings(got, want) {
		t.Fatalf("created Permissions = %v, want %v", got, want)
	}
	if got, want := gotRole.MenuIDs, wantRole.MenuIDs; !sameInt64s(got, want) {
		t.Fatalf("created MenuIDs = %v, want %v", got, want)
	}
	if got, want := gotRole.APIIDs, wantRole.APIIDs; !sameInt64s(got, want) {
		t.Fatalf("created APIIDs = %v, want %v", got, want)
	}
	if got, want := gotRole.ButtonIDs, wantRole.ButtonIDs; !sameInt64s(got, want) {
		t.Fatalf("created ButtonIDs = %v, want %v", got, want)
	}
	if got, want := gotRole.DataRoleIDs, wantRole.DataRoleIDs; !sameInt64s(got, want) {
		t.Fatalf("created DataRoleIDs = %v, want %v", got, want)
	}
}

func TestDeleteRoleRejectsAssignedRole(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1}, []int64{1}, []int64{1}, []int64{1, 2}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	operator, err := accessdomain.RestoreRole(2, 1, "operator", "运营", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{2}, "/admins", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(operator) error = %v", err)
	}
	store := &storeSpy{roles: []accessdomain.Role{root, operator}}
	uc := usecase.New(store, adminRoleReaderSpy{assignedRoles: map[int64]bool{2: true}})

	err = uc.DeleteRole(superAdminContext(), 2)
	if err == nil {
		t.Fatal("DeleteRole(assigned role) error = nil, want conflict")
	}
	if store.deletedRoleID != 0 {
		t.Fatalf("deletedRoleID = %d, want 0", store.deletedRoleID)
	}
}

func TestDeleteRoleDeletesLeafRole(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	root, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{1}, []int64{1}, []int64{1}, []int64{1, 2}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(root) error = %v", err)
	}
	operator, err := accessdomain.RestoreRole(2, 1, "operator", "运营", []string{accessdomain.PermissionAdminRead}, []int64{1}, []int64{1}, []int64{1}, []int64{2}, "/admins", true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(operator) error = %v", err)
	}
	store := &storeSpy{roles: []accessdomain.Role{root, operator}}
	uc := usecase.New(store, adminRoleReaderSpy{})

	if err := uc.DeleteRole(superAdminContext(), 2); err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
	if store.deletedRoleID != 2 {
		t.Fatalf("deletedRoleID = %d, want 2", store.deletedRoleID)
	}
}

func TestDeleteMenuRejectsAssignedMenu(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	role, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), []int64{2}, []int64{1}, []int64{1}, []int64{1}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(role) error = %v", err)
	}
	menu, err := accessdomain.RestoreMenu(2, 0, "管理员管理", "/admins", "user", false, "./Admins", accessdomain.MenuMeta{}, accessdomain.PermissionAdminRead, 20, true, nil, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(menu) error = %v", err)
	}
	store := &storeSpy{roles: []accessdomain.Role{role}, menus: []accessdomain.Menu{menu}}
	uc := usecase.New(store, adminRoleReaderSpy{})

	err = uc.DeleteMenu(context.Background(), 2)
	if err == nil {
		t.Fatal("DeleteMenu(assigned menu) error = nil, want conflict")
	}
	if store.deletedMenuID != 0 {
		t.Fatalf("deletedMenuID = %d, want 0", store.deletedMenuID)
	}
}

func TestDeleteMenuRejectsAssignedMenuButton(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	role, err := accessdomain.RestoreRole(1, 0, accessdomain.RoleCodeSuperAdmin, "超级管理员", accessdomain.PermissionCatalogTokens(), nil, nil, []int64{7}, []int64{1}, accessdomain.DefaultRolePath, true, now, now)
	if err != nil {
		t.Fatalf("RestoreRole(role) error = %v", err)
	}
	button, err := accessdomain.RestoreMenuButton(7, 2, "delete", "删除菜单", now, now)
	if err != nil {
		t.Fatalf("RestoreMenuButton() error = %v", err)
	}
	menu, err := accessdomain.RestoreMenu(2, 0, "菜单管理", "/menus", "menu", false, "./Menus", accessdomain.MenuMeta{}, accessdomain.PermissionMenuRead, 20, true, []accessdomain.MenuButton{button}, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(menu) error = %v", err)
	}
	store := &storeSpy{roles: []accessdomain.Role{role}, menus: []accessdomain.Menu{menu}}
	uc := usecase.New(store, adminRoleReaderSpy{})

	err = uc.DeleteMenu(context.Background(), 2)
	if err == nil {
		t.Fatal("DeleteMenu(assigned menu button) error = nil, want conflict")
	}
	if store.deletedMenuID != 0 {
		t.Fatalf("deletedMenuID = %d, want 0", store.deletedMenuID)
	}
}

func TestDeleteMenuDeletesUnassignedLeafMenu(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	menu, err := accessdomain.RestoreMenu(2, 0, "临时菜单", "/scratch", "menu", false, "./Scratch", accessdomain.MenuMeta{}, "", 20, true, nil, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(menu) error = %v", err)
	}
	store := &storeSpy{menus: []accessdomain.Menu{menu}}
	uc := usecase.New(store, adminRoleReaderSpy{})

	if err := uc.DeleteMenu(context.Background(), 2); err != nil {
		t.Fatalf("DeleteMenu() error = %v", err)
	}
	if store.deletedMenuID != 2 {
		t.Fatalf("deletedMenuID = %d, want 2", store.deletedMenuID)
	}
}

func TestAPIGroupsReturnsSortedUniqueGroups(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	adminAPI, err := accessdomain.RestoreAPI(1, "GET", "/api/admins", "管理员", "admin", accessdomain.PermissionAdminRead, now, now)
	if err != nil {
		t.Fatalf("RestoreAPI(admin) error = %v", err)
	}
	logAPI, err := accessdomain.RestoreAPI(2, "GET", "/api/logs", "日志", "log", accessdomain.PermissionLogRead, now, now)
	if err != nil {
		t.Fatalf("RestoreAPI(log) error = %v", err)
	}
	anotherAdminAPI, err := accessdomain.RestoreAPI(3, "POST", "/api/admins", "创建管理员", "admin", accessdomain.PermissionAdminCreate, now, now)
	if err != nil {
		t.Fatalf("RestoreAPI(another admin) error = %v", err)
	}
	store := &storeSpy{apis: []accessdomain.API{logAPI, adminAPI, anotherAdminAPI}}
	uc := usecase.New(store, adminRoleReaderSpy{})

	groups, err := uc.APIGroups(context.Background())
	if err != nil {
		t.Fatalf("APIGroups() error = %v", err)
	}
	if !sameStrings(groups, []string{"admin", "log"}) {
		t.Fatalf("APIGroups() = %v, want [admin log]", groups)
	}
}

func superAdminContext() context.Context {
	ctx := requestctx.WithUserID(context.Background(), 42)
	return requestctx.WithRoleID(ctx, 1)
}

type storeSpy struct {
	menuCreatedAt time.Time
	updatedMenu   accessdomain.Menu
	createdRole   accessdomain.Role
	updatedRoles  []accessdomain.Role
	roles         []accessdomain.Role
	menus         []accessdomain.Menu
	apis          []accessdomain.API
	deletedRoleID int64
	deletedMenuID int64
}

func (s *storeSpy) FindRoleByID(_ context.Context, id int64) (accessdomain.Role, error) {
	for _, role := range s.roles {
		if role.ID == id {
			return role, nil
		}
	}
	return accessdomain.Role{}, apperr.NewNotFound("role")
}

func (s *storeSpy) FindRoleByCode(context.Context, string) (accessdomain.Role, error) {
	return accessdomain.Role{}, apperr.NewNotFound("role")
}

func (s *storeSpy) ListAllRoles(context.Context) ([]accessdomain.Role, error) {
	return s.roles, nil
}

func (s *storeSpy) CreateRole(_ context.Context, role accessdomain.Role) (accessdomain.Role, error) {
	s.createdRole = role
	return accessdomain.RestoreRole(10, role.ParentID, role.Code, role.Name, role.Permissions, role.MenuIDs, role.APIIDs, role.ButtonIDs, role.DataRoleIDs, role.DefaultPath, role.Active, time.Now(), time.Now())
}

func (s *storeSpy) UpdateRole(_ context.Context, role accessdomain.Role) (accessdomain.Role, error) {
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

func (s *storeSpy) DeleteRole(_ context.Context, id int64) error {
	s.deletedRoleID = id
	return nil
}

func (s *storeSpy) FindAPIByID(_ context.Context, id int64) (accessdomain.API, error) {
	for _, api := range s.apis {
		if api.ID == id {
			return api, nil
		}
	}
	return accessdomain.RestoreAPI(id, "GET", "/api/existing", "Existing", "example", accessdomain.PermissionLogRead, time.Now(), time.Now())
}

func (s *storeSpy) FindAPIByRoute(_ context.Context, method, path string) (accessdomain.API, error) {
	for _, api := range s.apis {
		if api.Method == method && api.Path == path {
			return api, nil
		}
	}
	return accessdomain.API{}, apperr.NewNotFound("api")
}

func (s *storeSpy) ListAPIs(context.Context) ([]accessdomain.API, error) {
	return s.apis, nil
}

func (s *storeSpy) FindMenuByID(_ context.Context, id int64) (accessdomain.Menu, error) {
	for _, menu := range s.menus {
		if menu.ID == id {
			return menu, nil
		}
	}
	return accessdomain.RestoreMenu(1, 0, "Existing", "/existing", "menu", false, "./Existing", accessdomain.MenuMeta{}, accessdomain.PermissionLogRead, 10, true, nil, s.menuCreatedAt, s.menuCreatedAt)
}

func (s *storeSpy) ListMenus(context.Context) ([]accessdomain.Menu, error) {
	return s.menus, nil
}

func (s *storeSpy) CreateMenu(context.Context, accessdomain.Menu) (accessdomain.Menu, error) {
	return accessdomain.Menu{}, nil
}

func (s *storeSpy) UpdateMenu(_ context.Context, menu accessdomain.Menu) (accessdomain.Menu, error) {
	s.updatedMenu = menu
	return menu, nil
}

func (s *storeSpy) DeleteMenu(_ context.Context, id int64) error {
	s.deletedMenuID = id
	return nil
}

type adminRoleReaderSpy struct {
	state         usecase.AdminRoleState
	assignedRoles map[int64]bool
	err           error
}

func (s adminRoleReaderSpy) AdminRoleState(context.Context, int64) (usecase.AdminRoleState, error) {
	if s.err != nil {
		return usecase.AdminRoleState{}, s.err
	}
	if len(s.state.RoleIDs) > 0 {
		return s.state, nil
	}
	return usecase.AdminRoleState{RoleIDs: []int64{1}, ActiveRoleID: 1, Active: true}, nil
}

func (s adminRoleReaderSpy) RoleAssigned(_ context.Context, roleID int64) (bool, error) {
	return s.assignedRoles[roleID], nil
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func sameInt64s(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func findRoleForTest(roles []accessdomain.Role, roleID int64) (accessdomain.Role, bool) {
	for _, role := range roles {
		if role.ID == roleID {
			return role, true
		}
	}
	return accessdomain.Role{}, false
}

func containsIDForTest(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

type menuTreeStore struct {
	storeSpy
	nextMenuID int64
}

func (s *menuTreeStore) CreateMenu(_ context.Context, menu accessdomain.Menu) (accessdomain.Menu, error) {
	s.nextMenuID++
	menu.ID = s.nextMenuID
	s.menus = append(s.menus, menu)
	return menu, nil
}

func (s *menuTreeStore) UpdateMenu(_ context.Context, menu accessdomain.Menu) (accessdomain.Menu, error) {
	for index, existing := range s.menus {
		if existing.ID == menu.ID {
			s.menus[index] = menu
			return menu, nil
		}
	}
	s.menus = append(s.menus, menu)
	return menu, nil
}

func treeMenu(t *testing.T, id, parentID int64, path string) accessdomain.Menu {
	t.Helper()
	now := time.Unix(1_700_000_000, 0).UTC()
	menu, err := accessdomain.RestoreMenu(id, parentID, "菜单"+path, path, "menu", false, "./M"+path, accessdomain.MenuMeta{}, accessdomain.PermissionLogRead, 10, true, nil, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu(%s) error = %v", path, err)
	}
	return menu
}

func findTreeMenu(t *testing.T, menus []accessdomain.Menu, path string) accessdomain.Menu {
	t.Helper()
	for _, menu := range menus {
		if menu.Path == path {
			return menu
		}
	}
	t.Fatalf("menu %s not found in %#v", path, menus)
	return accessdomain.Menu{}
}

func menuNodeTreePath(nodes []usecase.MenuTreeInput) string {
	out := ""
	for index, node := range nodes {
		if index > 0 {
			out += ","
		}
		out += node.Path
		if len(node.Children) > 0 {
			out += ">" + menuNodeTreePath(node.Children)
		}
	}
	return out
}

func TestExportMenuTreeRequiresAllSelectedMenus(t *testing.T) {
	store := &menuTreeStore{nextMenuID: 100}
	store.menus = []accessdomain.Menu{treeMenu(t, 1, 0, "/a")}
	uc := usecase.New(store, adminRoleReaderSpy{})

	_, err := uc.ExportMenuTree(context.Background(), []int64{1, 9})
	appErr, ok := apperr.Parse(err)
	if !ok || appErr.Code() != apperr.ErrNotFound {
		t.Fatalf("ExportMenuTree(missing id) error = %v, want not found", err)
	}
}

func TestExportMenuTreeSkipsUnselectedAncestors(t *testing.T) {
	store := &menuTreeStore{nextMenuID: 100}
	store.menus = []accessdomain.Menu{
		treeMenu(t, 1, 0, "/a"),
		treeMenu(t, 2, 1, "/a/b"),
		treeMenu(t, 3, 2, "/a/b/c"),
	}
	uc := usecase.New(store, adminRoleReaderSpy{})

	tests := []struct {
		name     string
		ids      []int64
		wantTree string
	}{
		{name: "descendant without ancestors", ids: []int64{3}, wantTree: "/a/b/c"},
		{name: "selected ancestor skips unselected middle", ids: []int64{1, 3}, wantTree: "/a>/a/b/c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := uc.ExportMenuTree(context.Background(), tt.ids)
			if err != nil {
				t.Fatalf("ExportMenuTree() error = %v", err)
			}
			if got := menuNodeTreePath(nodes); got != tt.wantTree {
				t.Fatalf("exported tree = %s, want %s", got, tt.wantTree)
			}
		})
	}
}

func TestImportMenuTreeUpsertsByPathAndRebindsChildren(t *testing.T) {
	store := &menuTreeStore{nextMenuID: 100}
	store.menus = []accessdomain.Menu{treeMenu(t, 1, 0, "/old")}
	uc := usecase.New(store, adminRoleReaderSpy{})

	tree := usecase.MenuTreeInput{
		MenuInput: usecase.MenuInput{
			Name: "新父", Path: "/new", Icon: "menu", Component: "./New",
			Permission: accessdomain.PermissionLogRead, Sort: 10, Active: true,
		},
		Children: []usecase.MenuTreeInput{
			{
				MenuInput: usecase.MenuInput{
					Name: "旧子", Path: "/old", Icon: "menu", Component: "./Old",
					Permission: accessdomain.PermissionLogRead, Sort: 20, Active: true,
				},
			},
		},
	}
	if err := uc.ImportMenuTree(context.Background(), []usecase.MenuTreeInput{tree}); err != nil {
		t.Fatalf("ImportMenuTree() error = %v", err)
	}

	if got := len(store.menus); got != 2 {
		t.Fatalf("menu count after import = %d, want 2", got)
	}
	newMenu := findTreeMenu(t, store.menus, "/new")
	oldMenu := findTreeMenu(t, store.menus, "/old")
	if oldMenu.ParentID != newMenu.ID {
		t.Fatalf("imported /old parent id = %d, want freshly saved parent %d", oldMenu.ParentID, newMenu.ID)
	}
	if newMenu.ParentID != 0 {
		t.Fatalf("imported root /new parent id = %d, want top level", newMenu.ParentID)
	}

	if err := uc.ImportMenuTree(context.Background(), []usecase.MenuTreeInput{tree}); err != nil {
		t.Fatalf("ImportMenuTree(second pass) error = %v", err)
	}
	if got := len(store.menus); got != 2 {
		t.Fatalf("menu count after reimport = %d, want 2 (path upsert, no duplicates)", got)
	}
}

func TestImportMenuTreeRejectsInvalidPermission(t *testing.T) {
	store := &menuTreeStore{nextMenuID: 100}
	uc := usecase.New(store, adminRoleReaderSpy{})

	err := uc.ImportMenuTree(context.Background(), []usecase.MenuTreeInput{
		{MenuInput: usecase.MenuInput{Name: "坏权限", Path: "/bad", Component: "./Bad", Permission: "bogus", Sort: 10, Active: true}},
	})
	appErr, ok := apperr.Parse(err)
	if !ok || appErr.Code() != apperr.ErrBadRequest {
		t.Fatalf("ImportMenuTree(invalid permission) error = %v, want bad request", err)
	}
	if got := len(store.menus); got != 0 {
		t.Fatalf("menu count after rejected import = %d, want 0", got)
	}
}

package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRestoreRoleRejectsInvalidPermissionToken(t *testing.T) {
	_, err := RestoreRole(1, 0, "operator", "Operator", []string{"admin"}, []int64{1}, nil, nil, nil, DefaultRolePath, true, time.Now(), time.Now())
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("RestoreRole() error = %v, want %v", err, ErrInvalidPermission)
	}
}

func TestRoleHasAPIReportsExplicitGrant(t *testing.T) {
	role, err := RestoreRole(2, 1, "operator", "运营", []string{PermissionRoleRead}, []int64{2}, []int64{1, 3}, nil, nil, DefaultRolePath, true, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("RestoreRole() error = %v", err)
	}
	if !role.HasAPI(1) || !role.HasAPI(3) {
		t.Fatalf("HasAPI(granted) = false for api ids %v, want true", role.APIIDs)
	}
	if role.HasAPI(2) {
		t.Fatalf("HasAPI(ungranted) = true for api ids %v, want false", role.APIIDs)
	}
}

func TestRestoreMenuRejectsInvalidPermissionToken(t *testing.T) {
	_, err := RestoreMenu(1, 0, "Admins", "/admins", "user", false, "./Admins", MenuMeta{}, "admin", 10, true, nil, time.Now(), time.Now())
	if !errors.Is(err, ErrInvalidPermission) {
		t.Fatalf("RestoreMenu() error = %v, want %v", err, ErrInvalidPermission)
	}
}

func TestRestoreMenuNormalizesMetaAndButtons(t *testing.T) {
	now := time.Now()
	menu, err := RestoreMenu(1, 0, "菜单管理", "/menus", "menu", true, " ./Menus ", MenuMeta{
		ActiveName:     " menus ",
		KeepAlive:      true,
		DefaultMenu:    true,
		CloseTab:       true,
		TransitionType: " fade ",
	}, PermissionMenuRead, 10, true, []MenuButton{
		{Name: "create", Description: "新增"},
		{Name: "create", Description: "重复名称会去重"},
	}, now, now)
	if err != nil {
		t.Fatalf("RestoreMenu() error = %v", err)
	}
	if menu.Component != "./Menus" {
		t.Fatalf("Component = %q, want ./Menus", menu.Component)
	}
	if menu.Meta.ActiveName != "menus" || menu.Meta.TransitionType != "fade" {
		t.Fatalf("Meta = %#v, want trimmed active name and transition", menu.Meta)
	}
	if len(menu.Buttons) != 1 || menu.Buttons[0].Name != "create" {
		t.Fatalf("Buttons = %#v, want one create button", menu.Buttons)
	}
}

func TestPermissionCatalogTokensMatchesCatalog(t *testing.T) {
	catalog := PermissionCatalog()
	tokens := PermissionCatalogTokens()
	if len(tokens) != len(catalog) {
		t.Fatalf("PermissionCatalogTokens() length = %d, want %d", len(tokens), len(catalog))
	}
	for index, definition := range catalog {
		if tokens[index] != definition.Token {
			t.Fatalf("PermissionCatalogTokens()[%d] = %q, want %q", index, tokens[index], definition.Token)
		}
	}
}

func TestPermissionCatalogCoversFoundationPermissions(t *testing.T) {
	catalog := map[string]struct{}{}
	for _, permission := range PermissionCatalog() {
		if permission.Token == "" || permission.Resource == "" || permission.Action == "" || permission.Name == "" {
			t.Fatalf("permission catalog contains incomplete definition: %#v", permission)
		}
		if _, ok := catalog[permission.Token]; ok {
			t.Fatalf("permission catalog contains duplicate token %q", permission.Token)
		}
		catalog[permission.Token] = struct{}{}
	}

	for _, token := range requiredFoundationPermissions {
		if _, ok := catalog[token]; !ok {
			t.Fatalf("permission catalog missing %q", token)
		}
	}
}

var requiredFoundationPermissions = []string{
	PermissionAdminRead,
	PermissionAdminCreate,
	PermissionAdminUpdate,
	PermissionAdminDelete,
	PermissionRoleRead,
	PermissionRoleCreate,
	PermissionRoleUpdate,
	PermissionRoleDelete,
	PermissionMenuRead,
	PermissionMenuCreate,
	PermissionMenuUpdate,
	PermissionMenuDelete,
	PermissionAPIRead,
	PermissionAPIGrant,
	PermissionAPITokenRead,
	PermissionAPITokenCreate,
	PermissionAPITokenUpdate,
	PermissionAPITokenDelete,
	PermissionConfigRead,
	PermissionConfigUpdate,
	PermissionConfigDelete,
	PermissionParamRead,
	PermissionParamCreate,
	PermissionParamUpdate,
	PermissionParamDelete,
	PermissionVersionRead,
	PermissionVersionCreate,
	PermissionVersionUpdate,
	PermissionVersionDelete,
	PermissionDictRead,
	PermissionDictCreate,
	PermissionDictUpdate,
	PermissionDictDelete,
	PermissionFileRead,
	PermissionFileUpload,
	PermissionFileUpdate,
	PermissionFileDelete,
	PermissionFileCategoryCreate,
	PermissionFileCategoryUpdate,
	PermissionFileCategoryDelete,
	PermissionLogRead,
	PermissionLogDelete,
	PermissionLogResolve,
}

package domain

import (
	"testing"
	"time"
)

func TestMenuCatalogIsWellFormed(t *testing.T) {
	catalog := MenuCatalog()
	if len(catalog) == 0 {
		t.Fatal("MenuCatalog() is empty, want the installation menu baseline")
	}

	paths := make(map[string]bool, len(catalog))
	for _, seed := range catalog {
		if seed.Path == "" {
			t.Fatalf("catalog entry %q has empty Path", seed.Name)
		}
		if paths[seed.Path] {
			t.Fatalf("Path %q appears twice in MenuCatalog()", seed.Path)
		}
		paths[seed.Path] = true
		if seed.Name == "" {
			t.Fatalf("catalog entry %q has empty Name", seed.Path)
		}
		if seed.Component == "" {
			t.Fatalf("catalog entry %q has empty Component", seed.Path)
		}
		if seed.ParentPath == seed.Path {
			t.Fatalf("catalog entry %q parents itself", seed.Path)
		}
	}

	for _, seed := range catalog {
		if seed.ParentPath != "" && !paths[seed.ParentPath] {
			t.Fatalf("catalog entry %q references missing parent %q", seed.Path, seed.ParentPath)
		}
	}
}

func TestMenuCatalogReturnsACopy(t *testing.T) {
	first := MenuCatalog()
	first[0].Name = "mutated"
	second := MenuCatalog()
	if second[0].Name == "mutated" {
		t.Fatal("MenuCatalog() exposes mutable catalog state, want a defensive copy")
	}
}

func TestNewRootRoleAppliesBaselineShape(t *testing.T) {
	now := time.Now().UTC()

	role, err := NewRootRole([]int64{1}, []int64{2}, []int64{3}, []int64{4}, now)
	if err != nil {
		t.Fatalf("NewRootRole() error = %v, want nil", err)
	}
	if role.Code != RoleCodeSuperAdmin {
		t.Fatalf("Code = %q, want %q", role.Code, RoleCodeSuperAdmin)
	}
	if role.Name != "超级管理员" {
		t.Fatalf("Name = %q, want 超级管理员", role.Name)
	}
	if role.ParentID != 0 {
		t.Fatalf("ParentID = %d, want 0", role.ParentID)
	}
	if !role.Active {
		t.Fatal("Active = false, want true")
	}
	if role.DefaultPath != DefaultRolePath {
		t.Fatalf("DefaultPath = %q, want %q", role.DefaultPath, DefaultRolePath)
	}
	if !role.CreatedAt.Equal(now) || !role.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %v/%v, want %v", role.CreatedAt, role.UpdatedAt, now)
	}
}

func TestNewRootRoleCarriesCompleteGrants(t *testing.T) {
	menuIDs, apiIDs, buttonIDs, roleIDs := []int64{1, 2}, []int64{3}, []int64{4, 5}, []int64{6}

	role, err := NewRootRole(menuIDs, apiIDs, buttonIDs, roleIDs, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewRootRole() error = %v, want nil", err)
	}
	if len(role.Permissions) != len(PermissionCatalogTokens()) {
		t.Fatalf("Permissions length = %d, want %d from the permission catalog", len(role.Permissions), len(PermissionCatalogTokens()))
	}
	grants := []struct {
		name string
		got  []int64
		want []int64
	}{
		{name: "MenuIDs", got: role.MenuIDs, want: menuIDs},
		{name: "APIIDs", got: role.APIIDs, want: apiIDs},
		{name: "ButtonIDs", got: role.ButtonIDs, want: buttonIDs},
		{name: "DataRoleIDs", got: role.DataRoleIDs, want: roleIDs},
	}
	for _, grant := range grants {
		if len(grant.got) != len(grant.want) {
			t.Fatalf("%s = %v, want %v", grant.name, grant.got, grant.want)
		}
		for i := range grant.want {
			if grant.got[i] != grant.want[i] {
				t.Fatalf("%s = %v, want %v", grant.name, grant.got, grant.want)
			}
		}
	}
}

func TestMenuCatalogButtonsAreUniqueAndDescribed(t *testing.T) {
	for _, seed := range MenuCatalog() {
		seenButtons := make(map[string]struct{}, len(seed.Buttons))
		for _, button := range seed.Buttons {
			if button.Name == "" {
				t.Fatalf("catalog entry %q has empty button name", seed.Path)
			}
			if button.Description == "" {
				t.Fatalf("catalog entry %q button %q description is empty", seed.Path, button.Name)
			}
			if _, ok := seenButtons[button.Name]; ok {
				t.Fatalf("catalog entry %q button %q is duplicated", seed.Path, button.Name)
			}
			seenButtons[button.Name] = struct{}{}
		}
	}
}

func TestMenuCatalogParentsAreDeclaredBeforeChildren(t *testing.T) {
	seen := make(map[string]struct{})
	for _, seed := range MenuCatalog() {
		if seed.ParentPath != "" {
			if _, ok := seen[seed.ParentPath]; !ok {
				t.Fatalf("catalog entry %q parent %q must be declared before child", seed.Path, seed.ParentPath)
			}
		}
		seen[seed.Path] = struct{}{}
	}
}

func TestMenuCatalogUsesBackOfficeGroups(t *testing.T) {
	expectedParents := map[string]string{
		"/admins":       "/access",
		"/roles":        "/access",
		"/menus":        "/access",
		"/apis":         "/access",
		"/api-tokens":   "/access",
		"/configs":      "/system",
		"/params":       "/system",
		"/versions":     "/system",
		"/dictionaries": "/system",
		"/files":        "/resources",
		"/logs":         "/audit",
	}
	expectedGroups := map[string]struct{}{
		"/access":    {},
		"/system":    {},
		"/resources": {},
		"/audit":     {},
	}
	seenGroups := map[string]struct{}{}

	for _, seed := range MenuCatalog() {
		if wantParent, ok := expectedParents[seed.Path]; ok && seed.ParentPath != wantParent {
			t.Fatalf("catalog entry %q ParentPath = %q, want %q", seed.Path, seed.ParentPath, wantParent)
		}
		if _, ok := expectedGroups[seed.Path]; !ok {
			continue
		}
		if seed.ParentPath != "" {
			t.Fatalf("menu group %q ParentPath = %q, want empty", seed.Path, seed.ParentPath)
		}
		if seed.Permission != "" {
			t.Fatalf("menu group %q Permission = %q, want empty", seed.Path, seed.Permission)
		}
		if len(seed.Buttons) != 0 {
			t.Fatalf("menu group %q buttons = %d, want 0", seed.Path, len(seed.Buttons))
		}
		seenGroups[seed.Path] = struct{}{}
	}

	for group := range expectedGroups {
		if _, ok := seenGroups[group]; !ok {
			t.Fatalf("menu group %q is missing", group)
		}
	}
}

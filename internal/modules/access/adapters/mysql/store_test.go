package mysql

import (
	"reflect"
	"strings"
	"testing"

	"github.com/NSObjects/echo-admin/internal/modules/access/domain"
)

func planUpgradeCatalog() authorizationCatalog {
	return authorizationCatalog{
		permissionTokens: domain.PermissionCatalogTokens(),
		apiIDs:           []int64{1, 2, 3},
		menuIDs:          []int64{10, 20},
		buttonIDs:        []int64{100, 200},
	}
}

func assertRoleUpgradePlan(t *testing.T, got, want roleUpgradePlan) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan =\n got  %+v\n want %+v", got, want)
	}
}

func TestPlanRoleUpgradeRootReceivesCompleteCatalog(t *testing.T) {
	catalog := planUpgradeCatalog()
	want := roleUpgradePlan{
		permissions: catalog.permissionTokens,
		apiIDs:      []int64{1, 2, 3},
		menuIDs:     []int64{10, 20},
		buttonIDs:   []int64{100, 200},
	}
	tests := []struct {
		name string
		role roleAuthorization
	}{
		{
			name: "with current grants",
			role: roleAuthorization{
				code:        domain.RoleCodeSuperAdmin,
				permissions: []string{domain.PermissionAdminRead},
				apiIDs:      []int64{1},
			},
		},
		{name: "with empty current grants", role: roleAuthorization{code: domain.RoleCodeSuperAdmin}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planRoleUpgrade(tt.role, catalog)
			if err != nil {
				t.Fatalf("planRoleUpgrade() error = %v", err)
			}
			assertRoleUpgradePlan(t, got, want)
		})
	}
}

func TestPlanRoleUpgradeRetainsOrdinaryGrants(t *testing.T) {
	catalog := planUpgradeCatalog()
	tests := []struct {
		name    string
		role    roleAuthorization
		want    roleUpgradePlan
		wantErr string
	}{
		{
			name: "keeps grants intersected with the catalog",
			role: roleAuthorization{
				code:        "operator",
				permissions: []string{domain.PermissionAdminRead, "garbage:token"},
				apiIDs:      []int64{3, 9, 9, 1},
				menuIDs:     []int64{10, 30},
				buttonIDs:   []int64{200, 300},
			},
			want: roleUpgradePlan{
				permissions: []string{domain.PermissionAdminRead},
				apiIDs:      []int64{3, 1},
				menuIDs:     []int64{10},
				buttonIDs:   []int64{200},
			},
		},
		{
			name:    "fails closed without valid permissions",
			role:    roleAuthorization{code: "operator", permissions: []string{"garbage:token"}},
			wantErr: "operator",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planRoleUpgrade(tt.role, catalog)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("planRoleUpgrade() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("planRoleUpgrade() error = %v", err)
			}
			assertRoleUpgradePlan(t, got, tt.want)
		})
	}
}

func TestPlanRoleUpgradeMapsRetiredTokens(t *testing.T) {
	catalog := planUpgradeCatalog()
	tests := []struct {
		name string
		role roleAuthorization
		want roleUpgradePlan
	}{
		{
			name: "retired api management maps to api read",
			role: roleAuthorization{
				code:        "operator",
				permissions: []string{retiredPermissionAPICreate, retiredPermissionAPIDelete},
			},
			want: roleUpgradePlan{
				permissions: []string{domain.PermissionAPIRead},
				apiIDs:      []int64{},
				menuIDs:     []int64{},
				buttonIDs:   []int64{},
			},
		},
		{
			name: "retired api update adds api grant",
			role: roleAuthorization{
				code:        "operator",
				permissions: []string{retiredPermissionAPIUpdate},
			},
			want: roleUpgradePlan{
				permissions: []string{domain.PermissionAPIRead, domain.PermissionAPIGrant},
				apiIDs:      []int64{},
				menuIDs:     []int64{},
				buttonIDs:   []int64{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planRoleUpgrade(tt.role, catalog)
			if err != nil {
				t.Fatalf("planRoleUpgrade() error = %v", err)
			}
			assertRoleUpgradePlan(t, got, tt.want)
		})
	}
}

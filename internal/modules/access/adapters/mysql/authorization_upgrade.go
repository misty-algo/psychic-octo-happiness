package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/modules/access/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysqljson"
)

const (
	retiredPermissionAPICreate = "api:create"
	retiredPermissionAPIUpdate = "api:update"
	retiredPermissionAPIDelete = "api:delete"
)

// UpgradeManagedAPIRouteCatalog explicitly reconciles persisted authorization
// data with the deployment-owned catalog. Applications must invoke this as an
// operator-controlled upgrade; normal process startup never repairs grants.
func (s *Store) UpgradeManagedAPIRouteCatalog(ctx context.Context) error {
	if ctx == nil {
		return errors.New("upgrade managed api route catalog: nil context")
	}
	if s == nil || s.db == nil {
		return errors.New("upgrade managed api route catalog: nil store")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return (&Store{db: tx}).upgradeManagedAPIRouteCatalogData(ctx)
	}); err != nil {
		return fmt.Errorf("upgrade managed api route catalog data: %w", err)
	}

	// MySQL DDL commits independently from the data transaction. Run it last:
	// a failed drop leaves authorization correct and the command safely rerunnable.
	migrator := s.db.WithContext(ctx).Migrator()
	if migrator.HasColumn(&apiModel{}, "public") {
		if err := migrator.DropColumn(&apiModel{}, "public"); err != nil {
			return apperr.WrapDatabase(err, "drop retired api public column")
		}
	}
	return nil
}

func (s *Store) upgradeManagedAPIRouteCatalogData(ctx context.Context) error {
	if err := s.seedPermissions(ctx); err != nil {
		return err
	}
	apiIDs, err := s.seedAPIs(ctx)
	if err != nil {
		return err
	}
	if _, _, err := s.seedMenus(ctx); err != nil {
		return err
	}
	permissionTokens := domain.PermissionCatalogTokens()
	if err := s.deleteRetiredAuthorizationRows(ctx, permissionTokens, apiIDs); err != nil {
		return err
	}
	menuIDs, err := s.catalogIDs(ctx, &menuModel{})
	if err != nil {
		return err
	}
	buttonIDs, err := s.catalogIDs(ctx, &menuButtonModel{})
	if err != nil {
		return err
	}
	catalog := authorizationCatalog{permissionTokens: permissionTokens, apiIDs: apiIDs, menuIDs: menuIDs, buttonIDs: buttonIDs}
	return s.upgradeRoleAuthorization(ctx, catalog)
}

func (s *Store) deleteRetiredAuthorizationRows(ctx context.Context, permissionTokens []string, apiIDs []int64) error {
	if err := s.db.WithContext(ctx).Where("id NOT IN ?", apiIDs).Delete(&apiModel{}).Error; err != nil {
		return apperr.WrapDatabase(err, "delete retired api routes")
	}
	if err := s.db.WithContext(ctx).Where("token NOT IN ?", permissionTokens).Delete(&permissionModel{}).Error; err != nil {
		return apperr.WrapDatabase(err, "delete retired permissions")
	}
	return nil
}

func (s *Store) catalogIDs(ctx context.Context, model any) ([]int64, error) {
	var ids []int64
	if err := s.db.WithContext(ctx).Model(model).Order("id").Pluck("id", &ids).Error; err != nil {
		return nil, apperr.WrapDatabase(err, "list authorization catalog ids")
	}
	return ids, nil
}

// authorizationCatalog carries the persisted catalog identity sets that role
// grants are normalized onto during an upgrade.
type authorizationCatalog struct {
	permissionTokens []string
	apiIDs           []int64
	menuIDs          []int64
	buttonIDs        []int64
}

// roleAuthorization carries one role's current grant collections as planning
// input.
type roleAuthorization struct {
	code        string
	permissions []string
	apiIDs      []int64
	menuIDs     []int64
	buttonIDs   []int64
}

// roleUpgradePlan is the authorization state one role must hold after upgrade.
type roleUpgradePlan struct {
	permissions []string
	apiIDs      []int64
	menuIDs     []int64
	buttonIDs   []int64
}

// planRoleUpgrade normalizes one role's authorization onto the current
// catalog. The root role receives the complete catalog regardless of its
// current grants, and the static permission catalog is never empty so no
// empty-grant check applies to it; ordinary roles keep their current grants
// intersected with the catalog, with retired API-management tokens mapped to
// their read and grant replacements. A role left without any valid permission
// is an error so the upgrade fails closed instead of silently emptying a role.
func planRoleUpgrade(role roleAuthorization, catalog authorizationCatalog) (roleUpgradePlan, error) {
	if role.code == domain.RoleCodeSuperAdmin {
		return roleUpgradePlan{
			permissions: append([]string(nil), catalog.permissionTokens...),
			apiIDs:      append([]int64(nil), catalog.apiIDs...),
			menuIDs:     append([]int64(nil), catalog.menuIDs...),
			buttonIDs:   append([]int64(nil), catalog.buttonIDs...),
		}, nil
	}
	permissions := upgradeRolePermissions(role.permissions)
	if len(permissions) == 0 {
		return roleUpgradePlan{}, fmt.Errorf("upgrade role authorization: role %q has no current permissions", role.code)
	}
	return roleUpgradePlan{
		permissions: permissions,
		apiIDs:      retainCatalogIDs(role.apiIDs, catalog.apiIDs),
		menuIDs:     retainCatalogIDs(role.menuIDs, catalog.menuIDs),
		buttonIDs:   retainCatalogIDs(role.buttonIDs, catalog.buttonIDs),
	}, nil
}

func (s *Store) upgradeRoleAuthorization(ctx context.Context, catalog authorizationCatalog) error {
	var roles []roleModel
	if err := s.db.WithContext(ctx).Order("id").Find(&roles).Error; err != nil {
		return apperr.WrapDatabase(err, "list roles for authorization upgrade")
	}
	for _, role := range roles {
		plan, err := planRoleUpgrade(roleAuthorization{
			code:        role.Code,
			permissions: []string(role.Permissions),
			apiIDs:      []int64(role.APIIDs),
			menuIDs:     []int64(role.MenuIDs),
			buttonIDs:   []int64(role.ButtonIDs),
		}, catalog)
		if err != nil {
			return err
		}
		if err := s.updateRoleAuthorization(ctx, role, plan); err != nil {
			return err
		}
	}
	return nil
}

func upgradeRolePermissions(current []string) []string {
	allowed := make(map[string]struct{})
	for _, token := range domain.PermissionCatalogTokens() {
		allowed[token] = struct{}{}
	}
	out := make([]string, 0, len(current)+2)
	seen := make(map[string]struct{}, len(current)+2)
	hadRetiredAPIManagement := false
	hadRetiredAPIUpdate := false
	for _, token := range current {
		switch token {
		case retiredPermissionAPICreate, retiredPermissionAPIDelete:
			hadRetiredAPIManagement = true
			continue
		case retiredPermissionAPIUpdate:
			hadRetiredAPIManagement = true
			hadRetiredAPIUpdate = true
			continue
		}
		if _, ok := allowed[token]; !ok {
			continue
		}
		out = appendUniqueToken(out, seen, token)
	}
	// The retired API management UI always operated on the visible catalog.
	// Preserve that least-privilege read capability; only former update holders
	// inherit the replacement role-grant capability.
	if hadRetiredAPIManagement {
		out = appendUniqueToken(out, seen, domain.PermissionAPIRead)
	}
	if hadRetiredAPIUpdate {
		out = appendUniqueToken(out, seen, domain.PermissionAPIGrant)
	}
	return out
}

func appendUniqueToken(tokens []string, seen map[string]struct{}, token string) []string {
	if _, ok := seen[token]; ok {
		return tokens
	}
	seen[token] = struct{}{}
	return append(tokens, token)
}

func retainCatalogIDs(current, catalog []int64) []int64 {
	allowed := make(map[int64]struct{}, len(catalog))
	for _, id := range catalog {
		allowed[id] = struct{}{}
	}
	out := make([]int64, 0, len(current))
	seen := make(map[int64]struct{}, len(current))
	for _, id := range current {
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Store) updateRoleAuthorization(ctx context.Context, role roleModel, plan roleUpgradePlan) error {
	updates := map[string]any{
		"permissions": mysqljson.Strings(plan.permissions),
		"api_ids":     mysqljson.Int64s(plan.apiIDs),
		"menu_ids":    mysqljson.Int64s(plan.menuIDs),
		"button_ids":  mysqljson.Int64s(plan.buttonIDs),
		"updated_at":  time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Model(&roleModel{}).Where("id = ?", role.ID).Updates(updates).Error; err != nil {
		return apperr.WrapDatabase(err, "upgrade role authorization")
	}
	return nil
}

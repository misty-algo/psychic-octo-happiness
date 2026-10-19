package usecase

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"

	"github.com/NSObjects/echo-admin/internal/modules/access/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

const casbinRBACModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

// CurrentAuthorization returns the subject's current active-role grants and
// visible administration navigation.
func (u *Usecase) CurrentAuthorization(ctx context.Context, subject AuthorizationSubject) (AuthorizationView, error) {
	if err := u.ready(); err != nil {
		return AuthorizationView{}, err
	}
	state, err := u.authorizationAdminState(ctx, subject)
	if err != nil {
		return AuthorizationView{}, err
	}
	snapshot, err := u.buildAuthorizationSnapshot(ctx, subject, state.RoleIDs)
	if err != nil {
		return AuthorizationView{}, err
	}
	menus, err := u.visibleAuthorizationMenus(ctx, snapshot)
	if err != nil {
		return AuthorizationView{}, err
	}
	return AuthorizationView{
		ActiveRole:  snapshot.activeRole,
		Roles:       snapshot.roles,
		Permissions: snapshot.permissions,
		Menus:       menus,
		DefaultPath: snapshot.activeRole.DefaultPath,
	}, nil
}

// AuthorizeRoute verifies that the subject's active role explicitly grants one
// Managed API Route identified by its HTTP method and registered Echo pattern.
// Route grants are checked directly; Casbin permission tokens and the root role
// never bypass the explicit API grant.
func (u *Usecase) AuthorizeRoute(ctx context.Context, subject AuthorizationSubject, method, path string) error {
	if err := u.ready(); err != nil {
		return err
	}
	if _, err := u.authorizationAdminState(ctx, subject); err != nil {
		return err
	}
	role, err := u.store.FindRoleByID(ctx, subject.ActiveRoleID)
	if err != nil {
		if isNotFound(err) {
			return permissionDeniedRole(subject.ActiveRoleID)
		}
		return err
	}
	if !role.Active {
		return permissionDeniedRole(subject.ActiveRoleID)
	}
	api, err := u.findRouteAPI(ctx, method, path)
	if err != nil {
		return err
	}
	if !role.HasAPI(api.ID) {
		return apperr.NewPermissionDenied("api", api.Path)
	}
	return nil
}

func (u *Usecase) authorizationAdminState(ctx context.Context, subject AuthorizationSubject) (AdminRoleState, error) {
	if subject.AdminID <= 0 || subject.ActiveRoleID <= 0 {
		return AdminRoleState{}, apperr.NewUnauthorized()
	}
	state, err := u.admins.AdminRoleState(ctx, subject.AdminID)
	if err != nil {
		if isNotFound(err) {
			return AdminRoleState{}, apperr.NewUnauthorized()
		}
		return AdminRoleState{}, err
	}
	if !state.Active {
		return AdminRoleState{}, apperr.New(apperr.ErrAccountDisabled, "账号已停用，请联系管理员")
	}
	if !containsID(state.RoleIDs, subject.ActiveRoleID) {
		return AdminRoleState{}, apperr.NewUnauthorized()
	}
	return state, nil
}

func (u *Usecase) buildAuthorizationSnapshot(ctx context.Context, subject AuthorizationSubject, roleIDs []int64) (authorizationSnapshot, error) {
	enforcer, err := newCasbinEnforcer()
	if err != nil {
		return authorizationSnapshot{}, err
	}
	snapshot := authorizationSnapshot{
		enforcer: enforcer,
		user:     userSubject(subject.AdminID),
		roles:    make([]Role, 0, len(roleIDs)),
	}
	for _, roleID := range roleIDs {
		role, findErr := u.store.FindRoleByID(ctx, roleID)
		if findErr != nil {
			if roleID == subject.ActiveRoleID && isNotFound(findErr) {
				return authorizationSnapshot{}, permissionDeniedRole(subject.ActiveRoleID)
			}
			return authorizationSnapshot{}, findErr
		}
		if !role.Active {
			continue
		}
		snapshot.roles = append(snapshot.roles, fromRole(role))
		if role.ID != subject.ActiveRoleID {
			continue
		}
		snapshot.activeRole = fromRole(role)
		snapshot.menuIDs, snapshot.buttonIDs, err = addActiveRoleGrants(enforcer, snapshot.user, role)
		if err != nil {
			return authorizationSnapshot{}, err
		}
	}
	if snapshot.activeRole.ID == 0 {
		return authorizationSnapshot{}, permissionDeniedRole(subject.ActiveRoleID)
	}
	snapshot.permissions, err = implicitPermissions(enforcer, snapshot.user)
	if err != nil {
		return authorizationSnapshot{}, err
	}
	return snapshot, nil
}

func (u *Usecase) visibleAuthorizationMenus(ctx context.Context, snapshot authorizationSnapshot) ([]Menu, error) {
	menus, err := u.store.ListMenus(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Menu, 0, len(menus))
	for _, menu := range menus {
		if !menu.Active || !hasID(snapshot.menuIDs, menu.ID) {
			continue
		}
		if menu.Permission != "" {
			allowed, enforceErr := enforcePermission(snapshot.enforcer, snapshot.user, menu.Permission)
			if enforceErr != nil {
				return nil, enforceErr
			}
			if !allowed {
				continue
			}
		}
		menu.Buttons = visibleAuthorizationButtons(menu.Buttons, snapshot)
		out = append(out, fromMenu(menu))
	}
	return out, nil
}

func visibleAuthorizationButtons(buttons []domain.MenuButton, snapshot authorizationSnapshot) []domain.MenuButton {
	if snapshot.activeRole.Code == domain.RoleCodeSuperAdmin {
		return buttons
	}
	out := make([]domain.MenuButton, 0, len(buttons))
	for _, button := range buttons {
		if hasID(snapshot.buttonIDs, button.ID) {
			out = append(out, button)
		}
	}
	return out
}

func addActiveRoleGrants(enforcer *casbin.Enforcer, user string, role domain.Role) (map[int64]struct{}, map[int64]struct{}, error) {
	roleName := roleSubject(role.Code)
	if _, err := enforcer.AddRoleForUser(user, roleName); err != nil {
		return nil, nil, fmt.Errorf("add casbin role: %w", err)
	}
	for _, permission := range role.Permissions {
		object, action, err := splitPermission(permission)
		if err != nil {
			return nil, nil, err
		}
		if _, err := enforcer.AddPolicy(roleName, object, action); err != nil {
			return nil, nil, fmt.Errorf("add casbin policy: %w", err)
		}
	}
	return idSet(role.MenuIDs), idSet(role.ButtonIDs), nil
}

func (u *Usecase) findRouteAPI(ctx context.Context, method, path string) (domain.API, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	if method == "" || path == "" {
		return domain.API{}, apperr.NewPermissionDenied("api", "route")
	}
	api, err := u.store.FindAPIByRoute(ctx, method, path)
	if err != nil {
		if isNotFound(err) {
			return domain.API{}, apperr.NewPermissionDenied("api", path)
		}
		return domain.API{}, err
	}
	return api, nil
}

func permissionDeniedRole(roleID int64) error {
	return apperr.NewPermissionDenied("role", strconv.FormatInt(roleID, 10))
}

func isNotFound(err error) bool {
	appErr, ok := apperr.Parse(err)
	return ok && appErr.Code() == apperr.ErrNotFound
}

type authorizationSnapshot struct {
	enforcer    *casbin.Enforcer
	user        string
	activeRole  Role
	roles       []Role
	permissions []string
	menuIDs     map[int64]struct{}
	buttonIDs   map[int64]struct{}
}

func newCasbinEnforcer() (*casbin.Enforcer, error) {
	rbacModel, err := model.NewModelFromString(casbinRBACModel)
	if err != nil {
		return nil, fmt.Errorf("create casbin model: %w", err)
	}
	enforcer, err := casbin.NewEnforcer(rbacModel)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}
	enforcer.EnableAutoSave(false)
	return enforcer, nil
}

func implicitPermissions(enforcer *casbin.Enforcer, user string) ([]string, error) {
	policies, err := enforcer.GetImplicitPermissionsForUser(user)
	if err != nil {
		return nil, fmt.Errorf("get casbin permissions: %w", err)
	}
	set := make(map[string]struct{}, len(policies))
	for _, policy := range policies {
		if len(policy) >= 3 {
			set[policy[1]+":"+policy[2]] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for permission := range set {
		out = append(out, permission)
	}
	sort.Strings(out)
	return out, nil
}

func enforcePermission(enforcer *casbin.Enforcer, user, permission string) (bool, error) {
	object, action, err := splitPermission(permission)
	if err != nil {
		return false, err
	}
	allowed, err := enforcer.Enforce(user, object, action)
	if err != nil {
		return false, fmt.Errorf("enforce casbin permission: %w", err)
	}
	return allowed, nil
}

func splitPermission(permission string) (string, string, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(permission)), ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", apperr.NewBadRequest("invalid permission")
	}
	return parts[0], parts[1], nil
}

func userSubject(adminID int64) string {
	return "user:" + strconv.FormatInt(adminID, 10)
}

func roleSubject(code string) string {
	return "role:" + code
}

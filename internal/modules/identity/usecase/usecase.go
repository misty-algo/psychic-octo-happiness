package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/NSObjects/echo-admin/internal/modules/identity/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

// List returns paginated administrators visible to the active role.
func (u *Usecase) List(ctx context.Context, input ListInput) (ListOutput, error) {
	if err := u.ready(); err != nil {
		return ListOutput{}, err
	}
	filter, err := normalizeListInput(input)
	if err != nil {
		return ListOutput{}, err
	}
	admins, err := u.store.ListAll(ctx)
	if err != nil {
		return ListOutput{}, err
	}
	visibleRoleIDs, err := u.visibleRoleSet(ctx)
	if err != nil {
		return ListOutput{}, err
	}
	scoped := filterAdminsByRoles(admins, visibleRoleIDs)
	pageAdmins, err := paginateAdmins(scoped, filter.Window)
	if err != nil {
		return ListOutput{}, err
	}
	return ListOutput{
		Items:    mapAdmins(pageAdmins),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    len(scoped),
	}, nil
}

// Create validates and stores a new administrator.
func (u *Usecase) Create(ctx context.Context, input AdminInput) (Admin, error) {
	if err := u.ready(); err != nil {
		return Admin{}, err
	}
	if err := u.roles.EnsureAssignableRoles(ctx, input.RoleIDs); err != nil {
		return Admin{}, err
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		return Admin{}, err
	}
	admin, err := domain.RestoreAdmin(0, input.Username, input.DisplayName, input.Email, hash, input.RoleIDs, input.ActiveRoleID, input.Active, zeroTime(), zeroTime())
	if err != nil {
		return Admin{}, mapDomainError(err)
	}
	created, err := u.store.Create(ctx, admin)
	if err != nil {
		return Admin{}, err
	}
	return fromAdmin(created), nil
}

// Update applies mutable administrator changes.
func (u *Usecase) Update(ctx context.Context, input UpdateAdminInput) (Admin, error) {
	if err := u.ready(); err != nil {
		return Admin{}, err
	}
	existing, err := u.store.FindByID(ctx, input.ID)
	if err != nil {
		return Admin{}, err
	}
	if checkErr := u.roles.EnsureAssignableRoles(ctx, existing.RoleIDs); checkErr != nil {
		return Admin{}, checkErr
	}
	if checkErr := rejectSelfDisable(ctx, existing.ID, input.Active); checkErr != nil {
		return Admin{}, checkErr
	}
	updated, err := u.updateAdmin(ctx, existing, input)
	if err != nil {
		return Admin{}, err
	}
	if identitySecurityEvent(existing, input) {
		// Revoke before saving security-sensitive identity changes so a
		// failed revocation cannot leave old browser sessions alive after a
		// password reset.
		if err := u.sessions.RevokeLoginSessions(ctx, existing.ID); err != nil {
			return Admin{}, err
		}
	}
	saved, err := u.store.Update(ctx, updated)
	if err != nil {
		return Admin{}, err
	}
	return fromAdmin(saved), nil
}

// Delete removes an administrator within the active role delegation scope.
func (u *Usecase) Delete(ctx context.Context, id int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	existing, err := u.store.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if checkErr := u.roles.EnsureAssignableRoles(ctx, existing.RoleIDs); checkErr != nil {
		return checkErr
	}
	if checkErr := rejectSelfDelete(ctx, existing.ID); checkErr != nil {
		return checkErr
	}
	if err := u.store.Delete(ctx, existing.ID); err != nil {
		return err
	}
	return u.sessions.RevokeLoginSessions(ctx, existing.ID)
}

// RoleAdminIDs returns visible administrators currently assigned to one role.
func (u *Usecase) RoleAdminIDs(ctx context.Context, roleID int64) ([]int64, error) {
	if err := u.ready(); err != nil {
		return nil, err
	}
	if roleID <= 0 {
		return nil, apperr.NewBadRequest("invalid role id")
	}
	visibleRoleIDs, err := u.visibleRoleSet(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := visibleRoleIDs[roleID]; !ok {
		return nil, apperr.NewPermissionDenied("role", strconv.FormatInt(roleID, 10))
	}
	admins, err := u.store.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return adminIDsWithRole(admins, roleID, visibleRoleIDs), nil
}

// SetRoleAdmins replaces visible administrator membership for one assignable role.
func (u *Usecase) SetRoleAdmins(ctx context.Context, input RoleAdminsInput) ([]int64, error) {
	assignment, err := u.prepareRoleAdminAssignment(ctx, input)
	if err != nil {
		return nil, err
	}
	admins, err := u.applyRoleAdminAssignment(ctx, assignment)
	if err != nil {
		return nil, err
	}
	return adminIDsWithRole(admins, assignment.roleID, assignment.visibleRoleIDs), nil
}

type roleAdminAssignment struct {
	roleID         int64
	adminIDs       []int64
	admins         []domain.Admin
	visibleRoleIDs map[int64]struct{}
	currentAdminID int64
}

func (u *Usecase) prepareRoleAdminAssignment(ctx context.Context, input RoleAdminsInput) (roleAdminAssignment, error) {
	if err := u.ready(); err != nil {
		return roleAdminAssignment{}, err
	}
	if input.RoleID <= 0 {
		return roleAdminAssignment{}, apperr.NewBadRequest("invalid role id")
	}
	ensureErr := u.roles.EnsureAssignableRoles(ctx, []int64{input.RoleID})
	if ensureErr != nil {
		return roleAdminAssignment{}, ensureErr
	}
	adminIDs, err := normalizeRequestedAdminIDs(input.AdminIDs)
	if err != nil {
		return roleAdminAssignment{}, err
	}
	admins, err := u.store.ListAll(ctx)
	if err != nil {
		return roleAdminAssignment{}, err
	}
	visibleRoleIDs, err := u.visibleRoleSet(ctx)
	if err != nil {
		return roleAdminAssignment{}, err
	}
	visibilityErr := ensureAdminsVisible(admins, adminIDs, visibleRoleIDs)
	if visibilityErr != nil {
		return roleAdminAssignment{}, visibilityErr
	}
	currentID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return roleAdminAssignment{}, err
	}
	return roleAdminAssignment{
		roleID:         input.RoleID,
		adminIDs:       adminIDs,
		admins:         admins,
		visibleRoleIDs: visibleRoleIDs,
		currentAdminID: currentID,
	}, nil
}

func (u *Usecase) applyRoleAdminAssignment(ctx context.Context, assignment roleAdminAssignment) ([]domain.Admin, error) {
	wanted := idSet(assignment.adminIDs)
	admins := assignment.admins
	for index, admin := range admins {
		if !allRolesAllowed(admin.RoleIDs, assignment.visibleRoleIDs) {
			continue
		}
		shouldHaveRole := hasID(wanted, admin.ID)
		updated, changed, err := roleAdminProfile(admin, assignment.roleID, shouldHaveRole, assignment.currentAdminID)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		saved, updateErr := u.store.Update(ctx, updated)
		if updateErr != nil {
			return nil, updateErr
		}
		admins[index] = saved
	}
	return admins, nil
}

func roleAdminProfile(admin domain.Admin, roleID int64, shouldHaveRole bool, currentAdminID int64) (domain.Admin, bool, error) {
	if admin.HasRole(roleID) == shouldHaveRole {
		return domain.Admin{}, false, nil
	}
	if !shouldHaveRole && admin.ID == currentAdminID {
		return domain.Admin{}, false, apperr.NewBadRequest("cannot remove current admin from role")
	}
	nextRoleIDs := admin.RoleIDs
	activeRoleID := admin.ActiveRoleID
	if shouldHaveRole {
		nextRoleIDs = appendRoleID(nextRoleIDs, roleID)
	} else {
		nextRoleIDs = removeRoleID(nextRoleIDs, roleID)
		if activeRoleID == roleID {
			activeRoleID = 0
		}
	}
	updated, err := admin.UpdateProfile(admin.DisplayName, admin.Email, nextRoleIDs, activeRoleID, admin.Active)
	if err != nil {
		return domain.Admin{}, false, mapDomainError(err)
	}
	return updated, true, nil
}

func (u *Usecase) ready() error {
	if u == nil || u.store == nil || u.roles == nil || u.sessions == nil {
		return apperr.New(apperr.ErrInternalServer, "identity dependencies are not configured")
	}
	return nil
}

func identitySecurityEvent(existing domain.Admin, input UpdateAdminInput) bool {
	passwordReset := input.Password != nil
	disabled := input.Active != nil && existing.Active && !*input.Active
	return passwordReset || disabled
}

func (u *Usecase) updateAdmin(ctx context.Context, existing domain.Admin, input UpdateAdminInput) (domain.Admin, error) {
	displayName := existing.DisplayName
	if input.DisplayName != nil {
		displayName = *input.DisplayName
	}
	email := existing.Email
	if input.Email != nil {
		email = *input.Email
	}
	active := existing.Active
	if input.Active != nil {
		active = *input.Active
	}
	roleIDs := coalesceIDs(input.RoleIDs, existing.RoleIDs)
	if err := u.roles.EnsureAssignableRoles(ctx, roleIDs); err != nil {
		return domain.Admin{}, err
	}
	activeRoleID := existing.ActiveRoleID
	if input.ActiveRoleID != nil {
		activeRoleID = *input.ActiveRoleID
	} else if !containsID(roleIDs, activeRoleID) {
		activeRoleID = 0
	}
	updated, err := existing.UpdateProfile(displayName, email, roleIDs, activeRoleID, active)
	if err != nil {
		return domain.Admin{}, mapDomainError(err)
	}
	if input.Password == nil {
		return updated, nil
	}
	hash, err := hashPassword(*input.Password)
	if err != nil {
		return domain.Admin{}, err
	}
	updated, err = updated.ReplacePassword(hash)
	if err != nil {
		return domain.Admin{}, mapDomainError(err)
	}
	return updated, nil
}

func (u *Usecase) visibleRoleSet(ctx context.Context) (map[int64]struct{}, error) {
	ids, err := u.roles.VisibleRoleIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

func rejectSelfDisable(ctx context.Context, adminID int64, active *bool) error {
	if active == nil || *active {
		return nil
	}
	currentID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	if currentID == adminID {
		return apperr.NewBadRequest("不能停用当前登录的管理员")
	}
	return nil
}

// The current administrator must keep at least one usable session owner; deleting
// self would immediately invalidate accountability for the request in progress.
func rejectSelfDelete(ctx context.Context, adminID int64) error {
	currentID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	if currentID == adminID {
		return apperr.NewBadRequest("不能删除当前登录的管理员")
	}
	return nil
}

func hashPassword(password string) ([]byte, error) {
	if len(password) < 8 || len(password) > 72 {
		return nil, apperr.NewBadRequest("invalid password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

func normalizeListInput(input ListInput) (ListFilter, error) {
	window, err := pagination.Normalize(input.Page, input.PageSize, pagination.DefaultOptions)
	if err != nil {
		return ListFilter{}, apperr.NewBadRequest("invalid pagination")
	}
	return ListFilter{Window: window}, nil
}

func paginateAdmins(admins []domain.Admin, window pagination.Window) ([]domain.Admin, error) {
	start, end, ok, err := pagination.Bounds(len(admins), window.Offset, window.Limit)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []domain.Admin{}, nil
	}
	return admins[start:end], nil
}

func filterAdminsByRoles(admins []domain.Admin, allowed map[int64]struct{}) []domain.Admin {
	out := make([]domain.Admin, 0, len(admins))
	for _, admin := range admins {
		if allRolesAllowed(admin.RoleIDs, allowed) {
			out = append(out, admin)
		}
	}
	return out
}

func allRolesAllowed(roleIDs []int64, allowed map[int64]struct{}) bool {
	if len(roleIDs) == 0 {
		return false
	}
	for _, roleID := range roleIDs {
		if _, ok := allowed[roleID]; !ok {
			return false
		}
	}
	return true
}

func adminIDsWithRole(admins []domain.Admin, roleID int64, visibleRoleIDs map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(admins))
	for _, admin := range admins {
		if admin.HasRole(roleID) && allRolesAllowed(admin.RoleIDs, visibleRoleIDs) {
			out = append(out, admin.ID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func ensureAdminsVisible(admins []domain.Admin, adminIDs []int64, visibleRoleIDs map[int64]struct{}) error {
	byID := make(map[int64]domain.Admin, len(admins))
	for _, admin := range admins {
		byID[admin.ID] = admin
	}
	for _, adminID := range adminIDs {
		admin, ok := byID[adminID]
		if !ok {
			return apperr.NewNotFound("admin")
		}
		if !allRolesAllowed(admin.RoleIDs, visibleRoleIDs) {
			return apperr.NewPermissionDenied("admin", strconv.FormatInt(adminID, 10))
		}
	}
	return nil
}

func coalesceIDs(next, fallback []int64) []int64 {
	if next == nil {
		return fallback
	}
	return next
}

func normalizeRequestedAdminIDs(ids []int64) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, apperr.NewBadRequest("invalid admin id")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func idSet(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func hasID(ids map[int64]struct{}, want int64) bool {
	_, ok := ids[want]
	return ok
}

func appendRoleID(roleIDs []int64, roleID int64) []int64 {
	if containsID(roleIDs, roleID) {
		return roleIDs
	}
	out := append([]int64(nil), roleIDs...)
	out = append(out, roleID)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func removeRoleID(roleIDs []int64, roleID int64) []int64 {
	out := make([]int64, 0, len(roleIDs))
	for _, assignedID := range roleIDs {
		if assignedID != roleID {
			out = append(out, assignedID)
		}
	}
	return out
}

func containsID(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func mapAdmins(admins []domain.Admin) []Admin {
	out := make([]Admin, 0, len(admins))
	for _, admin := range admins {
		out = append(out, fromAdmin(admin))
	}
	return out
}

func fromAdmin(admin domain.Admin) Admin {
	return Admin{
		ID:           admin.ID,
		Username:     admin.Username,
		DisplayName:  admin.DisplayName,
		Email:        admin.Email,
		RoleIDs:      admin.RoleIDs,
		ActiveRoleID: admin.ActiveRoleID,
		Active:       admin.Active,
		CreatedAt:    admin.CreatedAt,
		UpdatedAt:    admin.UpdatedAt,
	}
}

func mapDomainError(err error) error {
	for _, entry := range domainErrorMessages {
		if errors.Is(err, entry.err) {
			return apperr.NewBadRequest(entry.message)
		}
	}
	return err
}

func zeroTime() time.Time {
	return time.Time{}
}

var domainErrorMessages = []struct {
	err     error
	message string
}{
	{domain.ErrInvalidAdminID, "invalid admin id"},
	{domain.ErrInvalidUsername, "invalid username"},
	{domain.ErrInvalidDisplayName, "invalid display name"},
	{domain.ErrInvalidPasswordHash, "invalid password"},
	{domain.ErrAdminNeedsRole, "admin needs a role"},
	{domain.ErrInvalidActiveRole, "active role is not assigned"},
}

package usecase_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	identitydomain "github.com/NSObjects/echo-admin/internal/modules/identity/domain"
	"github.com/NSObjects/echo-admin/internal/modules/identity/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

func TestDeleteRejectsCurrentAdmin(t *testing.T) {
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		42: newAdmin(t, 42, "admin"),
	}}
	sessions := &sessionRevokerSpy{}
	uc := usecase.New(store, roleScopeSpy{}, sessions)

	err := uc.Delete(requestctx.WithUserID(context.Background(), 42), 42)
	if err == nil {
		t.Fatal("Delete(current admin) error = nil, want bad request")
	}
	if store.deletedID != 0 {
		t.Fatalf("deletedID = %d, want 0", store.deletedID)
	}
	if len(sessions.adminIDs) != 0 {
		t.Fatalf("revoked sessions = %v, want none", sessions.adminIDs)
	}
}

func TestDeleteRemovesScopedAdmin(t *testing.T) {
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdmin(t, 7, "operator"),
	}}
	sessions := &sessionRevokerSpy{}
	uc := usecase.New(store, roleScopeSpy{}, sessions)

	if err := uc.Delete(requestctx.WithUserID(context.Background(), 42), 7); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.deletedID != 7 {
		t.Fatalf("deletedID = %d, want 7", store.deletedID)
	}
	if !reflect.DeepEqual(sessions.adminIDs, []int64{7}) {
		t.Fatalf("revoked sessions = %v, want [7]", sessions.adminIDs)
	}
}

func TestUpdateRevokesSessionsWhenAdminIsDisabled(t *testing.T) {
	inactive := false
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdmin(t, 7, "operator"),
	}}
	sessions := &sessionRevokerSpy{}
	uc := usecase.New(store, roleScopeSpy{}, sessions)

	_, err := uc.Update(requestctx.WithUserID(context.Background(), 42), usecase.UpdateAdminInput{
		ID:     7,
		Active: &inactive,
	})
	if err != nil {
		t.Fatalf("Update(disable admin) error = %v", err)
	}
	if !reflect.DeepEqual(sessions.adminIDs, []int64{7}) {
		t.Fatalf("revoked sessions = %v, want [7]", sessions.adminIDs)
	}
}

func TestUpdateRevokesSessionsWhenPasswordIsReset(t *testing.T) {
	password := "new-password"
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdmin(t, 7, "operator"),
	}}
	sessions := &sessionRevokerSpy{}
	uc := usecase.New(store, roleScopeSpy{}, sessions)

	_, err := uc.Update(requestctx.WithUserID(context.Background(), 42), usecase.UpdateAdminInput{
		ID:       7,
		Password: &password,
	})
	if err != nil {
		t.Fatalf("Update(reset password) error = %v", err)
	}
	if !reflect.DeepEqual(sessions.adminIDs, []int64{7}) {
		t.Fatalf("revoked sessions = %v, want [7]", sessions.adminIDs)
	}
}

func TestUpdateRejectsSecurityChangeWhenSessionRevocationFails(t *testing.T) {
	password := "new-password"
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdmin(t, 7, "operator"),
	}}
	sessions := &sessionRevokerSpy{err: errors.New("revoke sessions")}
	uc := usecase.New(store, roleScopeSpy{}, sessions)

	_, err := uc.Update(requestctx.WithUserID(context.Background(), 42), usecase.UpdateAdminInput{
		ID:       7,
		Password: &password,
	})
	if err == nil {
		t.Fatal("Update(reset password with revocation failure) error = nil, want error")
	}
	if !reflect.DeepEqual(sessions.adminIDs, []int64{7}) {
		t.Fatalf("revoked sessions = %v, want [7]", sessions.adminIDs)
	}
	if store.updateCalls != 0 {
		t.Fatalf("updateCalls = %d, want 0", store.updateCalls)
	}
}

func TestUpdateProfileDoesNotRevokeSessions(t *testing.T) {
	displayName := "Operator"
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdmin(t, 7, "operator"),
	}}
	sessions := &sessionRevokerSpy{}
	uc := usecase.New(store, roleScopeSpy{}, sessions)

	_, err := uc.Update(requestctx.WithUserID(context.Background(), 42), usecase.UpdateAdminInput{
		ID:          7,
		DisplayName: &displayName,
	})
	if err != nil {
		t.Fatalf("Update(display name) error = %v", err)
	}
	if len(sessions.adminIDs) != 0 {
		t.Fatalf("revoked sessions = %v, want none", sessions.adminIDs)
	}
}

func TestListFiltersAdminsByVisibleRoles(t *testing.T) {
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdminWithRoles(t, 7, "operator", []int64{1}),
		8: newAdminWithRoles(t, 8, "auditor", []int64{2}),
	}}
	uc := usecase.New(store, roleScopeSpy{visibleIDs: []int64{2}}, &sessionRevokerSpy{})

	output, err := uc.List(context.Background(), usecase.ListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if output.Total != 1 {
		t.Fatalf("List() total = %d, want 1", output.Total)
	}
	if len(output.Items) != 1 || output.Items[0].Username != "auditor" {
		t.Fatalf("List() items = %+v, want auditor only", output.Items)
	}
}

func TestRoleAdminIDsFiltersAdminsByVisibleRoles(t *testing.T) {
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdminWithRoles(t, 7, "operator", []int64{1}),
		8: newAdminWithRoles(t, 8, "auditor", []int64{2}),
		9: newAdminWithRoles(t, 9, "mixed", []int64{1, 2}),
	}}
	uc := usecase.New(store, roleScopeSpy{visibleIDs: []int64{1}}, &sessionRevokerSpy{})

	got, err := uc.RoleAdminIDs(context.Background(), 1)
	if err != nil {
		t.Fatalf("RoleAdminIDs() error = %v", err)
	}
	want := []int64{7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RoleAdminIDs() = %v, want %v", got, want)
	}
}

func TestSetRoleAdminsReplacesVisibleRoleMembership(t *testing.T) {
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdminWithRoles(t, 7, "operator", []int64{1, 2}),
		8: newAdminWithRoles(t, 8, "auditor", []int64{1}),
		9: newAdminWithRoles(t, 9, "reviewer", []int64{1, 2}),
	}}
	uc := usecase.New(store, roleScopeSpy{assignableIDs: []int64{2}, visibleIDs: []int64{1, 2}}, &sessionRevokerSpy{})

	got, err := uc.SetRoleAdmins(requestctx.WithUserID(context.Background(), 42), usecase.RoleAdminsInput{
		RoleID:   2,
		AdminIDs: []int64{8},
	})
	if err != nil {
		t.Fatalf("SetRoleAdmins() error = %v", err)
	}
	want := []int64{8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetRoleAdmins() = %v, want %v", got, want)
	}
	if store.admins[7].HasRole(2) {
		t.Fatal("admin 7 still has role 2, want removed")
	}
	if !store.admins[8].HasRole(2) {
		t.Fatal("admin 8 does not have role 2, want assigned")
	}
	if store.admins[9].HasRole(2) {
		t.Fatal("admin 9 still has role 2, want removed")
	}
}

func TestSetRoleAdminsRejectsRemovingOnlyRole(t *testing.T) {
	store := &storeSpy{admins: map[int64]identitydomain.Admin{
		7: newAdminWithRoles(t, 7, "operator", []int64{2}),
	}}
	uc := usecase.New(store, roleScopeSpy{assignableIDs: []int64{2}, visibleIDs: []int64{2}}, &sessionRevokerSpy{})

	_, err := uc.SetRoleAdmins(requestctx.WithUserID(context.Background(), 42), usecase.RoleAdminsInput{
		RoleID: 2,
	})
	if err == nil {
		t.Fatal("SetRoleAdmins(removing only role) error = nil, want bad request")
	}
	if !store.admins[7].HasRole(2) {
		t.Fatal("admin 7 role changed after failed SetRoleAdmins")
	}
}

func newAdmin(t *testing.T, id int64, username string) identitydomain.Admin {
	t.Helper()
	return newAdminWithRoles(t, id, username, []int64{1})
}

func newAdminWithRoles(t *testing.T, id int64, username string, roleIDs []int64) identitydomain.Admin {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	admin, err := identitydomain.RestoreAdmin(id, username, username, username+"@example.com", []byte("hash"), roleIDs, roleIDs[0], true, now, now)
	if err != nil {
		t.Fatalf("RestoreAdmin() error = %v", err)
	}
	return admin
}

type storeSpy struct {
	admins      map[int64]identitydomain.Admin
	updateCalls int
	deletedID   int64
}

func (s *storeSpy) FindByUsername(context.Context, string) (identitydomain.Admin, error) {
	return identitydomain.Admin{}, apperr.NewNotFound("admin")
}

func (s *storeSpy) FindByID(_ context.Context, id int64) (identitydomain.Admin, error) {
	admin, ok := s.admins[id]
	if !ok {
		return identitydomain.Admin{}, apperr.NewNotFound("admin")
	}
	return admin, nil
}

func (s *storeSpy) ListAll(context.Context) ([]identitydomain.Admin, error) {
	admins := make([]identitydomain.Admin, 0, len(s.admins))
	for _, admin := range s.admins {
		admins = append(admins, admin)
	}
	return admins, nil
}

func (s *storeSpy) Create(_ context.Context, admin identitydomain.Admin) (identitydomain.Admin, error) {
	return admin, nil
}

func (s *storeSpy) Update(_ context.Context, admin identitydomain.Admin) (identitydomain.Admin, error) {
	s.updateCalls++
	if s.admins != nil {
		s.admins[admin.ID] = admin
	}
	return admin, nil
}

func (s *storeSpy) Delete(_ context.Context, id int64) error {
	s.deletedID = id
	return nil
}

type roleScopeSpy struct {
	assignableIDs []int64
	visibleIDs    []int64
}

func (s roleScopeSpy) AssignableRoleIDs(context.Context) ([]int64, error) {
	return idsOrDefault(s.assignableIDs), nil
}

func (s roleScopeSpy) VisibleRoleIDs(context.Context) ([]int64, error) {
	return idsOrDefault(s.visibleIDs), nil
}

func (s roleScopeSpy) EnsureAssignableRoles(_ context.Context, roleIDs []int64) error {
	allowed := testIDSet(idsOrDefault(s.assignableIDs))
	for _, roleID := range roleIDs {
		if _, ok := allowed[roleID]; !ok {
			return apperr.NewPermissionDenied("role", "")
		}
	}
	return nil
}

func idsOrDefault(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{1}
	}
	return ids
}

func testIDSet(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

type sessionRevokerSpy struct {
	adminIDs []int64
	err      error
}

func (s *sessionRevokerSpy) RevokeLoginSessions(_ context.Context, adminID int64) error {
	s.adminIDs = append(s.adminIDs, adminID)
	return s.err
}

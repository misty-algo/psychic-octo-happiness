package usecase_test

import (
	"context"
	"testing"
	"time"

	apitokendomain "github.com/NSObjects/echo-admin/internal/modules/apitoken/domain"
	"github.com/NSObjects/echo-admin/internal/modules/apitoken/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

const knownSecret = "ea_known_secret"

func TestCreateTokenStoresHashAndReturnsSecretOnce(t *testing.T) {
	store := &tokenStore{}
	uc := usecase.New(
		store,
		adminReader{admins: map[int64]usecase.AdminSnapshot{7: {RoleIDs: []int64{9}, Active: true}}},
		rolePolicy{},
		usecase.WithClock(fixedTime),
		usecase.WithSecretSource(func() (string, error) { return knownSecret, nil }),
	)
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 7), 9)

	created, err := uc.CreateToken(ctx, usecase.TokenInput{Name: "Deploy Bot", Active: true, Days: 30})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	if created.Secret != knownSecret {
		t.Fatalf("Secret = %q, want %q", created.Secret, knownSecret)
	}
	if store.created.SecretHash == created.Secret {
		t.Fatal("stored token secret hash equals raw secret")
	}
	hash, err := apitokendomain.HashSecret(created.Secret)
	if err != nil {
		t.Fatalf("HashSecret() error = %v", err)
	}
	if store.created.SecretHash != hash {
		t.Fatalf("stored hash = %q, want %q", store.created.SecretHash, hash)
	}
	if store.created.AdminID != 7 || store.created.RoleID != 9 {
		t.Fatalf("created identity = (%d,%d), want (7,9)", store.created.AdminID, store.created.RoleID)
	}
	wantExpiry := fixedTime().Add(30 * 24 * time.Hour)
	if store.created.ExpiresAt == nil || !store.created.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("created expiry = %v, want %v", store.created.ExpiresAt, wantExpiry)
	}
}

func TestCreateTokenAllowsSuperAdminTargetAdminRole(t *testing.T) {
	store := &tokenStore{}
	uc := usecase.New(
		store,
		adminReader{admins: map[int64]usecase.AdminSnapshot{22: {RoleIDs: []int64{5}, Active: true}}},
		rolePolicy{super: map[int64]bool{1: true}},
		usecase.WithClock(fixedTime),
		usecase.WithSecretSource(func() (string, error) { return knownSecret, nil }),
	)
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 1), 1)

	_, err := uc.CreateToken(ctx, usecase.TokenInput{AdminID: 22, RoleID: 5, Name: "Deploy Bot", Active: true, Days: 10})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	if store.created.AdminID != 22 || store.created.RoleID != 5 {
		t.Fatalf("created identity = (%d,%d), want (22,5)", store.created.AdminID, store.created.RoleID)
	}
	wantExpiry := fixedTime().Add(10 * 24 * time.Hour)
	if store.created.ExpiresAt == nil || !store.created.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("created expiry = %v, want %v", store.created.ExpiresAt, wantExpiry)
	}
}

func TestCreateTokenRejectsNonSuperTargetAdminRole(t *testing.T) {
	store := &tokenStore{}
	uc := usecase.New(
		store,
		adminReader{admins: map[int64]usecase.AdminSnapshot{
			7: {RoleIDs: []int64{9}, Active: true},
			8: {RoleIDs: []int64{9}, Active: true},
		}},
		rolePolicy{},
		usecase.WithClock(fixedTime),
		usecase.WithSecretSource(func() (string, error) { return knownSecret, nil }),
	)
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 7), 9)

	_, err := uc.CreateToken(ctx, usecase.TokenInput{AdminID: 8, RoleID: 9, Name: "Deploy Bot", Active: true, Days: 30})
	if err == nil {
		t.Fatal("CreateToken() error = nil, want permission denial")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindForbidden {
		t.Fatalf("CreateToken() kind = %v, want %s", def.Kind, apperr.KindForbidden)
	}
}

func TestCreateTokenRejectsAdminMissingTargetRole(t *testing.T) {
	store := &tokenStore{}
	uc := usecase.New(
		store,
		adminReader{admins: map[int64]usecase.AdminSnapshot{22: {RoleIDs: []int64{6}, Active: true}}},
		rolePolicy{super: map[int64]bool{1: true}},
		usecase.WithClock(fixedTime),
		usecase.WithSecretSource(func() (string, error) { return knownSecret, nil }),
	)
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 1), 1)

	_, err := uc.CreateToken(ctx, usecase.TokenInput{AdminID: 22, RoleID: 5, Name: "Deploy Bot", Active: true, Days: 30})
	if err == nil {
		t.Fatal("CreateToken() error = nil, want missing role rejection")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("CreateToken() kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
}

func TestListTokensScopesNonSuperToCurrentAdmin(t *testing.T) {
	active := true
	store := &tokenStore{}
	uc := usecase.New(
		store,
		adminReader{admins: map[int64]usecase.AdminSnapshot{7: {RoleIDs: []int64{9}, Active: true}}},
		rolePolicy{},
		usecase.WithClock(fixedTime),
	)
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 7), 9)

	_, err := uc.ListTokens(ctx, usecase.ListInput{Page: 1, PageSize: 20, AdminID: 99, Active: &active})
	if err != nil {
		t.Fatalf("ListTokens() error = %v", err)
	}
	if store.listFilter.AdminID != 7 {
		t.Fatalf("list admin filter = %d, want 7", store.listFilter.AdminID)
	}
	if store.listFilter.Active == nil || *store.listFilter.Active != active {
		t.Fatalf("list active filter = %v, want true", store.listFilter.Active)
	}
}

func TestDeleteTokenRejectsOtherAdminForNonSuper(t *testing.T) {
	token := mustToken(t, 12, 8, 9, true, fixedTime().Add(time.Hour))
	store := &tokenStore{byID: map[int64]apitokendomain.APIToken{12: token}}
	uc := usecase.New(
		store,
		adminReader{admins: map[int64]usecase.AdminSnapshot{7: {RoleIDs: []int64{9}, Active: true}}},
		rolePolicy{},
		usecase.WithClock(fixedTime),
	)
	ctx := requestctx.WithRoleID(requestctx.WithUserID(context.Background(), 7), 9)

	err := uc.DeleteToken(ctx, 12)
	if err == nil {
		t.Fatal("DeleteToken() error = nil, want permission denial")
	}
	if store.deletedID != 0 {
		t.Fatalf("deletedID = %d, want 0", store.deletedID)
	}
}

func TestAuthenticateTouchesLastUsedForActiveToken(t *testing.T) {
	hash, err := apitokendomain.HashSecret(knownSecret)
	if err != nil {
		t.Fatalf("HashSecret() error = %v", err)
	}
	expiresAt := fixedTime().Add(time.Hour)
	token, err := apitokendomain.RestoreAPIToken(12, 7, 9, "Deploy Bot", "", "ea_known_sec", hash, true, &expiresAt, nil, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPIToken() error = %v", err)
	}
	store := &tokenStore{byHash: map[string]apitokendomain.APIToken{hash: token}}
	uc := usecase.New(store, nil, nil, usecase.WithClock(fixedTime))

	identity, err := uc.Authenticate(context.Background(), knownSecret)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if identity.AdminID != 7 || identity.RoleID != 9 {
		t.Fatalf("identity = (%d,%d), want (7,9)", identity.AdminID, identity.RoleID)
	}
	if store.touchedID != 12 || !store.touchedAt.Equal(fixedTime()) {
		t.Fatalf("touch = (%d,%s), want (12,%s)", store.touchedID, store.touchedAt, fixedTime())
	}
}

func TestAuthenticateRejectsExpiredTokenWithoutTouch(t *testing.T) {
	hash, err := apitokendomain.HashSecret(knownSecret)
	if err != nil {
		t.Fatalf("HashSecret() error = %v", err)
	}
	expiresAt := fixedTime().Add(-time.Minute)
	token, err := apitokendomain.RestoreAPIToken(12, 7, 9, "Deploy Bot", "", "ea_known_sec", hash, true, &expiresAt, nil, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPIToken() error = %v", err)
	}
	store := &tokenStore{byHash: map[string]apitokendomain.APIToken{hash: token}}
	uc := usecase.New(store, nil, nil, usecase.WithClock(fixedTime))

	if _, err := uc.Authenticate(context.Background(), knownSecret); err == nil {
		t.Fatal("Authenticate() error = nil, want expired token rejection")
	}
	if store.touchedID != 0 {
		t.Fatalf("touchedID = %d, want 0", store.touchedID)
	}
}

func TestAuthenticateMapsMissingTokenToUnauthorized(t *testing.T) {
	store := &tokenStore{byHash: map[string]apitokendomain.APIToken{}}
	uc := usecase.New(store, nil, nil, usecase.WithClock(fixedTime))

	_, err := uc.Authenticate(context.Background(), "ea_missing_secret")
	if err == nil {
		t.Fatal("Authenticate() error = nil, want unauthorized")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindUnauthorized {
		t.Fatalf("Authenticate() kind = %v, want %s", def.Kind, apperr.KindUnauthorized)
	}
}

type tokenStore struct {
	created    apitokendomain.APIToken
	byID       map[int64]apitokendomain.APIToken
	byHash     map[string]apitokendomain.APIToken
	listCalls  int
	listFilter usecase.ListFilter
	deletedID  int64
	touchedID  int64
	touchedAt  time.Time
}

func (s *tokenStore) ListTokens(_ context.Context, filter usecase.ListFilter) ([]apitokendomain.APIToken, int, error) {
	s.listCalls++
	s.listFilter = filter
	return nil, 0, nil
}

func (s *tokenStore) FindTokenByID(_ context.Context, id int64) (apitokendomain.APIToken, error) {
	if token, ok := s.byID[id]; ok {
		return token, nil
	}
	if s.created.ID == id || s.created.ID == 0 {
		return s.created, nil
	}
	return apitokendomain.APIToken{}, apperr.NewNotFound("api token")
}

func (s *tokenStore) FindTokenByHash(_ context.Context, hash string) (apitokendomain.APIToken, error) {
	token, ok := s.byHash[hash]
	if !ok {
		return apitokendomain.APIToken{}, apperr.NewNotFound("api token")
	}
	return token, nil
}

func (s *tokenStore) CreateToken(_ context.Context, token apitokendomain.APIToken) (apitokendomain.APIToken, error) {
	token.ID = 1
	token.CreatedAt = fixedTime()
	token.UpdatedAt = fixedTime()
	s.created = token
	return s.created, nil
}

func (s *tokenStore) UpdateToken(_ context.Context, token apitokendomain.APIToken) (apitokendomain.APIToken, error) {
	return token, nil
}

func (s *tokenStore) DeleteToken(_ context.Context, id int64) error {
	s.deletedID = id
	return nil
}

func (s *tokenStore) TouchLastUsed(_ context.Context, id int64, at time.Time) error {
	s.touchedID = id
	s.touchedAt = at
	return nil
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}

type adminReader struct {
	admins map[int64]usecase.AdminSnapshot
}

func (r adminReader) AdminSnapshot(_ context.Context, adminID int64) (usecase.AdminSnapshot, error) {
	admin, ok := r.admins[adminID]
	if !ok {
		return usecase.AdminSnapshot{}, apperr.NewNotFound("admin")
	}
	return admin, nil
}

type rolePolicy struct {
	super map[int64]bool
}

func (p rolePolicy) RoleView(_ context.Context, roleID int64) (usecase.RoleView, error) {
	return usecase.RoleView{IsSuper: p.super[roleID]}, nil
}

func mustToken(t *testing.T, id, adminID, roleID int64, active bool, expiresAt time.Time) apitokendomain.APIToken {
	t.Helper()
	hash, err := apitokendomain.HashSecret(knownSecret)
	if err != nil {
		t.Fatalf("HashSecret() error = %v", err)
	}
	token, err := apitokendomain.RestoreAPIToken(id, adminID, roleID, "Deploy Bot", "", "ea_known_sec", hash, active, &expiresAt, nil, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreAPIToken() error = %v", err)
	}
	return token
}

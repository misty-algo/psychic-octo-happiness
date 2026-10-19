package apitokenhttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	apitokendomain "github.com/NSObjects/echo-admin/internal/modules/apitoken/domain"
	apitokenhttp "github.com/NSObjects/echo-admin/internal/modules/apitoken/http"
	"github.com/NSObjects/echo-admin/internal/modules/apitoken/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

func TestCreateTokenRecordsOperation(t *testing.T) {
	e, store, recorder := newTokenEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/api-tokens", `{"name":"Deploy Bot","active":true,"days":30}`, 42, 7)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.created.AdminID != 42 || store.created.RoleID != 7 {
		t.Fatalf("created identity = (%d,%d), want (42,7)", store.created.AdminID, store.created.RoleID)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if recorder.records[0].Resource != "api_token" {
		t.Fatalf("operation resource = %q, want api_token", recorder.records[0].Resource)
	}
	var body responseEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body.Data.Secret != "ea_known_secret" {
		t.Fatalf("secret = %q, want ea_known_secret", body.Data.Secret)
	}
}

func newTokenEcho() (*echo.Echo, *tokenStore, *operationRecorder) {
	store := &tokenStore{}
	uc := usecase.New(
		store,
		tokenAdminReader{admins: map[int64]usecase.AdminSnapshot{42: {RoleIDs: []int64{7}, Active: true}}},
		tokenRolePolicy{},
		usecase.WithClock(fixedTime),
		usecase.WithSecretSource(func() (string, error) { return "ea_known_secret", nil }),
	)
	recorder := &operationRecorder{}
	handler := apitokenhttp.New(uc, oprec.New(recorder))

	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	apitokenhttp.Register(e.Group("/api"), handler)
	return e, store, recorder
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string, userID, roleID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	req = req.WithContext(requestctx.WithRoleID(requestctx.WithUserID(req.Context(), userID), roleID))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

type responseEnvelope struct {
	Data struct {
		Secret string `json:"secret"`
	} `json:"data"`
}

type tokenStore struct {
	created    apitokendomain.APIToken
	listCalls  int
	listFilter usecase.ListFilter
}

func (s *tokenStore) ListTokens(_ context.Context, filter usecase.ListFilter) ([]apitokendomain.APIToken, int, error) {
	s.listCalls++
	s.listFilter = filter
	return nil, 0, nil
}

func (s *tokenStore) FindTokenByID(context.Context, int64) (apitokendomain.APIToken, error) {
	return s.created, nil
}

func (s *tokenStore) FindTokenByHash(context.Context, string) (apitokendomain.APIToken, error) {
	return apitokendomain.APIToken{}, apperr.NewNotFound("api token")
}

func (s *tokenStore) CreateToken(_ context.Context, token apitokendomain.APIToken) (apitokendomain.APIToken, error) {
	token.ID = 99
	token.CreatedAt = fixedTime()
	token.UpdatedAt = fixedTime()
	s.created = token
	return token, nil
}

func (s *tokenStore) UpdateToken(_ context.Context, token apitokendomain.APIToken) (apitokendomain.APIToken, error) {
	return token, nil
}

func (s *tokenStore) DeleteToken(context.Context, int64) error {
	return nil
}

func (s *tokenStore) TouchLastUsed(context.Context, int64, time.Time) error {
	return nil
}

type operationRecorder struct {
	records []auditusecase.OperationInput
}

func (r *operationRecorder) RecordOperation(_ context.Context, input auditusecase.OperationInput) (auditusecase.OperationLog, error) {
	r.records = append(r.records, input)
	return auditusecase.OperationLog{}, nil
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}

type tokenAdminReader struct {
	admins map[int64]usecase.AdminSnapshot
}

func (r tokenAdminReader) AdminSnapshot(_ context.Context, adminID int64) (usecase.AdminSnapshot, error) {
	admin, ok := r.admins[adminID]
	if !ok {
		return usecase.AdminSnapshot{}, apperr.NewNotFound("admin")
	}
	return admin, nil
}

type tokenRolePolicy struct{}

func (tokenRolePolicy) RoleView(context.Context, int64) (usecase.RoleView, error) {
	return usecase.RoleView{}, nil
}

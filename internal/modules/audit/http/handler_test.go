package audithttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	auditdomain "github.com/NSObjects/echo-admin/internal/modules/audit/domain"
	audithttp "github.com/NSObjects/echo-admin/internal/modules/audit/http"
	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

func TestListOperationLogs(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doGET(e, "/api/logs/operations")
	if rec.Code != http.StatusOK {
		t.Fatalf("operation logs status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.listOperationCalls != 1 {
		t.Fatalf("listOperationCalls = %d, want 1", store.listOperationCalls)
	}
}

func TestListSystemErrorLogs(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doGET(e, "/api/logs/errors")
	if rec.Code != http.StatusOK {
		t.Fatalf("system error logs status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.listSystemErrorCalls != 1 {
		t.Fatalf("listSystemErrorCalls = %d, want 1", store.listSystemErrorCalls)
	}
}

func TestReadOperationLog(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doGET(e, "/api/logs/operations/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("get operation log status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.findOperationCalls != 1 {
		t.Fatalf("findOperationCalls = %d, want 1", store.findOperationCalls)
	}
}

func TestReadLoginLog(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doGET(e, "/api/logs/logins/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("get login log status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.findLoginCalls != 1 {
		t.Fatalf("findLoginCalls = %d, want 1", store.findLoginCalls)
	}
}

func TestReadSystemErrorLog(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doGET(e, "/api/logs/errors/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("get system error log status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.findSystemErrorCalls != 1 {
		t.Fatalf("findSystemErrorCalls = %d, want 1", store.findSystemErrorCalls)
	}
}

func TestDeleteOperationLog(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doDELETE(e, "/api/logs/operations/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete operation log status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !sameInt64s(store.deletedOperationIDs, []int64{1}) {
		t.Fatalf("deleted operation ids = %v, want [1]", store.deletedOperationIDs)
	}
}

func TestBatchDeleteLoginLogsDeletesSelectedRecords(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doPOST(e, "/api/logs/logins/batch-delete", `{"ids":[1,2]}`, 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch delete login logs status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !sameInt64s(store.deletedLoginIDs, []int64{1, 2}) {
		t.Fatalf("deleted login ids = %v, want [1 2]", store.deletedLoginIDs)
	}
}

func TestResolveSystemErrorLogRequiresResolvePermission(t *testing.T) {
	e, store := newAuditEcho(t)

	rec := doPOST(e, "/api/logs/errors/1/resolve", `{"note":"handled by restart"}`, 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve system error status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !store.updatedSystemError.Resolved {
		t.Fatal("updated system error resolved = false, want true")
	}
	if store.updatedSystemError.ResolvedBy != 42 {
		t.Fatalf("updated system error resolver = %d, want 42", store.updatedSystemError.ResolvedBy)
	}
	if store.updatedSystemError.ResolveNote != "handled by restart" {
		t.Fatalf("updated system error note = %q, want handled by restart", store.updatedSystemError.ResolveNote)
	}
}

func TestReopenSystemErrorLogClearsResolution(t *testing.T) {
	e, store := newAuditEcho(t)
	resolved, err := store.systemErrorLog.Resolve("fixed", 42, fixedTime())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	store.systemErrorLog = resolved

	rec := doDELETE(e, "/api/logs/errors/1/resolve")
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen system error status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.updatedSystemError.Resolved {
		t.Fatal("updated system error resolved = true, want false")
	}
	if store.updatedSystemError.ResolveNote != "" {
		t.Fatalf("updated system error note = %q, want empty", store.updatedSystemError.ResolveNote)
	}
}

func newAuditEcho(t *testing.T) (*echo.Echo, *auditStore) {
	t.Helper()
	store := newAuditStore(t)
	uc := auditusecase.New(store)
	handler := audithttp.New(uc)

	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	audithttp.Register(e.Group("/api"), handler)
	return e, store
}

func doGET(e *echo.Echo, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doDELETE(e *echo.Echo, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doPOST(e *echo.Echo, path, body string, userID int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(requestctx.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func newAuditStore(t *testing.T) *auditStore {
	t.Helper()
	log, err := auditdomain.RestoreOperationLog(1, 42, "create", "admin", "1", http.MethodPost, "/api/admins", "127.0.0.1", "test", true, "created admin", fixedTime())
	if err != nil {
		t.Fatalf("RestoreOperationLog() error = %v", err)
	}
	login, err := auditdomain.RestoreLoginLog(1, 42, "admin", "127.0.0.1", "test", true, "login succeeded", fixedTime())
	if err != nil {
		t.Fatalf("RestoreLoginLog() error = %v", err)
	}
	systemError, err := auditdomain.RestoreSystemErrorLog(1, apperr.ErrInternalServer, "Internal server error", "database failed", http.MethodGet, "/api/admins", "127.0.0.1", "test", "req-1", "42", false, "", 0, time.Time{}, fixedTime())
	if err != nil {
		t.Fatalf("RestoreSystemErrorLog() error = %v", err)
	}
	return &auditStore{operationLog: log, loginLog: login, systemErrorLog: systemError}
}

type auditStore struct {
	listOperationCalls   int
	listLoginCalls       int
	listSystemErrorCalls int
	findOperationCalls   int
	findLoginCalls       int
	findSystemErrorCalls int
	operationLog         auditdomain.OperationLog
	loginLog             auditdomain.LoginLog
	systemErrorLog       auditdomain.SystemErrorLog
	updatedSystemError   auditdomain.SystemErrorLog
	deletedOperationIDs  []int64
	deletedLoginIDs      []int64
	deletedSystemErrIDs  []int64
}

func (s *auditStore) RecordOperation(ctx context.Context, log auditdomain.OperationLog) (auditdomain.OperationLog, error) {
	if err := ctx.Err(); err != nil {
		return auditdomain.OperationLog{}, err
	}
	return log, nil
}

func (s *auditStore) ListOperationLogs(ctx context.Context, _ auditusecase.ListFilter) ([]auditdomain.OperationLog, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.listOperationCalls++
	return []auditdomain.OperationLog{s.operationLog}, 1, nil
}

func (s *auditStore) FindOperationLog(ctx context.Context, _ int64) (auditdomain.OperationLog, error) {
	if err := ctx.Err(); err != nil {
		return auditdomain.OperationLog{}, err
	}
	s.findOperationCalls++
	return s.operationLog, nil
}

func (s *auditStore) DeleteOperationLogs(_ context.Context, ids []int64) error {
	s.deletedOperationIDs = append([]int64(nil), ids...)
	return nil
}

func (s *auditStore) RecordLogin(ctx context.Context, log auditdomain.LoginLog) (auditdomain.LoginLog, error) {
	if err := ctx.Err(); err != nil {
		return auditdomain.LoginLog{}, err
	}
	return log, nil
}

func (s *auditStore) ListLoginLogs(ctx context.Context, _ auditusecase.ListFilter) ([]auditdomain.LoginLog, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.listLoginCalls++
	return []auditdomain.LoginLog{s.loginLog}, 1, nil
}

func (s *auditStore) FindLoginLog(ctx context.Context, _ int64) (auditdomain.LoginLog, error) {
	if err := ctx.Err(); err != nil {
		return auditdomain.LoginLog{}, err
	}
	s.findLoginCalls++
	return s.loginLog, nil
}

func (s *auditStore) DeleteLoginLogs(_ context.Context, ids []int64) error {
	s.deletedLoginIDs = append([]int64(nil), ids...)
	return nil
}

func (s *auditStore) RecordSystemError(ctx context.Context, log auditdomain.SystemErrorLog) (auditdomain.SystemErrorLog, error) {
	if err := ctx.Err(); err != nil {
		return auditdomain.SystemErrorLog{}, err
	}
	return log, nil
}

func (s *auditStore) ListSystemErrorLogs(ctx context.Context, _ auditusecase.ListFilter) ([]auditdomain.SystemErrorLog, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.listSystemErrorCalls++
	return []auditdomain.SystemErrorLog{s.systemErrorLog}, 1, nil
}

func (s *auditStore) FindSystemErrorLog(ctx context.Context, _ int64) (auditdomain.SystemErrorLog, error) {
	if err := ctx.Err(); err != nil {
		return auditdomain.SystemErrorLog{}, err
	}
	s.findSystemErrorCalls++
	return s.systemErrorLog, nil
}

func (s *auditStore) UpdateSystemErrorLog(_ context.Context, log auditdomain.SystemErrorLog) (auditdomain.SystemErrorLog, error) {
	s.updatedSystemError = log
	s.systemErrorLog = log
	return log, nil
}

func (s *auditStore) DeleteSystemErrorLogs(_ context.Context, ids []int64) error {
	s.deletedSystemErrIDs = append([]int64(nil), ids...)
	return nil
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
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

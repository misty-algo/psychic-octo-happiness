package oprec_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

type auditSpy struct {
	input     auditusecase.OperationInput
	recordErr error
}

func (s *auditSpy) RecordOperation(_ context.Context, input auditusecase.OperationInput) (auditusecase.OperationLog, error) {
	s.input = input
	return auditusecase.OperationLog{}, s.recordErr
}

func newEchoContext(t *testing.T) *echo.Context {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/roles", nil)
	req.Header.Set("User-Agent", "test-agent")
	req = req.WithContext(requestctx.WithUserID(req.Context(), 42))
	var captured *echo.Context
	e := echo.New()
	e.POST("/api/roles", func(c *echo.Context) error {
		captured = c
		return c.String(http.StatusOK, "ok")
	})
	e.ServeHTTP(httptest.NewRecorder(), req)
	return captured
}

func TestRecordMarksSuccessFromOperationError(t *testing.T) {
	tests := []struct {
		name     string
		opErr    error
		wantSucc bool
	}{
		{"successful operation", nil, true},
		{"failed operation", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audit := &auditSpy{}
			recorder := oprec.New(audit)

			got := recorder.Record(newEchoContext(t), "create", "role", "1", "created role", tt.opErr)
			if !errors.Is(got, tt.opErr) {
				t.Fatalf("Record() error = %v, want operation error %v", got, tt.opErr)
			}
			if audit.input.Success != tt.wantSucc {
				t.Fatalf("recorded Success = %v, want %v", audit.input.Success, tt.wantSucc)
			}
			if audit.input.ActorID != 42 {
				t.Fatalf("recorded ActorID = %d, want 42", audit.input.ActorID)
			}
			if audit.input.Method != http.MethodPost {
				t.Fatalf("recorded Method = %q, want POST", audit.input.Method)
			}
		})
	}
}

func TestRecordReturnsAuditErrorForSuccessfulOperation(t *testing.T) {
	auditErr := errors.New("audit down")
	recorder := oprec.New(&auditSpy{recordErr: auditErr})

	got := recorder.Record(newEchoContext(t), "create", "role", "1", "created role", nil)
	if !errors.Is(got, auditErr) {
		t.Fatalf("Record() error = %v, want audit error %v", got, auditErr)
	}
}

func TestRecordPrefersOperationErrorOverAuditError(t *testing.T) {
	opErr := errors.New("role code exists")
	recorder := oprec.New(&auditSpy{recordErr: errors.New("audit down")})

	got := recorder.Record(newEchoContext(t), "create", "role", "0", "created role", opErr)
	if !errors.Is(got, opErr) {
		t.Fatalf("Record() error = %v, want operation error %v", got, opErr)
	}
}

func TestRecordRejectsMissingActor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/roles", nil)
	req = req.WithContext(context.Background())
	var captured *echo.Context
	e := echo.New()
	e.POST("/api/roles", func(c *echo.Context) error {
		captured = c
		return c.String(http.StatusOK, "ok")
	})
	e.ServeHTTP(httptest.NewRecorder(), req)

	recorder := oprec.New(&auditSpy{})
	got := recorder.Record(captured, "create", "role", "1", "created role", nil)
	if got == nil {
		t.Fatal("Record() error = nil, want unauthorized for missing actor")
	}
}

func TestRecordRejectsInvalidActorWithoutWritingAudit(t *testing.T) {
	tests := []struct {
		name   string
		userID int64
	}{
		{name: "zero actor", userID: 0},
		{name: "negative actor", userID: -5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/roles", nil)
			req = req.WithContext(requestctx.WithUserID(context.Background(), tt.userID))
			var captured *echo.Context
			e := echo.New()
			e.POST("/api/roles", func(c *echo.Context) error {
				captured = c
				return c.String(http.StatusOK, "ok")
			})
			e.ServeHTTP(httptest.NewRecorder(), req)

			audit := &auditSpy{}
			recorder := oprec.New(audit)
			got := recorder.Record(captured, "create", "role", "1", "created role", nil)
			if got == nil {
				t.Fatal("Record() error = nil, want unauthorized for invalid actor")
			}
			if info := apperr.NewInfo(got); info.Kind != apperr.KindUnauthorized {
				t.Fatalf("kind = %s, want %s", info.Kind, apperr.KindUnauthorized)
			}
			if audit.input.ActorID != 0 {
				t.Fatalf("recorded ActorID = %d, want 0 because invalid actors must not reach audit", audit.input.ActorID)
			}
		})
	}
}

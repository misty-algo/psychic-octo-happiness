package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	setuphttp "github.com/NSObjects/echo-admin/internal/modules/setup/http"
	"github.com/NSObjects/echo-admin/internal/modules/setup/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

func TestStateReturnsInitializedFlag(t *testing.T) {
	e, uc := newEcho()
	uc.state = usecase.State{Initialized: true}

	rec := doJSON(t, e, http.MethodGet, "/api/setup/state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("state status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	data := responseData(t, rec)
	if data["initialized"] != true {
		t.Fatalf("initialized = %v, want true", data["initialized"])
	}
	if uc.stateCalls != 1 {
		t.Fatalf("stateCalls = %d, want 1", uc.stateCalls)
	}
}

func TestSubmitPassesSetupInputToUsecase(t *testing.T) {
	e, uc := newEcho()
	uc.submitState = usecase.State{Initialized: true}

	body := `{"username":"root","display_name":"Root Admin","email":"root@example.com","password":"secret-password","site_name":"Control"}`
	rec := doJSON(t, e, http.MethodPost, "/api/setup", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if uc.submitCalls != 1 {
		t.Fatalf("submitCalls = %d, want 1", uc.submitCalls)
	}
	want := usecase.SubmitInput{
		Username:    "root",
		DisplayName: "Root Admin",
		Email:       "root@example.com",
		Password:    "secret-password",
		SiteName:    "Control",
	}
	if uc.input != want {
		t.Fatalf("submit input = %+v, want %+v", uc.input, want)
	}
	data := responseData(t, rec)
	if data["initialized"] != true {
		t.Fatalf("initialized = %v, want true", data["initialized"])
	}
}

func TestSubmitRejectsInvalidRequestBeforeUsecase(t *testing.T) {
	e, uc := newEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/setup", `{"username":"","display_name":"","password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("submit status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if uc.submitCalls != 0 {
		t.Fatalf("submitCalls = %d, want 0", uc.submitCalls)
	}
	code := responseCode(t, rec)
	if code != apperr.ErrValidation {
		t.Fatalf("response code = %d, want %d", code, apperr.ErrValidation)
	}
}

func TestSubmitRendersUsecaseConflict(t *testing.T) {
	e, uc := newEcho()
	uc.submitErr = apperr.NewConflict("system is already initialized")

	rec := doJSON(t, e, http.MethodPost, "/api/setup", `{"username":"root","display_name":"Root Admin","password":"secret-password"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("submit status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if uc.submitCalls != 1 {
		t.Fatalf("submitCalls = %d, want 1", uc.submitCalls)
	}
	code := responseCode(t, rec)
	if code != apperr.ErrConflict {
		t.Fatalf("response code = %d, want %d", code, apperr.ErrConflict)
	}
}

func newEcho() (*echo.Echo, *setupUsecaseSpy) {
	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	uc := &setupUsecaseSpy{}
	setuphttp.Register(e.Group("/api"), setuphttp.New(uc))
	return e, uc
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func responseData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got.Data
}

func responseCode(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	var got struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got.Code
}

type setupUsecaseSpy struct {
	state       usecase.State
	stateErr    error
	stateCalls  int
	submitState usecase.State
	submitErr   error
	submitCalls int
	input       usecase.SubmitInput
}

func (s *setupUsecaseSpy) State(context.Context) (usecase.State, error) {
	s.stateCalls++
	return s.state, s.stateErr
}

func (s *setupUsecaseSpy) Submit(_ context.Context, input usecase.SubmitInput) (usecase.State, error) {
	s.submitCalls++
	s.input = input
	return s.submitState, s.submitErr
}

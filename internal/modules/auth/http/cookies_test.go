package authhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

type staticSessionAuthenticator struct {
	identity middlewares.LoginSessionIdentity
}

func (a staticSessionAuthenticator) AuthenticateLoginSession(context.Context, string) (middlewares.LoginSessionIdentity, error) {
	return a.identity, nil
}

// TestLoginSessionRunsBeforeCSRFProtection locks the last link of the
// authentication order through the real assembly interface. A valid session
// cookie plus an unsafe method without a CSRF header can only be rejected
// when the login-session middleware runs first and writes the LoginSessionID
// that keeps this package's CSRF skipper active; if CSRF ran first the
// skipper would see no session and the request would pass.
func TestLoginSessionRunsBeforeCSRFProtection(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	err := middlewares.ApplyMiddlewares(e, &middlewares.MiddlewareConfig{
		EnableRequestContext: true,
		LoginSession: &middlewares.LoginSessionConfig{
			CookieName:    LoginSessionCookieName,
			Authenticator: staticSessionAuthenticator{identity: middlewares.LoginSessionIdentity{SessionID: 9, UserID: 99, RoleID: 9}},
		},
		CSRF: CSRFMiddlewareConfig(nil, false),
	})
	if err != nil {
		t.Fatalf("ApplyMiddlewares() error = %v", err)
	}
	e.POST("/private", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/private", nil)
	req.AddCookie(&http.Cookie{Name: LoginSessionCookieName, Value: "opaque-token"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (CSRF must reject a session-authenticated unsafe request without a CSRF header)", rec.Code, http.StatusBadRequest)
	}
}

package middlewares

import (
	"context"
	"errors"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/logging"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

// LoginSessionIdentity is the request identity produced by a verified browser
// login session. Fields are int64 database keys.
type LoginSessionIdentity struct {
	SessionID int64
	UserID    int64
	RoleID    int64
}

// LoginSessionAuthenticator validates browser login session credentials.
type LoginSessionAuthenticator interface {
	AuthenticateLoginSession(context.Context, string) (LoginSessionIdentity, error)
}

// LoginSessionConfig controls browser login-session authentication. A nil
// config in MiddlewareConfig leaves the middleware uninstalled; a present
// config must carry an Authenticator and the login-session cookie name — the
// name is Login Session domain policy declared by the auth module and
// injected by the composition root.
type LoginSessionConfig struct {
	CookieName    string
	Exemptions    []RouteExemption
	Authenticator LoginSessionAuthenticator
}

// LoginSession creates browser login-session authentication middleware.
func LoginSession(config LoginSessionConfig) (echo.MiddlewareFunc, error) {
	if config.Authenticator == nil {
		return nil, errors.New("login session authenticator is required when login sessions are installed")
	}
	cookieName := strings.TrimSpace(config.CookieName)
	if cookieName == "" {
		return nil, errors.New("login session cookie name is required when login sessions are installed")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if requestctx.GetUserID(c.Request().Context()) != 0 || MatchRouteExemption(c, config.Exemptions) {
				return next(c)
			}
			cookie, err := c.Cookie(cookieName)
			if err != nil || strings.TrimSpace(cookie.Value) == "" {
				return apperr.NewUnauthorized()
			}
			identity, err := config.Authenticator.AuthenticateLoginSession(c.Request().Context(), cookie.Value)
			if err != nil {
				return err
			}
			if identity.UserID <= 0 || identity.RoleID <= 0 || identity.SessionID <= 0 {
				return apperr.NewUnauthorized()
			}
			request := c.Request()
			ctx := requestctx.WithLoginSessionID(
				requestctx.WithRoleID(
					requestctx.WithUserID(request.Context(), identity.UserID),
					identity.RoleID,
				),
				identity.SessionID,
			)
			logger := logging.FromContext(ctx).With().
				Int64("user_id", identity.UserID).
				Int64("role_id", identity.RoleID).
				Int64("login_session_id", identity.SessionID).
				Str("auth", "login_session").
				Logger()
			c.SetRequest(request.WithContext(logger.WithContext(ctx)))
			return next(c)
		}
	}, nil
}

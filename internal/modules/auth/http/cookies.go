package authhttp

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

const (
	// LoginSessionCookieName is the HttpOnly browser credential for login
	// sessions. The composition root injects this name into the login-session
	// middleware so read and write sides share one declaration.
	LoginSessionCookieName = "login_session"
	// CSRFCookieName is the browser-readable double-submit CSRF cookie.
	CSRFCookieName = "csrf_token"

	csrfTokenBytes = 32
)

// SetLoginSessionCookie stores the opaque browser session credential.
func SetLoginSessionCookie(c *echo.Context, token string, expiresAt time.Time, secure bool) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetCookie(&http.Cookie{
		Name:     LoginSessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearLoginSessionCookie removes the browser session credential.
func ClearLoginSessionCookie(c *echo.Context, secure bool) {
	clearCookie(c, LoginSessionCookieName, true, secure)
}

// NewCSRFToken creates a browser-readable CSRF token for login responses.
func NewCSRFToken() (string, error) {
	token := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

// SetCSRFCookie stores the browser-readable CSRF token used by Echo's CSRF
// middleware.
func SetCSRFCookie(c *echo.Context, token string, expiresAt time.Time, secure bool) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetCookie(&http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt,
		Secure:   secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCSRFCookie removes the browser-readable CSRF token.
func ClearCSRFCookie(c *echo.Context, secure bool) {
	clearCookie(c, CSRFCookieName, false, secure)
}

// CSRFMiddlewareConfig returns the Echo CSRF middleware configuration used
// for browser login-session requests. It shares the login-session exemptions
// because a route without a login session has no CSRF obligation.
func CSRFMiddlewareConfig(exemptions []middlewares.RouteExemption, secure bool) middleware.CSRFConfig {
	return middleware.CSRFConfig{
		Skipper: func(c *echo.Context) bool {
			return requestctx.GetLoginSessionID(c.Request().Context()) == 0 ||
				middlewares.MatchRouteExemption(c, exemptions)
		},
		TokenLookup:    "header:" + echo.HeaderXCSRFToken,
		CookieName:     CSRFCookieName,
		CookiePath:     "/",
		CookieMaxAge:   int((12 * time.Hour).Seconds()),
		CookieSecure:   secure,
		CookieHTTPOnly: false,
		CookieSameSite: http.SameSiteLaxMode,
	}
}

func clearCookie(c *echo.Context, name string, httpOnly, secure bool) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteLaxMode,
	})
}

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

// APIKeyHeader is the header used for back-office API token authentication.
const APIKeyHeader = "X-API-Token"

// APIKeyIdentity is the request identity produced by a verified API token.
// Fields are int64 database keys.
type APIKeyIdentity struct {
	UserID int64
	RoleID int64
}

// APIKeyVerifier validates a raw API token without exposing token storage to the server.
type APIKeyVerifier interface {
	VerifyAPIKey(context.Context, string) (APIKeyIdentity, error)
}

// APIKeyConfig controls API token authentication middleware. A nil config in
// MiddlewareConfig leaves the middleware uninstalled; a present config must
// carry a Verifier.
type APIKeyConfig struct {
	Header   string
	Verifier APIKeyVerifier
}

// apiKey creates middleware that authenticates requests carrying an API token.
// An empty Header falls back to APIKeyHeader.
func apiKey(config APIKeyConfig) (echo.MiddlewareFunc, error) {
	if config.Verifier == nil {
		return nil, errors.New("api key verifier is required when api key authentication is installed")
	}
	header := strings.TrimSpace(config.Header)
	if header == "" {
		header = APIKeyHeader
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw := strings.TrimSpace(c.Request().Header.Get(header))
			if raw == "" {
				return next(c)
			}
			identity, err := config.Verifier.VerifyAPIKey(c.Request().Context(), raw)
			if err != nil {
				return err
			}
			if identity.UserID <= 0 || identity.RoleID <= 0 {
				return apperr.NewUnauthorized()
			}
			request := c.Request()
			ctx := requestctx.WithRoleID(requestctx.WithUserID(request.Context(), identity.UserID), identity.RoleID)
			logger := logging.FromContext(ctx).With().
				Int64("user_id", identity.UserID).
				Int64("role_id", identity.RoleID).
				Str("auth", "api_token").
				Logger()
			c.SetRequest(request.WithContext(logger.WithContext(ctx)))
			return next(c)
		}
	}, nil
}

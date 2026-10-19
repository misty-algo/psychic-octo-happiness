// Package middlewares contains server-owned Echo middleware adapters.
package middlewares

import (
	"context"
	"errors"
	"net/http"

	echootel "github.com/labstack/echo-opentelemetry"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"

	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/logging"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

// RouteExemption is one exact HTTP method plus registered route pattern that
// skips a middleware. Route policy forbids path-only, prefix, or wildcard
// exemptions, so matching is always exact.
type RouteExemption struct {
	Method string
	Path   string
}

// MiddlewareConfig controls server-owned HTTP middleware.
//
// Activation has a single source of truth per middleware: the Enable* bools
// below carry genuine runtime configuration, while API-key authentication,
// the installation gate, and login sessions are installed exactly when their
// config pointer is non-nil — the composition root constructs the pointer
// only when the dependency is injected. CSRF follows login sessions because a
// route without a login session has no CSRF obligation.
type MiddlewareConfig struct {
	EnableRecovery bool

	EnableRequestContext bool

	EnableLogger bool

	EnableTracing bool

	TracingServiceName string

	EnableGzip bool

	EnableCORS bool

	CORS middleware.CORSConfig

	APIKey *APIKeyConfig

	InstallationGate *InstallationGateConfig

	LoginSession *LoginSessionConfig

	CSRF middleware.CSRFConfig
}

// ApplyMiddlewares installs server-owned middleware. A nil config is a
// programming error, not a request for defaults.
func ApplyMiddlewares(e *echo.Echo, config *MiddlewareConfig) error {
	if config == nil {
		return errors.New("middleware config is required")
	}

	if config.EnableRecovery {
		e.Use(errorRecovery())
	}

	if config.EnableRequestContext {
		e.Use(RequestContext())
	}

	if config.EnableTracing {
		e.Use(echootel.NewMiddleware(config.TracingServiceName))
	}

	if config.EnableLogger {
		e.Use(requestLogger())
	}

	if config.EnableGzip {
		e.Use(middleware.Gzip())
	}

	installCORS(e, config)

	// The install order below is load-bearing: each authentication stage
	// writes requestctx facts the next stage reads. The gate must reject
	// before any verifier runs; API-key identity short-circuits login-session
	// lookup via requestctx.GetUserID; the CSRF skipper only protects requests
	// that carry a login-session ID.
	if err := installInstallationGate(e, config); err != nil {
		return err
	}

	if err := installAPIKey(e, config); err != nil {
		return err
	}

	if err := installLoginSession(e, config); err != nil {
		return err
	}

	installCSRF(e, config)
	return nil
}

// installCORS installs Echo CORS with install-time defaults. Configuration
// validation (origins required, wildcard with credentials forbidden) is owned
// by configs.Load, the single entry point every production value passes
// through; this layer trusts the normalized value.
func installCORS(e *echo.Echo, config *MiddlewareConfig) {
	if !config.EnableCORS {
		return
	}
	e.Use(middleware.CORSWithConfig(normalizedCORSConfig(config.CORS)))
}

func installAPIKey(e *echo.Echo, config *MiddlewareConfig) error {
	if config.APIKey == nil {
		return nil
	}
	apiKeyMiddleware, err := apiKey(*config.APIKey)
	if err != nil {
		return err
	}
	e.Use(apiKeyMiddleware)
	return nil
}

func installInstallationGate(e *echo.Echo, config *MiddlewareConfig) error {
	if config.InstallationGate == nil {
		return nil
	}
	gateMiddleware, err := installationGate(*config.InstallationGate)
	if err != nil {
		return err
	}
	e.Use(gateMiddleware)
	return nil
}

func installLoginSession(e *echo.Echo, config *MiddlewareConfig) error {
	if config.LoginSession == nil {
		return nil
	}
	sessionMiddleware, err := LoginSession(*config.LoginSession)
	if err != nil {
		return err
	}
	e.Use(sessionMiddleware)
	return nil
}

func installCSRF(e *echo.Echo, config *MiddlewareConfig) {
	// CSRF protects browser login sessions only: no login session, no CSRF
	// obligation. A missing Skipper means the composition root never injected
	// the auth-owned CSRF configuration, and Echo's default skipper would
	// silently break that obligation, so a zero config skips installation.
	if config.LoginSession == nil || config.CSRF.Skipper == nil {
		return
	}
	e.Use(middleware.CSRFWithConfig(config.CSRF))
}

func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		BeforeNextFunc: func(c *echo.Context) {
			request := c.Request()
			ctx := contextWithActiveTrace(request.Context())
			logger := requestLoggerFromContext(ctx)
			c.SetRequest(request.WithContext(logger.WithContext(ctx)))
		},
		HandleError:  true,
		LogLatency:   true,
		LogMethod:    true,
		LogRoutePath: true,
		LogStatus:    true,
		LogURIPath:   true,
		LogValuesFunc: func(c *echo.Context, values middleware.RequestLoggerValues) error {
			logger := requestLoggerFromContext(c.Request().Context())
			status := responseStatus(c, values.Error)
			event := logger.Info()
			if status >= http.StatusInternalServerError {
				event = logger.Error()
			} else if status >= http.StatusBadRequest {
				event = logger.Warn()
			}
			if values.Error != nil {
				event = event.Err(values.Error)
			}
			if info, ok := requestctx.FromContext(c.Request().Context()); ok && info.UserID != 0 {
				event = event.Int64("user_id", info.UserID)
			}

			event.
				Str("method", values.Method).
				Str("path", requestLogPath(c, values)).
				Int("status", status).
				Dur("latency", values.Latency).
				Msg("HTTP request")

			return nil
		},
	})
}

func requestLoggerFromContext(ctx context.Context) zerolog.Logger {
	ctx = contextWithActiveTrace(ctx)
	info, _ := requestctx.FromContext(ctx)
	builder := logging.FromContext(ctx).With()
	if info.RequestID != "" {
		builder = builder.Str("request_id", info.RequestID)
	}
	if info.TraceID != "" {
		builder = builder.Str("trace_id", info.TraceID)
	}
	if info.SpanID != "" {
		builder = builder.Str("span_id", info.SpanID)
	}
	if info.UserID != 0 {
		builder = builder.Int64("user_id", info.UserID)
	}
	return builder.Logger()
}

func contextWithActiveTrace(ctx context.Context) context.Context {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ctx
	}
	return requestctx.WithTraceSpan(ctx, spanContext.TraceID().String(), spanContext.SpanID().String())
}

func responseStatus(c *echo.Context, err error) int {
	_, status := echo.ResolveResponseStatus(c.Response(), err)
	return status
}

func requestLogPath(c *echo.Context, values middleware.RequestLoggerValues) string {
	if values.RoutePath != "" {
		return values.RoutePath
	}
	if path := c.Path(); path != "" {
		return path
	}
	if values.URIPath != "" {
		return values.URIPath
	}
	if c.Request() == nil || c.Request().URL == nil {
		return ""
	}
	return c.Request().URL.Path
}

func requestPath(c *echo.Context) string {
	if path := c.Path(); path != "" {
		return path
	}
	if c.Request() == nil || c.Request().URL == nil {
		return ""
	}
	return c.Request().URL.Path
}

// MatchRouteExemption reports whether the request matches one exemption by
// exact method and registered route pattern. Route policy forbids path-only,
// prefix, or wildcard exemptions, so matching is always exact. Middleware
// packages and business-owned skippers share this executor so the policy has
// one implementation.
func MatchRouteExemption(c *echo.Context, exemptions []RouteExemption) bool {
	if c == nil || c.Request() == nil {
		return false
	}
	method := c.Request().Method
	path := requestPath(c)
	for _, exemption := range exemptions {
		if exemption.Method == method && exemption.Path == path {
			return true
		}
	}
	return false
}

// normalizedCORSConfig fills install-time CORS defaults. AllowHeaders keeps
// the API-token header readable by browsers on preflight; AllowMethods covers
// the route surface this server registers.
func normalizedCORSConfig(config middleware.CORSConfig) middleware.CORSConfig {
	if len(config.AllowHeaders) == 0 {
		config.AllowHeaders = []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderXCSRFToken,
			APIKeyHeader,
		}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
		}
	}
	return config
}

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	zlog "github.com/rs/zerolog/log"

	"github.com/NSObjects/echo-admin/internal/platform/configs"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/resources"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

const apiPrefix = "/api"

// Runtime tuning below is deliberately not configurable: no deployment has
// ever needed to change it, and inventing config surface nobody uses was the
// problem this package just shed.
const (
	defaultServerPort     = configs.DefaultPort
	defaultReadTimeout    = 30 * time.Second
	defaultWriteTimeout   = 30 * time.Second
	defaultIdleTimeout    = 120 * time.Second
	defaultShutdownPeriod = 10 * time.Second
)

// Server owns the Echo HTTP server lifecycle and system routes.
type Server struct {
	echo                  *echo.Echo
	api                   *echo.Group
	port                  string
	shutdownPeriod        time.Duration
	appConfig             configs.Config
	statusReporter        StatusReporter
	apiKeyVerifier        middlewares.APIKeyVerifier
	errorRecorder         middlewares.SystemErrorRecorder
	sessionAuth           middlewares.LoginSessionAuthenticator
	sessionCookieName     string
	csrfConfig            middleware.CSRFConfig
	installation          middlewares.InstallationStateReader
	preInitRoutes         []middlewares.RouteExemption
	unauthenticatedRoutes []middlewares.RouteExemption
}

type healthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

// StatusReporter supplies readiness and capability status to system routes.
// The status record shape is owned by the infrastructure resources package,
// which also builds the only three legal states.
type StatusReporter interface {
	Status(context.Context) []resources.CapabilityStatus
	Ready(context.Context) error
}

type infoResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

type readinessResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

type capabilitiesResponse struct {
	Capabilities []resources.CapabilityStatus `json:"capabilities"`
	Time         string                       `json:"time"`
}

// Option customizes the HTTP server.
type Option func(*Server)

// WithStatusReporter installs the readiness and capability reporter.
func WithStatusReporter(reporter StatusReporter) Option {
	return func(s *Server) {
		s.statusReporter = reporter
	}
}

// WithAPIKeyVerifier installs optional API token authentication.
func WithAPIKeyVerifier(verifier middlewares.APIKeyVerifier) Option {
	return func(s *Server) {
		s.apiKeyVerifier = verifier
	}
}

// WithSystemErrorRecorder installs optional internal-error recording.
func WithSystemErrorRecorder(recorder middlewares.SystemErrorRecorder) Option {
	return func(s *Server) {
		s.errorRecorder = recorder
	}
}

// WithLoginSessionAuthenticator installs browser login-session authentication.
func WithLoginSessionAuthenticator(authenticator middlewares.LoginSessionAuthenticator) Option {
	return func(s *Server) {
		s.sessionAuth = authenticator
	}
}

// WithLoginSessionCookieName installs the login-session cookie name. The name
// is Login Session domain policy declared by the auth module; the composition
// root injects it so the read and write sides share one declaration.
func WithLoginSessionCookieName(name string) Option {
	return func(s *Server) {
		s.sessionCookieName = name
	}
}

// WithCSRFConfig installs the auth-owned Echo CSRF configuration. CSRF
// activation still derives from login sessions: without an injected config
// the CSRF middleware is not installed.
func WithCSRFConfig(config middleware.CSRFConfig) Option {
	return func(s *Server) {
		s.csrfConfig = config
	}
}

// WithInstallationStateReader installs the uninitialized-system gate.
func WithInstallationStateReader(reader middlewares.InstallationStateReader) Option {
	return func(s *Server) {
		s.installation = reader
	}
}

// WithPreInitRoutes installs routes that stay reachable before first
// initialization completes. The composition root derives them from its route
// exposure declaration.
func WithPreInitRoutes(routes ...middlewares.RouteExemption) Option {
	return func(s *Server) {
		s.preInitRoutes = routes
	}
}

// WithUnauthenticatedRoutes installs routes reachable without a login
// session; CSRF exemptions derive from the same list. The composition root
// derives them from its route exposure declaration.
func WithUnauthenticatedRoutes(routes ...middlewares.RouteExemption) Option {
	return func(s *Server) {
		s.unauthenticatedRoutes = routes
	}
}

// Echo returns the underlying Echo instance for HTTP adapter tests and
// framework-level integration.
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// API returns the root API route group used by boot to register business routes.
func (s *Server) API() *echo.Group {
	return s.api
}

// New creates an Echo-backed HTTP server. cfg must come from configs.Load,
// which owns normalization and validation; New trusts its input.
func New(cfg configs.Config, opts ...Option) (*Server, error) {
	e := echo.New()
	s := &Server{
		echo:           e,
		api:            e.Group(apiPrefix),
		port:           cfg.System.Port,
		shutdownPeriod: defaultShutdownPeriod,
		appConfig:      cfg,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	s.configureEcho()
	if err := s.installMiddleware(); err != nil {
		return nil, err
	}
	s.registerSystemRoutes()

	return s, nil
}

func (s *Server) configureEcho() {
	s.echo.Validator = &middlewares.Validator{Validator: validator.New()}
	s.echo.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(s.errorRecorder)
}

func (s *Server) installMiddleware() error {
	if err := middlewares.ApplyMiddlewares(s.echo, s.middlewareConfig()); err != nil {
		return fmt.Errorf("install middleware: %w", err)
	}
	return nil
}

// middlewareConfig is the single place where "is this dependency injected"
// decides middleware activation: a config pointer exists only when its
// dependency was injected, and a nil pointer leaves the middleware
// uninstalled.
func (s *Server) middlewareConfig() *middlewares.MiddlewareConfig {
	httpConfig := s.appConfig.HTTP
	var apiKey *middlewares.APIKeyConfig
	if s.apiKeyVerifier != nil {
		apiKey = &middlewares.APIKeyConfig{Verifier: s.apiKeyVerifier}
	}
	var installationGate *middlewares.InstallationGateConfig
	if s.installation != nil {
		installationGate = &middlewares.InstallationGateConfig{
			Reader:     s.installation,
			Exemptions: s.preInitRoutes,
		}
	}
	var loginSession *middlewares.LoginSessionConfig
	if s.sessionAuth != nil {
		loginSession = &middlewares.LoginSessionConfig{
			CookieName:    s.sessionCookieName,
			Authenticator: s.sessionAuth,
			Exemptions:    s.unauthenticatedRoutes,
		}
	}

	return &middlewares.MiddlewareConfig{
		EnableRecovery:       httpConfig.EnableRecovery,
		EnableRequestContext: httpConfig.EnableRequestContext,
		EnableLogger:         httpConfig.EnableRequestLog,
		EnableTracing:        s.appConfig.Tracing.Enabled,
		TracingServiceName:   s.appConfig.App.Name,
		EnableGzip:           httpConfig.EnableGzip,
		EnableCORS:           httpConfig.CORS.Enabled,
		CORS:                 corsMiddlewareConfig(httpConfig.CORS),
		APIKey:               apiKey,
		InstallationGate:     installationGate,
		LoginSession:         loginSession,
		CSRF:                 s.csrfConfig,
	}
}

func corsMiddlewareConfig(cfg configs.CORSConfig) middleware.CORSConfig {
	return middleware.CORSConfig{
		AllowOrigins:     cfg.AllowOrigins,
		AllowMethods:     cfg.AllowMethods,
		AllowHeaders:     cfg.AllowHeaders,
		AllowCredentials: cfg.AllowCredentials,
		ExposeHeaders:    cfg.ExposeHeaders,
		MaxAge:           cfg.MaxAgeSeconds,
	}
}

func (s *Server) registerSystemRoutes() {
	// Health and readiness also serve HEAD because liveness and readiness
	// probes commonly issue HEAD requests and Echo does not fall back HEAD to
	// GET routes. Response bodies are dropped by net/http for HEAD anyway.
	health := func(c *echo.Context) error {
		return c.JSON(http.StatusOK, healthResponse{
			Status: "ok",
			Time:   time.Now().Format(time.RFC3339),
		})
	}
	s.api.GET("/health", health)
	s.api.HEAD("/health", health)

	s.api.GET("/info", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, infoResponse{
			Name:    s.appConfig.App.Name,
			Version: s.appConfig.App.Version,
			Time:    time.Now().Format(time.RFC3339),
		})
	})

	ready := func(c *echo.Context) error {
		response := readinessResponse{
			Status: "ready",
			Time:   time.Now().Format(time.RFC3339),
		}
		if s.statusReporter == nil {
			return c.JSON(http.StatusOK, response)
		}
		if err := s.statusReporter.Ready(c.Request().Context()); err != nil {
			response.Status = "unavailable"
			return c.JSON(http.StatusServiceUnavailable, response)
		}
		return c.JSON(http.StatusOK, response)
	}
	s.api.GET("/ready", ready)
	s.api.HEAD("/ready", ready)

	s.api.GET("/capabilities", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, capabilitiesResponse{
			Capabilities: s.statuses(c.Request().Context()),
			Time:         time.Now().Format(time.RFC3339),
		})
	})
}

func (s *Server) statuses(ctx context.Context) []resources.CapabilityStatus {
	if s.statusReporter == nil {
		return nil
	}
	statuses := s.statusReporter.Status(ctx)
	copied := make([]resources.CapabilityStatus, len(statuses))
	copy(copied, statuses)
	return copied
}

// Run starts the HTTP server and blocks until ctx is canceled or startup fails.
func (s *Server) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("server run: nil context")
	}

	addr := s.port
	if addr == "" {
		addr = defaultServerPort
	}

	zlog.Info().Str("addr", addr).Msg("starting server")
	shutdownErrCh := make(chan error, 1)
	startConfig := s.startConfig(addr)
	startConfig.OnShutdownError = func(err error) {
		select {
		case shutdownErrCh <- err:
		default:
		}
	}

	if err := startConfig.Start(ctx, s.echo); err != nil {
		return err
	}
	select {
	case err := <-shutdownErrCh:
		return fmt.Errorf("shutdown server: %w", err)
	default:
	}

	zlog.Info().Msg("server exited")
	return nil
}

func (s *Server) startConfig(addr string) echo.StartConfig {
	return echo.StartConfig{
		Address:         addr,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: s.shutdownPeriod,
		BeforeServeFunc: func(server *http.Server) error {
			server.ReadTimeout = defaultReadTimeout
			server.WriteTimeout = defaultWriteTimeout
			server.IdleTimeout = defaultIdleTimeout
			return nil
		},
	}
}

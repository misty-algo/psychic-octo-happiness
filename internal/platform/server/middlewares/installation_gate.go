package middlewares

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// InstallationStateReader reports whether first initialization has completed.
type InstallationStateReader interface {
	Initialized(context.Context) (bool, error)
}

// InstallationGateConfig controls the uninitialized-system route gate. A nil
// config in MiddlewareConfig leaves the gate uninstalled; a present config
// must carry a Reader.
type InstallationGateConfig struct {
	Reader     InstallationStateReader
	Exemptions []RouteExemption
}

// installationGate blocks normal administration routes until setup completes.
// Exemptions are injected by the composition root and matched by exact method
// and registered pattern, so this layer carries no route policy of its own.
func installationGate(config InstallationGateConfig) (echo.MiddlewareFunc, error) {
	if config.Reader == nil {
		return nil, errors.New("installation state reader is required when the installation gate is installed")
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Request().Method == http.MethodOptions || MatchRouteExemption(c, config.Exemptions) {
				return next(c)
			}
			initialized, err := config.Reader.Initialized(c.Request().Context())
			if err != nil {
				return err
			}
			if !initialized {
				return apperr.New(apperr.ErrSystemUninitialized, "")
			}
			return next(c)
		}
	}, nil
}

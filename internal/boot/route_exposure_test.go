package boot

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

// registerExemptAPIRoutes mounts every declared exemption so finalize can
// verify the declaration against the real router.
func registerExemptAPIRoutes(t *testing.T, e *echo.Echo) {
	t.Helper()
	handler := func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}
	for _, route := range exemptAPIRoutes {
		e.Add(route.method, route.pattern, handler)
	}
}

func TestFinalizeRouteExposureAllowsSystemAndBootstrapRoutesWithoutAuthorizer(t *testing.T) {
	e := echo.New()
	registerExemptAPIRoutes(t, e)
	e.Add(http.MethodOptions, "/api/things", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	manifest, err := finalizeRouteExposure(e, nil)
	if err != nil {
		t.Fatalf("finalizeRouteExposure() error = %v, want nil", err)
	}
	if got := len(manifest.managed); got != 0 {
		t.Fatalf("managed route count = %d, want 0", got)
	}
}

func TestCheckExemptAPIRoutes(t *testing.T) {
	t.Run("full registration passes", func(t *testing.T) {
		e := echo.New()
		registerExemptAPIRoutes(t, e)

		if err := checkExemptAPIRoutes(e); err != nil {
			t.Fatalf("checkExemptAPIRoutes() error = %v, want nil", err)
		}
	})
	t.Run("partial registration fails", func(t *testing.T) {
		e := echo.New()
		e.Add(http.MethodGet, "/api/health", func(c *echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})

		err := checkExemptAPIRoutes(e)
		if !errors.Is(err, errRouteExposureMismatch) {
			t.Fatalf("checkExemptAPIRoutes() error = %v, want %v", err, errRouteExposureMismatch)
		}
	})
}

func TestRouteExemptionDerivations(t *testing.T) {
	preInit := preInitRouteExemptions()
	wantPreInit := []middlewares.RouteExemption{
		{Method: http.MethodGet, Path: "/api/health"},
		{Method: http.MethodHead, Path: "/api/health"},
		{Method: http.MethodGet, Path: "/api/info"},
		{Method: http.MethodGet, Path: "/api/setup/state"},
		{Method: http.MethodPost, Path: "/api/setup"},
	}
	if !reflect.DeepEqual(preInit, wantPreInit) {
		t.Fatalf("preInitRouteExemptions() = %v, want %v", preInit, wantPreInit)
	}

	unauth := unauthenticatedRouteExemptions()
	if len(unauth) != len(exemptAPIRoutes) {
		t.Fatalf("unauthenticatedRouteExemptions() length = %d, want %d", len(unauth), len(exemptAPIRoutes))
	}
	unauthSet := make(map[middlewares.RouteExemption]struct{}, len(unauth))
	for _, exemption := range unauth {
		unauthSet[exemption] = struct{}{}
	}
	for _, exemption := range preInit {
		if _, ok := unauthSet[exemption]; !ok {
			t.Fatalf("pre-init exemption %v missing from unauthenticated exemptions", exemption)
		}
	}
}

func TestFinalizeRouteExposureTreatsExemptionVariantsAsManaged(t *testing.T) {
	tests := []apiRouteKey{
		{method: http.MethodGet, pattern: "/api/auth/login"},
		{method: http.MethodPost, pattern: "/api/auth/login/:suffix"},
		{method: http.MethodPost, pattern: "/api/setup/state"},
	}
	for _, route := range tests {
		t.Run(route.String(), func(t *testing.T) {
			e := echo.New()
			e.Add(route.method, route.pattern, func(c *echo.Context) error {
				return c.NoContent(http.StatusNoContent)
			})

			_, err := finalizeRouteExposure(e, nil)
			if !errors.Is(err, errRouteAuthorizationRequired) {
				t.Fatalf("finalizeRouteExposure() error = %v, want %v", err, errRouteAuthorizationRequired)
			}
		})
	}
}

func TestFinalizeRouteExposureLeavesOptionsAndUnmatchedRequestsOutsideAuthorization(t *testing.T) {
	e := echo.New()
	e.GET("/api/things/:id", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	authorizer := &testRouteAuthorizer{}
	if _, err := finalizeRouteExposure(e, authorizer); err != nil {
		t.Fatalf("finalizeRouteExposure() error = %v", err)
	}

	for _, tt := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "options", method: http.MethodOptions, path: "/api/things/42"},
		{name: "not found", method: http.MethodGet, path: "/api/missing"},
		{name: "method not allowed", method: http.MethodDelete, path: "/api/things/42"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			e.ServeHTTP(recorder, request)
			if got := len(authorizer.calls); got != 0 {
				t.Fatalf("authorizer calls = %d, want 0", got)
			}
		})
	}
}

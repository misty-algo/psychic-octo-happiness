package boot

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	accessdomain "github.com/NSObjects/echo-admin/internal/modules/access/domain"
	accesshttp "github.com/NSObjects/echo-admin/internal/modules/access/http"
	apitokenhttp "github.com/NSObjects/echo-admin/internal/modules/apitoken/http"
	audithttp "github.com/NSObjects/echo-admin/internal/modules/audit/http"
	authhttp "github.com/NSObjects/echo-admin/internal/modules/auth/http"
	filehttp "github.com/NSObjects/echo-admin/internal/modules/fileasset/http"
	identityhttp "github.com/NSObjects/echo-admin/internal/modules/identity/http"
	settingshttp "github.com/NSObjects/echo-admin/internal/modules/settings/http"
	setuphttp "github.com/NSObjects/echo-admin/internal/modules/setup/http"
	"github.com/NSObjects/echo-admin/internal/platform/configs"
	"github.com/NSObjects/echo-admin/internal/platform/server"
)

func TestRouteManifestCheckManagedCatalog(t *testing.T) {
	manifest := thingRouteManifest(t)
	exact := accessdomain.ManagedAPIRouteDefinition{
		Method:      http.MethodGet,
		Pattern:     "/api/things/:id",
		Description: "Thing detail",
		Group:       "thing",
	}
	for _, tt := range routeManifestCoverageCases(exact) {
		t.Run(tt.name, func(t *testing.T) {
			checkErr := manifest.CheckManagedCatalog(tt.definitions)
			if len(tt.wantParts) == 0 {
				if checkErr != nil {
					t.Fatalf("CheckManagedCatalog() error = %v, want nil", checkErr)
				}
				return
			}
			if !errors.Is(checkErr, errManagedAPIRouteCoverage) {
				t.Fatalf("CheckManagedCatalog() error = %v, want %v", checkErr, errManagedAPIRouteCoverage)
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(checkErr.Error(), part) {
					t.Fatalf("CheckManagedCatalog() error = %q, want part %q", checkErr, part)
				}
			}
		})
	}
}

func thingRouteManifest(t *testing.T) routeManifest {
	t.Helper()
	e := echo.New()
	e.GET("/api/things/:id", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	manifest, err := finalizeRouteExposure(e, &testRouteAuthorizer{})
	if err != nil {
		t.Fatalf("finalizeRouteExposure() error = %v", err)
	}
	return manifest
}

func routeManifestCoverageCases(exact accessdomain.ManagedAPIRouteDefinition) []struct {
	name        string
	definitions []accessdomain.ManagedAPIRouteDefinition
	wantParts   []string
} {
	return []struct {
		name        string
		definitions []accessdomain.ManagedAPIRouteDefinition
		wantParts   []string
	}{
		{name: "exact coverage", definitions: []accessdomain.ManagedAPIRouteDefinition{exact}},
		{name: "missing definition", wantParts: []string{"missing_from_catalog=[GET /api/things/:id]"}},
		{
			name: "stale definition",
			definitions: []accessdomain.ManagedAPIRouteDefinition{
				exact,
				{Method: http.MethodPost, Pattern: "/api/stale", Description: "Stale", Group: "thing"},
			},
			wantParts: []string{"stale_in_catalog=[POST /api/stale]"},
		},
		{
			name: "method mismatch",
			definitions: []accessdomain.ManagedAPIRouteDefinition{
				{Method: http.MethodPost, Pattern: "/api/things/:id", Description: "Thing detail", Group: "thing"},
			},
			wantParts: []string{
				"missing_from_catalog=[GET /api/things/:id]",
				"stale_in_catalog=[POST /api/things/:id]",
			},
		},
		{
			name:        "duplicate definition",
			definitions: []accessdomain.ManagedAPIRouteDefinition{exact, exact},
			wantParts:   []string{"duplicate=[GET /api/things/:id]"},
		},
		{
			name: "exempt definition",
			definitions: []accessdomain.ManagedAPIRouteDefinition{
				exact,
				{Method: http.MethodGet, Pattern: "/api/health", Description: "Health", Group: "system"},
			},
			wantParts: []string{"wrongly_classified=[GET /api/health]"},
		},
	}
}

func TestBusinessRouteManifestMatchesManagedAPIRouteCatalog(t *testing.T) {
	srv, err := server.New(configs.Config{})
	if err != nil {
		t.Fatalf("server.New() error = %v", err)
	}
	injector := do.New()
	do.ProvideValue(injector, &setuphttp.Handler{})
	do.ProvideValue(injector, &accesshttp.Handler{})
	do.ProvideValue(injector, &identityhttp.Handler{})
	do.ProvideValue(injector, &audithttp.Handler{})
	do.ProvideValue(injector, &apitokenhttp.Handler{})
	do.ProvideValue(injector, &authhttp.Handler{})
	do.ProvideValue(injector, &settingshttp.Handler{})
	do.ProvideValue(injector, &filehttp.Handler{})

	if mountErr := mountModules(Context{
		Server:    srv,
		Router:    srv.API(),
		Container: injector,
	}, BusinessModules()); mountErr != nil {
		t.Fatalf("mountModules() error = %v", mountErr)
	}
	manifest, err := finalizeRouteExposure(srv.Echo(), &testRouteAuthorizer{})
	if err != nil {
		t.Fatalf("finalizeRouteExposure() error = %v", err)
	}
	if err := checkExemptAPIRoutes(srv.Echo()); err != nil {
		t.Fatalf("checkExemptAPIRoutes() error = %v", err)
	}
	if err := manifest.CheckManagedCatalog(accessdomain.ManagedAPIRouteCatalog()); err != nil {
		t.Fatalf("CheckManagedCatalog() error = %v", err)
	}
}

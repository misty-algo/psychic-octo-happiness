package boot

import (
	"context"

	"github.com/samber/do/v2"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
)

type routeAuthorizer interface {
	AuthorizeRoute(context.Context, string, string) error
}

func resolveRouteAuthorizer(injector do.Injector) (routeAuthorizer, error) {
	authorizer, err := do.InvokeAs[routeAuthorizer](injector)
	if err != nil {
		if optionalServiceMissing(err) {
			return nil, nil
		}
		return nil, err
	}
	return authorizer, nil
}

type routeAuthorizerAdapter struct {
	access *accessusecase.Usecase
}

type authAuthorizationReader struct {
	access *accessusecase.Usecase
}

func newRouteAuthorizer(i do.Injector) (routeAuthorizer, error) {
	access, err := do.Invoke[*accessusecase.Usecase](i)
	if err != nil {
		return nil, err
	}
	return routeAuthorizerAdapter{access: access}, nil
}

func (a routeAuthorizerAdapter) AuthorizeRoute(ctx context.Context, method, path string) error {
	if a.access == nil {
		return apperr.New(apperr.ErrInternalServer, "route authorizer is not configured")
	}
	adminID, err := requestctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	roleID, err := requestctx.RequireRoleID(ctx)
	if err != nil {
		return err
	}
	return a.access.AuthorizeRoute(ctx, accessusecase.AuthorizationSubject{
		AdminID:      adminID,
		ActiveRoleID: roleID,
	}, method, path)
}

func (r authAuthorizationReader) CurrentAuthorization(ctx context.Context, subject accessusecase.AuthorizationSubject) (accessusecase.AuthorizationView, error) {
	return r.access.CurrentAuthorization(ctx, subject)
}

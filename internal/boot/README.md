# Boot

`internal/boot` is the composition root. It owns process startup, configured infrastructure resources, `samber/do` dependency injection, and business route mounting.

Business modules declare what they provide and what they mount:

```go
func accessModule() Module {
	return NewModule("access",
		Provide(newAccessStore),
		Provide(newAccessUsecase),
		Provide(newAccessHandler),
		Route(accesshttp.Register),
	)
}
```

Provider functions are ordinary `do.Provider[T]` functions. Runtime business storage uses the configured MySQL resource:

```go
func newAccessStore(i do.Injector) (*accessmysql.Store, error) {
	ctx, db, err := startupMySQL(i)
	if err != nil {
		return nil, err
	}
	return accessmysql.NewStore(ctx, db)
}
```

Business code lives under `internal/modules/<module>`. Platform runtime code lives under `internal/platform`. Boot is allowed to import adapters, infrastructure, server, and configs so usecase and domain packages stay clean.

Cross-module wiring belongs here. For example, auth defines the small authorization reader, recorder, and login-session interfaces it needs; boot bridges its authorization reader to the access usecase, passes the concrete identity/auth MySQL stores, and bridges login records into the audit usecase. Access also receives a small identity reader so role delegation and runtime authorization can use current administrator state without importing the identity adapter directly. Settings receives a version-catalog bridge for access-owned menus only; Managed API Route definitions cannot travel through version bundles. API token authentication is wired here through a small server-facing verifier adapter, so `internal/platform/server` can accept `X-API-Token` without importing token storage. Browser Login Session authentication is wired the same way. System First Initialization coordinates one transaction across setup, access, identity, and settings stores to create the root authorization baseline, first administrator, initial settings, and completion marker.

Access owns Casbin RBAC, current authorization views, and exact Managed API Route decisions. Boot first mounts every module, then finalizes route exposure from Echo's registered route table and installs one global middleware. Exact System API Route and Bootstrap API Route identities are exempt; every other registered `/api` route is Managed and must pass Route Authorization with its registered method/pattern. A runtime with Managed routes cannot assemble without a route authorizer. The returned manifest is compared with `access`'s deployment-owned catalog in a DB-free contract test, while runtime authorization reads the persisted catalog and current administrator and role state. System First Initialization persists only Managed definitions and gives the root role complete explicit grants; catalog baseline changes require explicit access-owned upgrade work, never startup repair or administrator CRUD.

# Echo Admin

Echo Admin provides browser-based administration for operators and token-based access for non-browser API clients.

## Language

**Administrator**:
A human operator who signs in to the administration console.
_Avoid_: User, account

**Login Session**:
A single browser/device login state for one **Administrator**.
_Avoid_: Admin Session, JWT session, global login

**API Token**:
A credential issued for non-browser API clients acting as an administrator role.
_Avoid_: Login Session, login token

**Session Revocation**:
The act of making one or more **Login Sessions** no longer valid.
_Avoid_: Token blacklist

**CSRF Token**:
A browser-readable request token required for state-changing browser API calls.
_Avoid_: Login token, session token

**Security Event**:
An administrator account change that requires **Session Revocation**.
_Avoid_: Normal logout

**System First Initialization**:
The one-time setup that makes a new Echo Admin installation usable for its first **Administrator**.
_Avoid_: Process startup, migration, seed

**Installation State**:
The persistent state that says whether **System First Initialization** has completed for an Echo Admin installation.
_Avoid_: Admin existence check, table existence check

**Root Role**:
The highest-authority administrator role created during **System First Initialization**.
_Avoid_: Custom setup role, normal role

**Administration Authorization**:
The role-based authorization model that controls what an **Administrator** may see and do in Echo Admin's administration surface.
_Avoid_: General business permission system, generic data authorization

**System API Route**:
A registered operational route for process and capability status that is outside **Administration Authorization**.
_Avoid_: Managed API Route, Bootstrap API Route

**Bootstrap API Route**:
A registered administration entry route for probing or establishing **Installation State** or creating a **Login Session** before managed route authorization can apply.
_Avoid_: Public route, System API Route

**Managed API Route**:
A registered administration business route represented by HTTP method and Echo route pattern in the **Managed API Route Catalog**.
_Avoid_: Raw URL, System API Route, Bootstrap API Route

**Managed API Route Catalog**:
The authorization catalog for administration business API routes.
_Avoid_: System route list, bootstrap route list, every HTTP endpoint

## Relationships

- An **Administrator** may have zero or more active **Login Sessions**.
- A **Login Session** belongs to exactly one **Administrator**.
- A **Login Session** may hold the currently active role for that browser login.
- A **Login Session** does not contain a permission snapshot; authorization uses the current administrator and role state.
- A **Login Session** uses an HttpOnly cookie credential; browser JavaScript must not read the session token.
- A **CSRF Token** is not a **Login Session** and cannot authenticate a request by itself.
- An **API Token** is not a **Login Session**.
- Browser administration routes authenticate only with **Login Session** cookies.
- Machine API routes authenticate only with **API Tokens**.
- Browser login responses do not expose **Login Session** credentials in response bodies.
- Browser clients recover the current administrator view from the active **Login Session**, not from locally stored credentials.
- A **Security Event** may revoke all **Login Sessions** for one **Administrator**.
- Disabling, deleting, resetting the password for, or explicitly signing out an **Administrator** from all devices is a **Security Event**.
- When an **Administrator** changes their own password, other **Login Sessions** are revoked while the current **Login Session** remains active.
- Role and permission changes are not **Security Events** because authorization uses current administrator and role state.
- A normal logout revokes only one **Login Session**.
- Signing out from other devices revokes the other **Login Sessions** for the same **Administrator** while keeping the current **Login Session** active.
- An unavailable **Login Session** is an authentication failure; a valid **Login Session** without route permission is an authorization failure.
- During **Route Authorization**, a missing or inactive active role is an authorization failure, not a not-found response.
- **System First Initialization** happens before the first **Administrator** can use the administration console.
- **System First Initialization** creates the first **Administrator** with the **Root Role**.
- **Installation State** is the source of truth for whether **System First Initialization** is still allowed.
- **System First Initialization** may be retried until **Installation State** records completion.
- **Administration Authorization** governs administration features, not arbitrary business-domain data ownership.
- Every registered `/api` route is exactly one **System API Route**, **Bootstrap API Route**, or **Managed API Route**.
- Health, info, readiness, and capability routes are **System API Routes**.
- Setup state, setup submission, and login routes are **Bootstrap API Routes**.
- The **Managed API Route Catalog** contains **Managed API Routes** only.
- The identity and metadata of every **Managed API Route** are deployment-owned and cannot be created, changed, or deleted by an **Administrator** at runtime.
- An **Administrator** may inspect the **Managed API Route Catalog** and manage role grants for its routes.
- New, changed, and retired **Managed API Routes** enter runtime catalog state through explicit access-owned authorization upgrade work.
- Boot identifies **System API Routes** and **Bootstrap API Routes** by exact HTTP method and registered Echo route pattern; every other registered `/api` route is a **Managed API Route**.
- Route exemptions never use path-only, prefix, or wildcard matching.
- `OPTIONS` preflight and unmatched requests are not registered API routes and are outside this classification.
- Route classification does not determine pre-initialization reachability; **Installation State** rules decide that separately.
- Route exposure policy belongs to the composition root, so boot owns which system and bootstrap routes are outside the **Managed API Route Catalog**.
- **System API Route** and **Bootstrap API Route** identities are declared once in the composition root; middleware exemptions are derived from that single declaration.
- At test time, registered **Managed API Routes** and the access-owned catalog definition must match exactly; missing, stale, duplicate, or wrongly classified routes are contract failures.
- A runtime with any registered **Managed API Route** must not complete assembly without **Route Authorization**; a runtime containing only **System API Routes** and **Bootstrap API Routes** does not require it.
- Runtime **Route Authorization** must reject non-exempt routes that have no matching **Managed API Route** entry.
- Process startup must not require a populated database route catalog because **System First Initialization** creates the administration baseline after startup.
- The **Managed API Route Catalog** data belongs to Administration Authorization, so access owns the catalog content.
- Boot owns route exposure policy and catalog coverage checks, but boot must not become the author of authorization catalog data.
- **Route Authorization** happens in boot middleware before business HTTP handlers run.
- **Route Authorization** uses the current active role and **Managed API Route** grants, not handler-declared permission tokens.
- Permission tokens remain **Administration Authorization** metadata for Casbin permission views, menus, buttons, and grant catalogs.
- The **Root Role** has the complete **Administration Authorization** baseline.

## Example Dialogue

> **Dev:** "When an **Administrator** signs in on a second browser, should it replace the first login?"
> **Domain expert:** "No. Create another **Login Session**. Only explicit security events should revoke all sessions for that administrator."

## Flagged Ambiguities

- "session" was used to mean both browser login state and API credentials; resolved: browser login state is **Login Session**, while machine credentials are **API Token**.
- "permission system" was used to mean both administration feature authorization and generic business-data authorization; resolved: the current target is **Administration Authorization** only.

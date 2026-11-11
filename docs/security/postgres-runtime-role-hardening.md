# PostgreSQL runtime role hardening

## Status

This document records the verified production finding and the safe implementation sequence. It is **not** a production credential or privilege change.

Production Render deployment `65eacc43e3885fdf4373e23dfc43da449b351b26` reported:

- `current_user = postgres`
- `session_user = postgres`
- `rolsuper = false`
- `rolbypassrls = true`

The connection therefore bypasses PostgreSQL row-level security. This is a defense-in-depth gap: the Go API currently relies on its authentication, role/permission middleware, tenant context, and school-scoped query logic. RLS being enabled on tables does not constrain a connection whose effective role has `BYPASSRLS`.

## Verified database observations

The live project was inspected read-only:

- All 42 public tables inspected have RLS enabled.
- The tables are owned by `postgres`; `FORCE ROW LEVEL SECURITY` is false.
- The application connection is `postgres`, which has `BYPASSRLS`.
- `anon` and `authenticated` do not have `BYPASSRLS`.
- Existing `deny_client_access` policies explicitly reject `anon` and `authenticated` on the 26 public tables to which those roles currently have table privileges.
- Default privileges for objects created by `postgres` in `public` grant broad table privileges to `anon`, `authenticated`, and `service_role`. This must be tightened for future objects, with explicit per-object grants where needed.
- These findings do not establish that tenant data has been accessed by an unauthorized party. No evidence of such access was collected.

## Why changing the Render DB username alone is unsafe

The server currently calls `database.Migrate(db)` and `bootstrap.EnsureInitialAdmin(db)` during application startup. The migration path performs schema changes and data backfills. A least-privilege runtime role must not inherit those administrative capabilities.

The server also shares a GORM database handle across middleware, repositories, services, and controllers. `RequireSchoolContext` loads the authenticated user's school ID and stores it in Gin context, but this does not automatically set a transaction-local PostgreSQL tenant variable for every query. Setting a session variable on a pooled connection is unsafe because a later request may reuse that connection.

Consequently, neither of these is an acceptable immediate production fix:

- Replacing the production connection string with an arbitrary non-owner role before testing the required grants and RLS policies.
- Adding permissive tenant policies that depend on a session variable without ensuring the variable is set transaction-locally for every request and every query path.

## Target architecture

Separate privileged deployment/migration work from ordinary request handling:

1. **Migration identity** — a separately managed credential used only by a controlled migration job. It owns or can alter schema objects and is never used by the HTTP server.
2. **Runtime identity** — a dedicated login role with `NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE`, no ownership of application tables, and only the minimum table/sequence/function privileges required by the API.
3. **Request-scoped tenant context** — derive school ID from the authenticated database user record, never from a client-supplied school ID. Begin a database transaction for a protected request, set the tenant identifier with `set_config('app.current_school_id', ..., true)` (transaction-local), and ensure all handler/repository queries use that transaction. Roll back/commit at the request boundary.
4. **RLS policies** — create explicit policies for the runtime role. Tenant-owned tables must compare their `school_id` with the transaction-local tenant context and fail closed when it is missing or invalid. Global/platform tables need separately designed policies; do not apply a blanket school policy to them.
5. **Administrative routes and startup** — identify platform-wide operations and startup bootstrap/migration paths that legitimately need cross-tenant access. Move schema migrations to a one-shot deployment step and isolate narrowly scoped administrative operations rather than granting the HTTP runtime role `BYPASSRLS`.
6. **Pool safety** — test transaction-local context with concurrent requests and connection reuse. Never use a persistent session-level `SET` for tenant context.

## Required implementation sequence

### Phase A — inventory and design (no production writes)

- Map every table into tenant-owned, global reference/catalogue, platform administration, or operational/security categories.
- Inventory all direct GORM calls and identify which handlers run without `RequireSchoolContext`.
- Trace platform super-admin operations and public endpoints separately.
- Specify the exact runtime grants, sequence access, function execution, and RLS policy for each table.
- Review existing default privileges and all current client-role grants.

### Phase B — isolated PostgreSQL proof

- Create a disposable database or isolated test schema using a privileged test connection.
- Create a non-owner `NOBYPASSRLS` runtime role with no membership in privileged roles.
- Test that no tenant context returns zero rows / rejects writes.
- Test School A cannot read, update, insert, or delete School B rows, including joins and child tables.
- Test valid same-school reads and writes for all seven application roles.
- Test global curriculum/catalogue access and super-admin-only platform operations.
- Test concurrent requests and connection-pool reuse to prove tenant context cannot leak between requests.
- Verify migration and bootstrap tasks work through the separate migration identity and are unavailable to the runtime role.

### Phase C — application integration

- Introduce a request-scoped transaction abstraction and ensure all protected handler queries use it.
- Make transaction setup fail closed if school context is absent or invalid.
- Keep login/session validation and platform-wide routes on explicit, separately reviewed paths.
- Add integration tests for every role and cross-school access.
- Add CI checks that fail if a new public table lacks RLS, an explicit access policy, or an intentional documented exception.

### Phase D — controlled production rollout (requires separate approval)

- Take/verify a restorable backup.
- Apply reviewed grants, policies, and migration-role separation in a maintenance window.
- Deploy the application configured with the runtime identity.
- Verify startup diagnostic reports `current_bypass_rls=false` and `session_bypass_rls=false`.
- Run read-only health checks and approved role/tenant smoke tests.
- Keep a tested rollback path to the prior deployment and connection configuration.

## Acceptance criteria

Do not call this remediation complete until all are true:

- The HTTP service connects as the dedicated non-owner, non-superuser, `NOBYPASSRLS` role.
- The runtime role has no inherited or settable membership in privileged roles.
- Migrations and bootstrap use a separate privileged path that is not present in the web runtime.
- Every tenant-owned table has a fail-closed tenant RLS policy for the runtime role.
- Global/platform tables have explicit, tested access rules.
- Cross-tenant tests pass for SELECT, INSERT, UPDATE, and DELETE.
- Concurrent pooled-request tests prove tenant context isolation.
- Default privileges no longer automatically grant broad access to `anon` and `authenticated` on future public tables.
- Backup restoration and production smoke checks pass.

## Scope of this change

This plan intentionally makes no changes to production database roles, grants, policies, secrets, connection strings, or records. The current startup diagnostic remains the verification mechanism until the runtime-role migration is implemented and separately approved.

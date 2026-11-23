# Authentication Bootstrap Runtime Integration

This change connects request authentication to the narrow database capability defined separately in PR #372.

## Runtime flow

1. The middleware validates the JWT signature and rejects refresh tokens.
2. It parses the JWT session ID and user ID as UUIDs.
3. The PostgreSQL adapter calls only `public.authenticate_access_session(session_id, user_id)` through a dedicated GORM connection pool.
4. The capability returns the live user ID, current role, and school ID. The JWT role is not used for authorization.
5. Middleware stores those three values in Gin request context.
6. `RequireSchoolContext` consumes the authenticated school ID from request context; it no longer performs a second users-table query.

An invalid/revoked/expired session returns HTTP 401. An unavailable bootstrap database or function returns HTTP 503. Both paths abort the request before protected handlers run.

## Connection configuration

PostgreSQL deployments must configure `AUTH_BOOTSTRAP_DATABASE_URL` separately from `DATABASE_URL` and `MIGRATION_DATABASE_URL`.

The credential should:
- be a dedicated login role;
- have schema USAGE and EXECUTE only on `public.authenticate_access_session(uuid, uuid)`;
- have no direct SELECT access to `users` or `sessions`;
- be NOSUPERUSER, NOBYPASSRLS, NOCREATEDB, NOCREATEROLE, and NOREPLICATION;
- not be a member of the function-owner or migration-owner role.

The application opens a separate bounded connection pool, pings it, and executes a zero-UUID lookup at startup to verify the function is callable. It also verifies the connected role is non-superuser and NOBYPASSRLS, has no privileged-role membership, has schema USAGE but not CREATE, can execute the capability, has no direct privileges on application tables, views, or sequences in non-system schemas, and is not a member of the function-owner role. The probe is read-only and is expected to return no row. Startup fails closed if any invariant is not met.

Local non-PostgreSQL development retains a legacy GORM adapter. That adapter explicitly refuses PostgreSQL connections and cannot be used as a production fallback.

## Scope and rollout boundary

- No SQL migration is applied by this change.
- The function definition and its actual PostgreSQL/RLS tests remain in PR #372; this runtime PR depends on that capability being reviewed and merged first.
- No roles, credentials, grants, secrets, database settings, or production services are created or changed here.
- Do not configure or deploy `AUTH_BOOTSTRAP_DATABASE_URL` until the function capability, role grants, staging catalog, and rollback plan are independently verified.
- This change does not authorize enabling `REQUIRE_NOBYPASSRLS`, tenant policy cutover, or production deployment.

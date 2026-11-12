# Restricted API role and tenant-aware RLS rollout

## Current verified posture

- The API currently connects as PostgreSQL `postgres`, which has `BYPASSRLS`.
- All 45 public application tables have RLS enabled, but RLS is not forced.
- The current `deny_client_access` policies target only `anon` and `authenticated`; they do not provide tenant isolation for a direct PostgreSQL API login.
- Direct Data API table privileges for `anon` and `authenticated` have been revoked.
- The API uses custom HMAC JWT authentication. It does not currently use Supabase Auth JWT claims as the source of tenant context.
- The Gin middleware validates a user and school context, but the application constructs repositories and services around a shared GORM database handle. A school ID stored in Gin context is not automatically attached to every SQL statement or guaranteed to use the same pooled connection.

## Why the runtime role must not be switched yet

A role with `NOBYPASSRLS` will be subject to applicable RLS policies. Simply changing the Render connection string to such a role before policies and request transaction context exist can cause legitimate reads/writes to return no rows or fail. Conversely, setting a session-level custom GUC on a pooled connection is unsafe because a later request may reuse that connection with stale tenant state.

## Required implementation sequence

1. **Request-scoped database transaction**
   - Establish one GORM transaction for each authenticated request that needs tenant data.
   - Set tenant/user context with parameterized `set_config(..., true)` (transaction-local) inside that transaction.
   - Ensure every repository/service query for that request uses the same transaction handle.
   - Roll back on request failure and always commit/close at request completion.
   - Do not use session-level `SET` or a pooled-connection hook that can leak context between requests.
2. **Tenant policy inventory**
   - Classify each table as school-scoped, user-scoped, platform-global, public-read, or migration/system-only.
   - Include child tables whose ownership is derived through parent rows; verify tenant consistency with composite foreign keys or policy joins.
   - Handle `users`, `sessions`, refresh/reset tokens, billing, public school pages, background jobs, and super-admin operations explicitly.
3. **Policies and grants**
   - Add least-privilege DML grants to a dedicated API role that does not own application tables and has `NOBYPASSRLS`.
   - Keep DDL ownership and migration credentials separate; do not grant the API role membership in the migration/owner role.
   - Add fail-closed policies for every tenant table using transaction-local context, with both `USING` and `WITH CHECK` for writes.
   - Add narrowly scoped rules for platform-global and public endpoints; never treat a missing tenant context as unrestricted access.
   - Review views and functions separately because their security mode can affect RLS behavior.
4. **Isolation tests**
   - Run PostgreSQL integration tests with two schools and the exact restricted API role.
   - Prove cross-school SELECT, UPDATE, DELETE, and INSERT are denied; same-school operations succeed.
   - Test no-context requests, context reset between pooled connections, transaction rollback, user-specific records, background jobs, and platform-admin workflows.
5. **Controlled cutover**
   - Provision roles and grants only after the tests pass.
   - Set the Render API URL to the restricted role and enable `REQUIRE_NOBYPASSRLS=true` in the same approved cutover.
   - Keep the migration URL separate and verify startup diagnostics and production smoke tests.
   - Have a tested rollback connection ready before changing production credentials.

## Startup guard

`REQUIRE_NOBYPASSRLS=true` makes startup fail if PostgreSQL role inspection fails or either the effective role or authenticated session role is superuser/`BYPASSRLS`. It is a deployment guard only; it does not create roles, grants, or RLS policies.

Keep the flag disabled until steps 1–4 are implemented and validated in an isolated PostgreSQL environment. Do not apply production DDL or change Render secrets as part of this preparation work.

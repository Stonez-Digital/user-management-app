# Restricted API role and tenant-aware RLS rollout

## Current verified posture

- The API currently connects as PostgreSQL postgres, which has BYPASSRLS.
- All 45 public application tables have RLS enabled, but RLS is not forced.
- Current deny_client_access policies target only anon and authenticated; they do not provide tenant isolation for a direct PostgreSQL API login.
- Direct Data API table privileges for anon and authenticated have been revoked.
- The API uses custom HMAC JWT authentication, not Supabase Auth JWT claims, as the source of tenant context.
- Gin validates a user and school context, but controllers and services currently hold a shared GORM database handle. A school ID in Gin context is not automatically attached to every SQL statement or guaranteed to use the same pooled connection.

## Request-scoped transaction foundation

The request transaction middleware is integrated in a staged manner:

- TenantTransaction(db) requires an authenticated user ID and server-derived school ID.
- It opens one GORM transaction, sets app.current_school_id and app.current_user_id with parameterized set_config(..., true), and exposes the transaction through TenantDBFromContext(c).
- Settings are transaction-local. They are never installed with session-level SET, so pooled connections cannot retain a prior request's tenant.
- The middleware buffers the HTTP response until commit succeeds. Error/aborted responses roll back; a commit failure returns a server error instead of an already-sent success.
- The `/admin/students` create, list, get, update, and delete routes now opt in. The `/admin/guardian-links` create, deactivate, and reactivate mutations also use the request transaction; their GuardianService repository and direct queries share that handle, and audit writes participate in the same transaction. Their controller resolves a request-scoped StudentService, whose direct user lookup, student repository queries, and audit writes all use the same transaction. The controller fails closed if the transaction middleware is missing.
- Guardian parent-portal read routes (`/parent/terms`, `/parent/children`, and child attendance, report-card, timetable, invoice, and payment reads) now use the request transaction. Their controller obtains a request-scoped GuardianService through `TenantDBFromContext(c)`, and all repository/direct GORM reads in that service use the same transaction. Each read endpoint fails closed if the middleware is absent. Other tenant routes remain unmigrated and must not be treated as RLS-ready; continue in bounded vertical slices and only install the middleware after every repository/service/audit query uses the request transaction.
- Timetable administration (`/admin/timetable` list/get/create/update/delete) and teacher timetable reads now use request-scoped `TimetableService`; validation queries, repository writes, conflict checks, and audit writes share the request transaction. Academic readiness (`/admin/school/readiness`) uses a request-scoped service for all school/session/term/class/subject/assignment reads.
- Student portal profile, enrollment, terms, attendance, timetable, report-card, invoice, and payment reads now use a request-scoped `StudentPortalService`. All 15 timetable/readiness/student-portal handlers fail closed when the tenant transaction middleware is missing.
- Person onboarding (`/admin/onboarding/people`) and school user CRUD (`/admin/users` create/list/get/delete) now use request-scoped services and audit writes on the same transaction. These routes fail closed when tenant transaction context is missing. User role assignment and activation/deactivation endpoints remain separately scoped for review.
- Bulk import request handlers and background workers remain separate audit work; worker jobs require their own tenant transaction context and must not be treated as covered by HTTP request middleware alone. Finance administration, dashboard, student career-path, announcements/notifications, and other remaining tenant routes are still pending and must not be treated as RLS-ready.

## PostgreSQL isolation test

The Go integration suite creates a temporary schema and a dedicated login role with NOSUPERUSER and NOBYPASSRLS. It enables and forces RLS on an isolated test table, applies a tenant policy, and connects through that restricted role. The test verifies:

- School A sees only School A's row.
- School A cannot update or delete School B's row.
- School A cannot insert a row assigned to School B.
- Same-school inserts succeed.
- A subsequent School B request on a one-connection pool sees only School B's row.
- A query without tenant transaction context sees no rows, proving transaction-local context is cleared after commit.

This is an isolated integration test only; it does not create or modify production roles, tables, grants, policies, credentials, or Render environment variables.

## Why the runtime role must not be switched yet

A role with NOBYPASSRLS will be subject to applicable RLS policies. Simply changing the Render connection string to such a role before policies and request transaction context exist can cause legitimate reads/writes to return no rows or fail. Conversely, setting a session-level custom GUC on a pooled connection is unsafe because a later request may reuse that connection with stale tenant state.

## Required implementation sequence

1. Request-scoped database transaction
   - Migrate every remaining tenant route, controller, repository, service, and audit write to the transaction returned by TenantDBFromContext. The student CRUD routes are the first integrated vertical slice.
   - Keep authentication/session lookups and platform-global operations on explicitly classified handles until their access model is defined.
   - Roll back on request failure and commit only after the response has been successfully prepared.
2. Tenant policy inventory
   - Classify each table as school-scoped, user-scoped, platform-global, public-read, or migration/system-only.
   - Include child tables whose ownership is derived through parent rows; verify tenant consistency with composite foreign keys or policy joins.
   - Handle users, sessions, refresh/reset tokens, billing, public school pages, background jobs, and super-admin operations explicitly.
3. Policies and grants
   - Add least-privilege DML grants to a dedicated API role that does not own application tables and has NOBYPASSRLS.
   - Keep DDL ownership and migration credentials separate; do not grant the API role membership in the migration/owner role.
   - Add fail-closed policies for every tenant table using transaction-local context, with both USING and WITH CHECK for writes.
   - Add narrowly scoped rules for platform-global and public endpoints; never treat missing tenant context as unrestricted access.
   - Review views and functions separately because their security mode can affect RLS behavior.
4. Isolation tests
   - Run PostgreSQL integration tests with two schools and the exact restricted API role.
   - Prove cross-school SELECT, UPDATE, DELETE, and INSERT are denied; same-school operations succeed.
   - Test no-context requests, context reset between pooled connections, transaction rollback, user-specific records, background jobs, and platform-admin workflows.
5. Controlled cutover
   - Provision roles and grants only after tests pass.
   - Set the Render API URL to the restricted role and enable REQUIRE_NOBYPASSRLS=true in the same approved cutover.
   - Keep `MIGRATION_DATABASE_URL` separate from the runtime URL. When `REQUIRE_NOBYPASSRLS=true`, startup now requires this separate connection and uses it for both versioned migrations and the one-time platform-admin bootstrap before closing it. Verify startup diagnostics and production smoke tests.
   - Have a tested rollback connection ready before changing production credentials.

## Startup guard

REQUIRE_NOBYPASSRLS=true makes startup fail if PostgreSQL role inspection fails or either the effective role or authenticated session role is superuser/BYPASSRLS. It is a deployment guard only; it does not create roles, grants, or RLS policies.

Keep the flag disabled until all tenant routes, table policies, and restricted-role isolation tests are implemented and validated in an isolated PostgreSQL environment. Do not apply production DDL or change Render secrets as part of this preparation work.

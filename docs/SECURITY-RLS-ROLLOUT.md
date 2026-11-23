# Restricted API role and tenant-aware RLS rollout

## Current verified posture

- The API currently connects as PostgreSQL postgres, which has BYPASSRLS.
- All 45 public application tables have RLS enabled, but RLS is not forced.
- Current deny_client_access policies target only anon and authenticated; they do not provide tenant isolation for a direct PostgreSQL API login.
- Direct Data API table privileges for anon and authenticated have been revoked.
- The API uses custom HMAC JWT authentication, not Supabase Auth JWT claims, as the source of tenant context.
- Gin validates a user and school context, but controllers and services currently hold a shared GORM database handle. A school ID in Gin context is not automatically attached to every SQL statement or guaranteed to use the same pooled connection.

## RLS trust boundary

The `app.current_school_id` and `app.current_user_id` values are custom PostgreSQL settings supplied by the application; they are not authenticated database identity claims. Any session using the runtime database login can set custom settings itself. These policies therefore protect against accidental unscoped application queries only while the Go authentication and transaction middleware remain trusted. They do not, by themselves, protect against arbitrary SQL execution/SQL injection or a compromised runtime database credential. Do not describe this as complete isolation from hostile SQL. Before making that stronger security claim, design and test a database-verifiable tenant context (for example, a signed context assertion) or another identity boundary that the runtime login cannot forge.

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
- Person onboarding (`/admin/onboarding/people`) and school user CRUD (`/admin/users` create/list/get/delete) now use request-scoped services and audit writes on the same transaction. These routes fail closed when tenant transaction context is missing.
- Role assignment and account activation/deactivation now require the request tenant transaction. Target-user and role-profile reads, role updates, role-change audit rows, account status changes, session/refresh-token revocation, and activation audit records share that transaction. The school role API rejects promotion to the platform super-admin role.
- Bulk import preview/start/list/get routes now use request-scoped tenant transactions; creation and queueing commit before the polling worker can observe the job. The worker claims jobs in a short database transaction with `FOR UPDATE SKIP LOCKED`, records a worker lease/attempt count, and reclaims expired or legacy unleased running jobs. Every job validation, status/progress update, and row import uses a separate transaction-local `app.current_school_id` and `app.current_user_id`; job updates are constrained by job ID, school ID, initiating user, and (for claimed work) worker ID. The worker verifies that the initiating user belongs to the job school before processing. The global queue claim now has a dedicated boundary in the draft implementation: PostgreSQL workers call `stonez_internal.claim_next_bulk_import_job`, a SECURITY DEFINER function outside Supabase's exposed public schema, owned by a NOLOGIN NOBYPASSRLS role with SELECT and claim-column UPDATE only on `bulk_import_jobs`; a table-specific RLS policy grants that role global visibility on the queue table without granting general RLS bypass. The NOBYPASSRLS API group receives EXECUTE on that function plus tenant-policy-controlled SELECT/INSERT/UPDATE for school-facing import operations. The function checks that the session login belongs to the API group; the installer creates school-scoped SELECT and school/initiator-scoped write policies, revokes private-schema access from PUBLIC/anon/authenticated, and the Go worker fails closed if the function is missing rather than falling back to direct global table access. This boundary is not deployed: the role/function scripts remain operator-run, Render still uses the existing `postgres`/BYPASSRLS connection, and no production role, grant, policy, or credential has been changed. A restricted-runtime cutover remains blocked until the scripts are separately approved/applied, a dedicated NOBYPASSRLS INHERIT API login is provisioned, the role-capable PostgreSQL tests pass, and all remaining tenant routes/policies are complete. Recovery is at-least-once; row handlers must remain idempotent, and external email delivery cannot be made exactly-once by the database transaction alone. Finance fee, invoice, and payment administration now uses request-scoped FinanceService repositories, validation queries, nested write transactions, and audit writes on the same tenant transaction. School dashboard reads now use a request-scoped SchoolDashboardService and fail closed without tenant transaction context. Announcement creation, publication, listing, and parent notification list/read routes now use a request-scoped NotificationService and repository; audience lookup failures abort publication, and notification writes, announcement state, and audit records share the request transaction. Student career-path and other remaining tenant routes are still pending and must not be treated as RLS-ready.

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

# Platform and tenant database access boundary

**Status:** architecture proposal for security-owner review. This document is not a database migration and does not provision roles, grants, credentials, or policies.

## Decision

Use separate PostgreSQL login identities and separate application connection pools for tenant requests and platform administration. Both runtime identities must be `NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION`, must not own application tables, and must not be members of the migration/owner role.

Do not implement platform access as a broad exception in the tenant role's policies. Do not use a caller-controlled tenant GUC, JWT role claim, nullable `school_id`, or request parameter as a platform authorization mechanism.

Proposed identity names (final names are subject to repository/configuration review):

- `stonez_tenant_runtime`: tenant-scoped application operations.
- `stonez_platform_runtime`: explicitly authorized platform administration and platform audit operations.
- `stonez_auth_runtime`: narrowly scoped pre-authentication lookup operations, if the reviewed implementation chooses a dedicated login.
- `stonez_worker_runtime`: queue claim/lease and background processing operations.
- migration/owner identity: DDL and bootstrap only; never used by HTTP runtime pools.

The auth and worker identities are separate capabilities, not permission to use the platform pool.

## Trust boundaries

### Tenant runtime

- Used only by tenant routes after authentication and server-derived school membership have been established.
- Each tenant request uses one transaction and transaction-local `app.current_school_id` and `app.current_user_id` settings.
- All handler, service, repository, and audit queries for an opted-in route must use that transaction handle.
- RLS policies target the tenant runtime role and fail closed when context is absent or invalid.
- School-owned rows require matching school context. User-private rows additionally require matching user context.
- The tenant role receives no platform-wide audit access, global school-directory access, migration-ledger access, or queue-claim capability.
- Do not install tenant transaction middleware globally until every tenant route has been migrated to the request-scoped handle.

### Platform runtime

- Used only by platform-administration services after server-side authorization has verified the actor, action, and target.
- Has a distinct DSN and pool; tenant handlers must not receive or import this handle.
- RLS policies target the platform runtime database role explicitly. The role remains subject to RLS; it is not a superuser, table owner, or BYPASSRLS role.
- Platform policies are explicit per table and operation. They do not grant blanket access to every application table.
- Platform actions record actor, action, target, and outcome in the platform audit stream.
- Platform identity is established by the database connection role, not by setting `app.is_platform_admin` or accepting a JWT claim as a database capability.
- Application authorization remains mandatory: possession of the pool alone must not make an untrusted HTTP caller an administrator.

### Authentication bootstrap

Current `AuthMiddleware` reads `sessions` and `users` before tenant context is installed. `RequireSchoolContext` also performs a pre-transaction user lookup. These lookups need a deliberate design before a NOBYPASSRLS tenant pool can replace the current privileged connection.

Preferred narrow options for review:

1. A dedicated auth runtime role with execute permission on carefully reviewed, fixed-purpose database functions for session validation, account active/role lookup, and school membership lookup; or
2. A dedicated auth pool with column- and operation-limited grants plus row policies that scope each lookup to the presented session/user identifier.

Do not grant the tenant role unrestricted SELECT on all users or sessions merely to make login work. Token hashes and password/reset secrets must not be exposed. The selected mechanism must handle revoked/expired sessions, inactive users, role changes, missing school membership, and account enumeration safely.

Any SECURITY DEFINER function must have a fixed safe search_path, a non-login owner, minimal privileges, validated arguments, no dynamic SQL from caller input, and explicit EXECUTE grants. Review function ownership and behavior as part of migration parity.

### Background workers

The current bulk-import claim uses the privileged PostgreSQL connection and global `FOR UPDATE SKIP LOCKED` queue access. It is not ready for a tenant NOBYPASSRLS role.

Use a dedicated worker capability. Prefer narrowly scoped database functions for claim/lease/heartbeat/release, or a worker role with narrowly scoped policies and grants. Processing a claimed job must re-enter a tenant-scoped transaction and verify job school, initiating user, and worker lease. The worker must not receive platform-administration privileges.

### Public content

Unauthenticated pages must not receive broad table grants. Use narrowly scoped application endpoints or reviewed views/functions that expose only published fields and rows after trusted hostname/slug resolution. Enquiry creation is an application write path, not permission to read enquiry rows.

## Role and grant invariants

For every runtime identity, automated catalog tests must assert:

- `rolsuper = false`, `rolbypassrls = false`, `rolcreatedb = false`, `rolcreaterole = false`, and `rolreplication = false`.
- Runtime role is not owner of any protected application table.
- Runtime role is not a member of the migration/owner role or another role that can bypass RLS or alter protected tables.
- No unintended `TRUNCATE`, `REFERENCES`, `TRIGGER`, DDL, schema-create, or migration-ledger privileges.
- Grants are explicit by schema, table, sequence, function, and operation.
- RLS is both enabled and forced for each protected table.
- Policy role targets and `USING` / `WITH CHECK` expressions match the approved inventory exactly.
- SECURITY DEFINER functions, views, triggers, and foreign-key behavior are separately reviewed; catalog existence alone is not sufficient.

Do not rely on `PUBLIC` grants or implicit membership.

## Policy design constraints

The policy migration must be based on the reviewed table inventory and actual schema constraints. Do not write generic policies against every table.

- School-owned tables: tenant role only; fail closed without `app.current_school_id`; both `USING` and `WITH CHECK` for writes.
- User-private tables: include authenticated user ownership in addition to school ownership where applicable.
- `career_path_attempts`: verify school, student, and user ownership; a school-only policy is insufficient.
- Parent/child rows: ensure child school ownership agrees with the parent. Prefer composite foreign keys or carefully reviewed policy predicates; RLS on a child table alone does not establish referential tenant consistency.
- `users`: school users and NULL-school platform identities must be separated. NULL is not a wildcard.
- `audit_logs` and `role_change_audits`: school identities see only their own school's records. NULL-school platform events are platform-only.
- `schools`, billing, curricula, subscriptions, and public content require explicit per-operation decisions; do not infer that platform-global means all-platform-role DML.
- Queue/system/migration records are unavailable to the tenant runtime except through approved narrow capabilities.

### Important threat-model note

Transaction-local custom GUCs prevent pooled-connection context leakage, but PostgreSQL custom settings are not cryptographic identity. A role that can execute arbitrary SQL can generally set a custom GUC itself. The RLS design therefore assumes application SQL is parameterized and that SQL injection is separately prevented; it must not claim that a GUC alone protects against arbitrary SQL execution by a compromised tenant database credential. Keep platform authorization independent of tenant GUCs and isolate the platform credential.

## Implementation sequence

1. **Complete route inventory:** identify every remaining shared-handle query in authentication, school profile/logo, audit, CareerPath, content, platform admin, and worker paths.
2. **Approve bootstrap design:** choose the auth lookup mechanism and implement it without broad tenant-role reads.
3. **Separate application handles:** introduce explicit tenant/platform/auth/worker database configuration and constructors. Make privileged/migration access unavailable to request handlers by default.
4. **Finish route migration:** tenant routes use `TenantDBFromContext`; platform services use only the platform service boundary; auth and worker paths use their own narrow interfaces.
5. **Write policy SQL:** create a separately reviewed, versioned migration with explicit policies and grants for the classified tables. No production execution in this phase.
6. **Test exact migration:** apply the actual versioned migrations and policy migration in disposable PostgreSQL; do not create synthetic policies in test setup.
7. **Exercise real isolation:** connect as each non-owner runtime role and test two schools, two users within one school, NULL-school platform events, public published/unpublished content, auth bootstrap, platform operations, and worker queue capabilities.
8. **Reconcile staging:** capture a read-only catalog export and compare tables, owners, RLS flags, policy definitions, role attributes/memberships, and grants with the migration baseline.
9. **Cutover review:** only after every gate passes, prepare a separately approved role provisioning, secret configuration, smoke test, and rollback plan. No production change is authorized by this document.

## Required negative tests

- Missing, empty, malformed, and stale tenant context returns no rows or rejects writes.
- School A cannot read/update/delete School B rows or insert/reassign rows to School B.
- A different user in School A cannot read another user's CareerPath attempt or private session/token data.
- Child rows cannot reference parents from another school.
- Neither school can read NULL-school platform audit events.
- Tenant runtime cannot execute platform operations even if an HTTP request supplies a platform-looking claim or identifier.
- Platform runtime can perform only explicitly granted operations and is still subject to RLS.
- Auth runtime cannot enumerate arbitrary users or retrieve secret columns.
- Worker runtime cannot access platform data or claim arbitrary non-queue records.
- Context is cleared after commit and rollback on a one-connection pool.
- Failed requests and commit failures do not leak success responses or leave partial database writes.

## Current blockers carried forward

- The clean Go migration baseline has 42 tables, 15 RLS-enabled, zero policies, and no FORCE RLS.
- The rollout document reports a different deployed-catalog snapshot (45 tables, RLS enabled but not forced). This is unverified in this task and requires approved read-only staging reconciliation.
- The current API connection is documented as PostgreSQL `postgres` with BYPASSRLS.
- Many routes remain unmigrated from the shared GORM handle.
- Auth middleware and school-context lookup execute before tenant transaction context.
- Platform controllers currently use the shared DB handle for cross-school operations.
- Bulk-import queue claiming still depends on the privileged connection.

**Safety:** this proposal changes no database objects or credentials. Keep related PRs draft/unmerged; do not enable `REQUIRE_NOBYPASSRLS`, provision production roles, change secrets, or deploy until the complete test and rollback gates are independently approved.

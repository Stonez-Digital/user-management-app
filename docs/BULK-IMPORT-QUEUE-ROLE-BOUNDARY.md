# Bulk-import queue role boundary

## Purpose

The worker's global claim operation must continue to claim queued jobs atomically without giving the API runtime direct access to the cross-tenant queue. The intended design is one narrowly scoped PostgreSQL SECURITY DEFINER function, owned by a dedicated role, with execution granted only to a non-login API group role.

The repository now includes the role-preparation scripts, a SECURITY DEFINER claim-function installer, a PostgreSQL worker call, and isolated PostgreSQL tests. The scripts are operator-run; none has been executed against production.

## Roles

| Role | Login | RLS bypass | Intended authority |
|---|---:|---:|---|
| stonez_bulk_queue_owner | No | No | Owns the claim function; can SELECT queue rows and UPDATE only lease/claim columns on public.bulk_import_jobs. |
| stonez_api_runtime | No | No | Receives EXECUTE on the global claim function plus SELECT/INSERT/UPDATE on bulk_import_jobs; SELECT is school-scoped and INSERT/UPDATE are school-and-initiator-scoped. |
| Actual API login | Yes | No | Must be a dedicated NOBYPASSRLS, INHERIT login, later granted membership in stonez_api_runtime. It must not be postgres or a member of the queue-owner role. |

The queue owner remains NOBYPASSRLS. A dedicated `FOR ALL` RLS policy on `bulk_import_jobs` grants this role global visibility only on that queue table; narrow column-level UPDATE grants still limit it to lease/claim columns. It has no privileges on other application tables, no DDL privileges after installation, no role-management privileges, and no login capability.

The provisioning script does not create an API login or password, and does not grant role membership to any existing login. The function installer grants the API group role only tenant-policy-controlled SELECT/INSERT/UPDATE on the queue table; it does not grant DELETE or TRUNCATE. The production Render connection currently uses postgres; do not grant the group role to that connection or treat it as a restricted runtime.

The claim function returns a full queue row across schools. Consequently, every login granted membership in `stonez_api_runtime` can invoke the global claim operation and receive a job payload; this is a worker-capable service privilege, not a school-facing user permission. Grant membership only to the trusted service identity that runs the worker. If HTTP request handling and background processing are later separated into different processes, use a worker-only role and database connection rather than sharing this global claim capability with the web API login.

## Preparation

1. Review scripts/security/provision_bulk_import_queue_roles.sql against the target database and confirm public.bulk_import_jobs exists with the lease columns from migration 47.
2. When separately approved, run scripts/security/provision_bulk_import_queue_roles.sql using an administrative database connection that can create/alter roles and grant table privileges.
3. Run scripts/security/verify_bulk_import_queue_roles.sql before function installation and retain its successful result as role-preparation evidence.
4. Review and run scripts/security/install_bulk_import_claim_function.sql only after the role assertions pass. This creates the non-exposed stonez_internal schema, revokes schema access from PUBLIC/anon/authenticated, creates the SECURITY DEFINER function there as the queue-owner role, and installs school-scoped SELECT plus school-and-initiator-scoped write policies for normal API operations.
5. Identify and provision a dedicated API login through the secret-management process. Do not put its password in this repository or in migration SQL. Grant that login membership in stonez_api_runtime only after confirming it is NOBYPASSRLS, INHERIT, and cannot assume a privileged role.
6. Do not change Render's DATABASE_URL, enable REQUIRE_NOBYPASSRLS, or grant the current postgres login membership in either role during this preparation stage.

All three SQL scripts are explicit operator-run artifacts. They are not executed by application startup, CI migrations, or the normal Migrate function. No production role or grant is changed by merging these files.

## Required PostgreSQL integration test setup

The isolated PostgreSQL bulk-worker test harness uses a database where the test administrator can create and drop temporary roles. It creates uniquely named test roles and removes them in cleanup; it never reuses or alters the production role names.

The PostgreSQL worker tests create uniquely named roles and isolated schemas, execute the shipped provisioning/verification/installation SQL scripts with substituted test identifiers, exercise the installed function and worker lifecycle, and clean up all roles/grants/schemas. They skip clearly when TEST_DATABASE_URL is absent or cannot create roles. The tests must establish the following using actual PostgreSQL role switching, not SQLite mocks:

- A NOBYPASSRLS caller can SELECT, INSERT, and UPDATE jobs only within the school and initiating-user context set transaction-locally by the API.
- Missing tenant context returns no rows and rejects inserts; cross-school SELECT/UPDATE/INSERT are denied by RLS.
- The API caller cannot DELETE or TRUNCATE queue rows.
- The caller can execute the global queue-claim function without receiving unrestricted direct-table access; the function rejects a session login that is not a member of stonez_api_runtime.
- The function owner is a non-login, non-superuser NOBYPASSRLS role with only SELECT and claim-column UPDATE on the queue table; its RLS bypass is not inherited by the API group.
- PUBLIC and unrelated roles cannot execute the function.
- Concurrent callers using FOR UPDATE SKIP LOCKED receive distinct jobs.
- Queued jobs and expired/legacy running leases are claimable; an active unexpired lease is not.
- The function returns the queue job payload needed by the worker; only its lease/claim columns can be updated by the function owner.
- The Go worker processes a claimed job only under its school and initiating-user transaction context; a mismatched or expired worker lease cannot write.
- Role memberships do not allow the API caller to SET ROLE to or inherit the queue-owner role.

If the test database does not permit role creation, the integration test must report a clear skip rather than silently substituting SQLite coverage. CI must include a PostgreSQL job with role-creation capability before this boundary is considered verified.

## Implemented queue boundary

The claim-function installer and Go call are now in this branch together. The PostgreSQL worker path invokes stonez_internal.claim_next_bulk_import_job(worker_id) and fails closed if it is missing; it has no direct-table fallback. SQLite retains its existing direct claim implementation for local unit tests only. The function is stored outside Supabase's exposed public schema, uses a fixed pg_catalog search_path and schema-qualified table references, verifies session_user belongs to stonez_api_runtime, accepts a required worker UUID, uses a five-minute database-clock lease and atomic FOR UPDATE SKIP LOCKED selection, and has explicit EXECUTE revocations/grants. Keep stonez_internal out of the Supabase Data API exposed-schema list. The installer also creates three RLS policies: school-scoped SELECT, school-and-initiator-scoped INSERT, and school-and-initiator-scoped UPDATE. The API group receives only SELECT/INSERT/UPDATE; global claim is the function's narrowly scoped exception.

The function owner and API group role names in the provisioning scripts are the intended production names. The SQL installer validates role attributes before installing/changing function ownership. Do not silently create privileged roles in the ordinary application migration path.

## Safety status

No production SQL has been executed. No database role, grant, RLS policy, Render setting, or secret has been changed. The runtime still uses the existing privileged connection, and this preparation does not make a NOBYPASSRLS cutover safe.

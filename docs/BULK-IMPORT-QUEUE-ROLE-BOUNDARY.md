# Bulk-import queue role boundary

## Purpose

The worker's global claim operation must continue to claim queued jobs atomically without giving the API runtime direct access to the cross-tenant queue. The intended design is one narrowly scoped PostgreSQL SECURITY DEFINER function, owned by a dedicated role, with execution granted only to a non-login API group role.

This document records the role preparation and verification stage. It does not install the function, change the worker, or authorize a production database change.

## Roles

| Role | Login | RLS bypass | Intended authority |
|---|---:|---:|---|
| stonez_bulk_queue_owner | No | Yes | Owns the claim function; can SELECT queue rows and UPDATE only lease/claim columns on public.bulk_import_jobs. |
| stonez_api_runtime | No | No | Receives EXECUTE on the claim function after it exists; receives no direct queue-table privileges. |
| Actual API login | Yes | No | Must be a dedicated restricted login, later granted membership in stonez_api_runtime. It must not be postgres or a member of the queue-owner role. |

The queue owner requires BYPASSRLS because the claim operation is intentionally global across schools. This is bounded by table ACLs: it receives no privileges on other application tables, no DDL privileges, no role-management privileges, and no login capability. The API runtime role itself remains NOBYPASSRLS.

The provisioning script does not create an API login or password, and does not grant role membership to any existing login. The production Render connection currently uses postgres; do not grant the group role to that connection or treat it as a restricted runtime.

## Preparation

1. Review scripts/security/provision_bulk_import_queue_roles.sql against the target database and confirm public.bulk_import_jobs exists with the lease columns from migration 47.
2. When separately approved, run the script using an administrative database connection that can create/alter roles and grant table privileges.
3. Run scripts/security/verify_bulk_import_queue_roles.sql and retain its successful result as deployment evidence.
4. Identify and provision a dedicated API login through the secret-management process. Do not put its password in this repository or in migration SQL.
5. Do not change Render's DATABASE_URL, enable REQUIRE_NOBYPASSRLS, or grant the current postgres login membership in either role during this preparation stage.

Both scripts are explicit operator-run artifacts. They are not executed by application startup, CI migrations, or the normal Migrate function. No production role or grant is changed by merging these files.

## Required PostgreSQL integration test setup

The next implementation stage must extend the isolated PostgreSQL bulk-worker test harness. It must use a database where the test administrator can create and drop temporary roles. It must create uniquely named test roles and remove them in cleanup; it must never reuse or alter the production role names.

The tests must establish all of the following using actual PostgreSQL role switching, not SQLite mocks:

- A NOBYPASSRLS caller cannot SELECT, INSERT, UPDATE, DELETE, or TRUNCATE bulk_import_jobs directly.
- The caller can execute only the queue-claim function.
- The function owner is a non-login, non-superuser BYPASSRLS role with only SELECT and claim-column UPDATE on the queue table.
- PUBLIC and unrelated roles cannot execute the function.
- Concurrent callers using FOR UPDATE SKIP LOCKED receive distinct jobs.
- Queued jobs and expired/legacy running leases are claimable; an active unexpired lease is not.
- The function returns only the job fields the worker needs and cannot update non-claim columns.
- The Go worker processes a claimed job only under its school and initiating-user transaction context; a mismatched or expired worker lease cannot write.
- Role memberships do not allow the API caller to SET ROLE to or inherit the queue-owner role.

If the test database does not permit role creation, the integration test must report a clear skip rather than silently substituting SQLite coverage. CI must include a PostgreSQL job with role-creation capability before this boundary is considered verified.

## Next implementation stage

After role preparation and the restricted-role PostgreSQL test harness are reviewed, add the SQL function and Go worker call in the same change. The function must use a fixed safe search_path, fully qualified table references, parameterized worker identity/time inputs, atomic row locking and lease updates, and explicit EXECUTE revocations/grants. The Go worker must fail closed if the function is unavailable; it must not fall back to direct global table access.

The function owner and API group role names in the provisioning scripts are the intended production names. The eventual migration must verify that these roles are provisioned with the expected attributes and grants before installing/changing function ownership. Do not silently create privileged roles in the ordinary application migration path.

## Safety status

No production SQL has been executed. No database role, grant, RLS policy, Render setting, or secret has been changed. The runtime still uses the existing privileged connection, and this preparation does not make a NOBYPASSRLS cutover safe.

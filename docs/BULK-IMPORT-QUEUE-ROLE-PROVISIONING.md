# Bulk-import queue role provisioning and restricted-role test plan

## Scope and safety

This is the prerequisite for replacing the bulk-import worker's global table claim with a narrowly scoped PostgreSQL function. It is a preparation document only: it does not create roles, change grants or policies, alter Render settings, or apply production SQL.

The current worker still claims jobs directly through the application database connection. Do not enable `REQUIRE_NOBYPASSRLS` or change the production `DATABASE_URL` as part of this work.

## Role model

Use two dedicated, non-login roles. Do not reuse `postgres`, `service_role`, the migration login, or the application login.

| Role | Attributes | Purpose |
| --- | --- | --- |
| `stonez_bulk_queue_owner` | NOLOGIN, NOSUPERUSER, NOBYPASSRLS | Owns only the queue-claim function. Receives narrowly scoped column privileges and a dedicated RLS policy on `public.bulk_import_jobs`. |
| `stonez_api_runtime` | NOLOGIN, NOSUPERUSER, NOBYPASSRLS | Group role for the API login. Receives EXECUTE on the claim function, but no direct privileges on the global queue table. |

The function owner should **not** be a table owner and should **not** have BYPASSRLS. The table remains RLS-enabled; a policy addressed only to `stonez_bulk_queue_owner` grants that role the queue-wide row access needed by the definer function. The role receives only SELECT on the fields the function must return and UPDATE on claim-state fields. It receives no INSERT, DELETE, TRUNCATE, schema ownership, or membership in privileged roles.

The API login must be a member of `stonez_api_runtime`, but must not be a member of `stonez_bulk_queue_owner`. The actual runtime login name is deliberately not guessed here; inspect the approved runtime connection identity before granting membership.

## Provisioning preflight

Before applying a role-provisioning script in any environment:

1. Confirm the target database and environment explicitly.
2. Confirm `public.bulk_import_jobs` exists and RLS is enabled.
3. Confirm neither role name is already occupied by an unrelated role. If a name exists, inspect its attributes, memberships, ownerships, grants, and dependencies; do not silently repurpose it.
4. Confirm the provisioning identity can create roles and grant the required memberships. Do not broaden the migration login or runtime login to make provisioning succeed.
5. Confirm the migration/deployment identity can create the function and assign its owner to `stonez_bulk_queue_owner`.
6. Confirm the runtime login is known and is not privileged, table-owning, or a member of the function-owner role.
7. Record the before-state of role attributes, memberships, table ACLs, RLS policies, and function ACLs for rollback.

Provisioning is a separate administrative operation, not an automatic side effect of application startup or the ordinary versioned schema migration. Do not put a guessed production login name into migration SQL.

## Function contract to implement next

The eventual function should be a single atomic claim operation:

- Accept only a worker UUID; derive eligibility and tenant/job values from the database.
- Select the oldest eligible queued job or expired/legacy unleased running job using `FOR UPDATE SKIP LOCKED`.
- Update status, worker ID, claim timestamp, attempts, and last error in the same transaction.
- Return only the job fields required by the worker (including the import payload and stored validation errors); never return unrelated columns.
- Use schema-qualified table/function references and a fixed safe `search_path`.
- Be `SECURITY DEFINER`, owned by `stonez_bulk_queue_owner`.
- Revoke EXECUTE from `PUBLIC`; grant EXECUTE only to `stonez_api_runtime`.
- Do not grant the runtime role direct SELECT/UPDATE on `bulk_import_jobs`.
- Preserve the existing worker-lease fencing and tenant-scoped transactions for all processing and progress updates.

The Go worker must call this function and scan its restricted return shape. It must not retain a fallback to the old global GORM claim query when the function is unavailable; failure should be visible and fail closed.

## Restricted-role PostgreSQL test fixture

The integration test must run against an isolated PostgreSQL database/schema and must never target production. The fixture should:

1. Require an explicit `TEST_DATABASE_URL`; skip with a clear reason when absent.
2. Verify the test connection is not production by requiring a dedicated test database name/explicit opt-in, and create a unique temporary schema.
3. Create uniquely named temporary NOLOGIN roles for queue owner and runtime group; do not use fixed shared test role names.
4. Create the queue table in the isolated schema with RLS enabled and no table ownership by either role.
5. Grant the queue owner only the necessary column-level SELECT/UPDATE privileges and create the owner-only queue policy.
6. Create the definer function with fixed search path and assign ownership to the queue-owner role.
7. Grant function EXECUTE to the runtime role, but no direct queue table privileges.
8. Connect as a temporary NOBYPASSRLS login that is a member only of the runtime role.
9. Assert direct SELECT, UPDATE, INSERT, and DELETE against the queue table are denied for the runtime login.
10. Assert function execution succeeds and returns a claimed job.
11. Assert an unrelated role cannot execute the function.
12. Assert two concurrent workers never claim the same job and that `SKIP LOCKED` allows another eligible job to be claimed.
13. Assert expired leases and legacy running jobs with null claim timestamps can be recovered, attempts increment once, and active leases cannot be stolen.
14. Assert the worker's tenant-scoped processing transaction cannot read or mutate another school's data, including when a pooled connection is reused.
15. Drop the temporary schema and temporary roles in cleanup, including cleanup on test failure where possible.

If the CI PostgreSQL identity cannot create and drop temporary roles, add a dedicated isolated CI database job with the minimum administrative capability required. Do not weaken the test by running the restricted-role assertions as the schema owner.

## Acceptance gates

- [ ] Role provisioning SQL reviewed and idempotent; no production identity hardcoded.
- [ ] Role owner is NOLOGIN, NOSUPERUSER, NOBYPASSRLS and not a table owner.
- [ ] API group role is NOLOGIN, NOSUPERUSER, NOBYPASSRLS.
- [ ] Runtime login is not a member of the queue-owner role.
- [ ] Queue owner has only the documented column grants and one dedicated RLS policy.
- [ ] Runtime has no direct queue-table privileges.
- [ ] Function has fixed safe search path, restricted return columns, and no PUBLIC EXECUTE.
- [ ] Restricted-role tests prove direct table denial and authorized function execution.
- [ ] Concurrency, lease recovery, unauthorized execution, and tenant-isolation tests pass on PostgreSQL.
- [ ] Go worker has no privileged direct-query fallback.
- [ ] Existing CI workflows pass.

Only after these gates should the function migration and Go integration be prepared for review. The broader tenant-route and RLS-policy inventory remains a separate prerequisite to any production runtime cutover.

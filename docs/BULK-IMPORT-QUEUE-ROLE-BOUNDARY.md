# Bulk-import queue role boundary

## Purpose

The global queue claim operation is a worker-only capability. It must not be available to the HTTP API runtime or to any school-facing login. A SECURITY DEFINER function, owned by a dedicated non-login queue-owner role, performs atomic cross-tenant job claiming. Only a dedicated claim-only login can execute it.

The SQL and provisioning tools are operator-run. Committing these files does not execute them or change staging/production roles, grants, secrets, or deployments.

## Roles and permissions

| Role | Login | Intended authority |
|---|---:|---|
| stonez_bulk_queue_owner | No | Owns the claim function; SELECT on the queue and UPDATE only on lease/claim columns |
| stonez_api_runtime | No | Tenant-policy-controlled SELECT/INSERT/UPDATE on bulk_import_jobs; no EXECUTE on the global claim function |
| stonez_bulk_claim_group | No | USAGE on the required schemas and EXECUTE on the claim function only; no direct queue-table privileges |
| stonez_bulk_claim_login | Yes | Inherits only stonez_bulk_claim_group; cannot SET ROLE to it; no API, queue-owner, service, or privileged memberships |
| Actual HTTP API login | Yes | Separate NOBYPASSRLS runtime identity; never receives the claim group |

The claim login must be a LOGIN, NOSUPERUSER, NOBYPASSRLS, NOCREATEDB, NOCREATEROLE role. Its only membership is stonez_bulk_claim_group, with INHERIT TRUE and SET FALSE. The claim group itself must be NOLOGIN, NOSUPERUSER, NOBYPASSRLS, NOCREATEDB, NOCREATEROLE, INHERIT, and must not inherit any other role.

The queue-owner role remains NOBYPASSRLS. Its queue-wide RLS policy applies only to bulk_import_jobs. It receives SELECT and narrowly scoped UPDATE privileges for claim/lease columns, no payload writes, no INSERT/DELETE/TRUNCATE, no other application-table privileges, and no login capability.

## Function authorization

The function checks session_user rather than current_user because it is SECURITY DEFINER. It requires effective inherited membership in stonez_bulk_claim_group, rejects SET capability to that group, and rejects API-runtime, queue-owner, service-role, and other privileged memberships. It also rejects any additional membership beyond the dedicated claim group.

The function EXECUTE ACL is granted only to stonez_bulk_claim_group. PUBLIC and stonez_api_runtime must not be able to execute it. The claim login has no direct SELECT, INSERT, UPDATE, DELETE, or column-level privilege on bulk_import_jobs. Its only queue operation is the SECURITY DEFINER function.

The function returns the full claimed job row across schools. Therefore, the claim credential belongs only on the standalone background worker. Never configure it on the HTTP API service or share it with school-facing users.

## Operator preparation sequence

1. Confirm public.bulk_import_jobs exists with lease_generation and the expected claim columns.
2. Run scripts/security/provision_bulk_import_queue_roles.sql using an appropriately authorized database operator.
3. Run scripts/security/verify_bulk_import_queue_roles.sql before installing the function. Retain the output.
4. Review and run scripts/security/install_bulk_import_claim_function.sql. It creates the private stonez_internal schema, installs the SECURITY DEFINER function with a fixed pg_catalog search_path, and grants EXECUTE only to the claim group.
5. Run scripts/security/verify_bulk_import_queue_roles.sql again. It must confirm the function ACL and absence of API-runtime EXECUTE.
6. From an interactive psql session, run scripts/security/provision_bulk_import_claim_login.psql. It prompts for the password without echoing it and verifies the role boundary.
7. Store the resulting claim-login DSN only in the background worker's BULK_IMPORT_CLAIM_DATABASE_URL setting. Do not place the credential on the HTTP API service.
8. Keep the processing database connection separate. It must use the dedicated processing login described in the signed-context implementation document, not the claim login.

The older scripts/security/provision_bulk_import_worker_login.psql provisions an API-runtime login for tenant-scoped API checks. It is not the claim credential and must not be used for BULK_IMPORT_CLAIM_DATABASE_URL. Existing logins created by the older script are not automatically modified or dropped; any retirement requires a separately approved cutover.

## Required PostgreSQL evidence

The isolated PostgreSQL test suite must use actual LOGIN connections and demonstrate:

- The claim login inherits the claim group, cannot SET ROLE to it, and has no additional role memberships.
- The claim login is not a member of stonez_api_runtime, stonez_bulk_queue_owner, service_role, or any privileged role.
- The claim login cannot directly SELECT or mutate bulk_import_jobs.
- The claim login can execute the installed claim function and atomically claims one eligible job.
- The HTTP API runtime login cannot execute the global claim function.
- An unrelated login and the CREATEROLE installer cannot execute the function.
- The queue owner cannot write payload columns or acquire unrelated table privileges.
- The API runtime retains only its tenant-policy-controlled queue access and cannot DELETE/TRUNCATE.
- Concurrent claims use FOR UPDATE SKIP LOCKED and return distinct jobs.
- Expired/legacy leases can be reclaimed, while active unexpired leases cannot.
- Role membership, function ACLs, schema usage and table/column privileges match the intended capability matrix.

The test harness uses unique role names and schemas and cleans them up. The queue SQL-script fixture runs in a disposable database separate from the application/authentication PostgreSQL test database. PostgreSQL tests must not rely on one suite leaving stonez_internal behind for another suite.

## Relationship to signed processing context

Claiming and processing are separate trust boundaries:

- The claim login can only claim a job and receive its trusted queue record.
- The processing login cannot claim jobs and cannot read verifier key/session tables.
- The Go worker signs the job, school, initiating user, worker, lease generation, backend PID and transaction ID using a Vault-backed HMAC key.
- PostgreSQL verifies the assertion and binds tenant RLS context to the exact backend transaction and current lease.

The signed-context implementation is tracked in PR #362. The claim-only login implementation is a separate follow-on boundary. Neither change authorizes the processing login to access tenant-owned import tables until their exact table/column grants and RLS policies have been separately reviewed and tested.

## Deployment status and safety

These scripts and tests do not make live changes. Do not alter Render DATABASE_URL, configure the claim DSN, grant memberships to the current postgres API connection, or deploy the worker as part of this repository change. Staging provisioning and acceptance require separate explicit approval and retained evidence.

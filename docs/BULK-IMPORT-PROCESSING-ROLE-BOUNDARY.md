# Bulk import worker processing-role boundary

## Status

This document defines the next security boundary for the worker introduced by the dedicated claim-connection work. It does not provision roles, grant privileges, install policies, change credentials, or authorize a deployment.

The existing worker has two database handles:

- The claim handle calls `stonez_internal.claim_next_bulk_import_job(uuid)`.
- The processing handle runs tenant transactions and performs import reads/writes.

The processing handle remains transitional until the dedicated role and all table policies have been installed and tested in an isolated PostgreSQL environment.

## Role separation contract

Use three independent database identities:

1. **Claim login** — authenticates only to execute the claim function. It must not inherit `stonez_api_runtime`, `stonez_bulk_queue_owner`, or `service_role`, and must have no direct INSERT/UPDATE/DELETE privilege on `bulk_import_jobs`. The SECURITY DEFINER function owner remains a NOLOGIN role with only queue SELECT and the five claim/progress column updates it needs.
2. **Processing login** — authenticates as its own LOGIN, NOSUPERUSER, NOBYPASSRLS role. It must not be a member of the API runtime, queue owner, or Supabase service role; must not be able to assume privileged roles; and must not have CREATEDB or CREATEROLE.
3. **Processing privilege group** — NOLOGIN, NOSUPERUSER, NOBYPASSRLS. It receives only the required processing table/column privileges and is not a member of any owner, migration, API, queue-owner, or service role.

The processing login should inherit only the processing privilege group. Do not reuse `stonez_bulk_worker` until its current API-runtime membership and effective queue table-level UPDATE privileges have been removed through a separately approved role-provisioning change.

## Processing operation inventory

The current worker code uses these operations:

| Table | Operations | Tenant boundary |
|---|---|---|
| `users` | SELECT, INSERT | School ownership; user creation must not cross schools |
| `students` | SELECT, INSERT | School ID |
| `teacher_profiles` | SELECT, INSERT | School ID |
| `parent_profiles` | SELECT, INSERT | School ID |
| `guardian_relationships` | SELECT, INSERT | School ID and both related entities |
| `student_enrollments` | SELECT, INSERT | School ID and student/session/class/section consistency |
| `academic_sessions` | SELECT | School ID |
| `school_classes` | SELECT | School ID |
| `sections` | SELECT | School ID and parent class |
| `bulk_import_jobs` | SELECT, narrowly scoped UPDATE | School, initiator, worker ID and lease |

These are code-derived operations, not verified live grants. Re-check the worker code whenever this inventory changes.

## Tenant context: current limitation

The worker currently sets `app.current_school_id` and `app.current_user_id` transaction-locally from the claimed job. Transaction-local settings prevent pooled-connection leakage, but any holder of the database login can set arbitrary custom GUC values.

Therefore, GUC-based policies are useful protection against accidental unscoped queries, but they are not a database-verifiable identity boundary against arbitrary SQL execution or a compromised processing credential.

Do not claim hostile-SQL tenant isolation on the strength of these settings alone.

Before issuing processing grants, implement and test a context assertion that the processing login cannot forge. The assertion must bind at least:

- job ID;
- school ID;
- initiating user ID;
- worker ID and current lease;
- an expiry/lease condition;
- an integrity proof whose signing key is unavailable to SQL callers using the processing login.

The database must validate the assertion against the current claimed job and reject missing, expired, replayed, or mismatched assertions. The worker must derive all values from the row returned by the claim function, never from uploaded row data. The exact signing/verification mechanism and key custody need a separate reviewed implementation; do not improvise a shared GUC secret or put an HMAC key in SQL-visible session settings.

A safer alternative may be a database-side worker capability table/function with unforgeable, short-lived capability material and strict EXECUTE-only access. Whichever design is selected must have equivalent replay, concurrency, revocation, and tenant-binding tests.

## Required RLS and privilege gates

Before the processing login receives table access:

1. Enable and force RLS on each tenant-owned processing table where ownership and maintenance paths permit it.
2. Add fail-closed policies for SELECT and INSERT on all nine processing data tables, including parent/child consistency checks for guardian relationships and enrollments.
3. Add a job policy that restricts reads and updates to the exact claimed job, school, initiator, worker and valid lease.
4. Keep platform-global or identity tables separately classified; do not apply a simplistic `school_id` policy where the model requires user-level or cross-entity validation.
5. Grant only the operations in the inventory. For job progress updates, prefer column-level grants and ensure the worker cannot modify tenant ownership, initiator, job kind, payload, or other immutable fields.
6. Confirm table ownership, FORCE ROW LEVEL SECURITY behavior, views, triggers, SECURITY DEFINER functions, and foreign-key side channels.
7. Prove the processing login cannot SET ROLE to API, queue-owner, service, migration, or owner identities.

The current catalog audit found six processing tables with RLS disabled and two with RLS enabled but no policies. The audit was performed on the approved read-only recovery project, not the production database. Re-run the catalog inventory against the intended isolated staging database before applying any changes.

## Startup guard added with this slice

`REQUIRE_BULK_IMPORT_PROCESS_ROLE=true` makes the worker reject a processing connection unless:

- the authenticated session is a LOGIN;
- current_user equals session_user;
- neither identity is superuser or BYPASSRLS;
- the session cannot create databases or roles;
- it cannot assume a superuser/BYPASSRLS role;
- it is not a member of `stonez_api_runtime`, `stonez_bulk_queue_owner`, or `service_role`.

This is an identity guard only. It does **not** prove the exact table grants, RLS policies, signed-context implementation, or row-level behavior. Those remain mandatory independent gates. Keep the flag unset/false until a dedicated processing identity is provisioned and the isolated PostgreSQL suite passes.

## Required isolated tests

Run using a dedicated PostgreSQL fixture and the exact processing login; never use the shared recovery audit project as a write fixture.

- Safe processing identity accepted; privileged, role-switchable, API, queue-owner and service-role identities rejected.
- No context, malformed assertion, expired assertion, replay, wrong worker, wrong school, wrong initiator and wrong job are denied.
- Same-school import succeeds for students, teachers and parents.
- Cross-school reads and writes are denied, including nested guardian and enrollment relationships.
- A worker cannot alter job tenant, initiator, payload, status outside the allowed transitions, or another worker's lease.
- Concurrent claim and processing attempts cannot both retain a valid lease.
- Lease expiry and replacement fence old workers.
- A row failure rolls back the complete row transaction; job-level progress/audit behavior remains consistent.
- Connection pool reuse does not retain tenant context or capability state.
- Claim connection has EXECUTE only for the claim operation and no direct queue DML.
- Processing connection has no direct claim-function execution unless explicitly justified.

## Non-goals and deployment guardrails

This slice must not:

- apply SQL to Supabase;
- alter role membership or grants;
- modify Render or GitHub secrets;
- enable the scheduled worker;
- change production credentials;
- merge PRs #357, #358, or #359;
- enable the processing-role guard in a deployed environment.

A passing Go unit test is not evidence that the PostgreSQL role, grants, policies, or transaction boundary are correct. The next gate is an isolated PostgreSQL integration fixture that creates the proposed identities and proves the full boundary end-to-end.

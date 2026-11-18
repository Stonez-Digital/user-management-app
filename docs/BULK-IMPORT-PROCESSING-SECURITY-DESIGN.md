# Bulk-import worker: processing identity and verifiable tenant context

## Status

**Design proposal only — not approved for database installation or deployment.**

This document defines the security boundary required before the bulk-import worker can use a least-privileged processing login. It does not provision roles, create policies, change credentials, or enable a production cutover.

The proposal builds on the queue claim boundary in the dependent draft pull requests. Keep PRs #357, #358, and #359 draft and unmerged until their independent gates are satisfied.

## 1. Verified starting point

The current worker has two database handles:

- The claim handle calls `stonez_internal.claim_next_bulk_import_job(uuid)`.
- The processing handle runs job validation, row imports, and progress updates.

The claim function is `SECURITY DEFINER`, uses a fixed `pg_catalog` search path, and is owned by the non-login queue-owner role. The current claim authorization nevertheless depends on effective membership in `stonez_api_runtime`. That group also has ordinary queue-table privileges and tenant-context policies. It is therefore not a claim-only authority boundary.

The processing handle still uses the existing application database identity. Its startup check warns when the session is privileged; it does not currently reject that identity by default.

The worker sets `app.current_school_id` and `app.current_user_id` with transaction-local `set_config`. This prevents stale session state from leaking across pooled transactions and helps prevent accidental unscoped queries. It does **not** authenticate those values: a SQL session using the same login can set them itself.

The claim function returns a full job row, so the worker must treat the returned tenant and initiator identifiers as data that requires validation before they become authorization context.

## 2. Target role separation

Use three distinct role responsibilities:

| Role responsibility | Login | Authority |
|---|---:|---|
| Queue function owner | No | Owns the narrowly scoped claim function; queue SELECT and only the required lease/claim-column UPDATE |
| Claim executor group | No | EXECUTE on the claim function only; no direct queue-table DML |
| Claim worker login | Yes | Inherits claim-executor EXECUTE only; cannot SET ROLE to the function owner or processing role |
| Processing group | No | Exact import-processing table/column privileges, constrained by tenant RLS |
| Processing worker login | Yes | Inherits processing-group permissions only; NOBYPASSRLS and no privileged memberships |

The claim login and processing login must be distinct database identities and distinct secrets. Neither may inherit `stonez_api_runtime`, `stonez_bulk_queue_owner`, `service_role`, or another privileged group. The HTTP API must not receive either worker credential.

A SECURITY DEFINER function may access queue rows as its owner. Its caller should not need direct SELECT, INSERT, UPDATE, DELETE, or TRUNCATE on the queue table. Function authorization must identify the dedicated claim login/group rather than relying on API-runtime membership.

The function must continue to validate its caller using the authenticated session identity, reject privileged logins, use a fixed search path, schema-qualify referenced objects, and expose only the minimum returned fields needed by the worker. If the worker needs the stored import payload, that payload must be returned only to the dedicated trusted claim identity.

## 3. Verifiable tenant context

### Problem

A policy based only on:

```sql
current_setting('app.current_school_id', true)
```

cannot distinguish a server-validated tenant context from a value supplied by arbitrary SQL using the same database login. Transaction-local settings solve connection-pool hygiene, not authenticity.

### Required assertion

Before any processing transaction reads or writes tenant rows, the worker must present a signed, short-lived assertion bound to the claimed job. At minimum, the signed claims should bind:

- job ID;
- school ID;
- initiating user ID;
- worker ID / lease owner;
- lease or claim generation (or another non-reusable claim identifier);
- issued-at and expiry timestamps;
- a unique assertion identifier.

The worker must derive these claims from the claimed job and independently validate the job lease before using them. Spreadsheet columns and other user-supplied import data must never determine tenant, initiator, or worker identity.

### Verification boundary

The database must verify the assertion cryptographically before tenant RLS policies trust its claims. A possible implementation is an HMAC assertion:

1. A trusted worker process signs the canonical claims using a dedicated secret available to that worker process.
2. The database verifies the signature through a narrowly scoped SECURITY DEFINER verifier using a secret inaccessible to the processing login.
3. The verifier checks expiry, claim shape, and the current job/lease tuple against the queue record.
4. Tenant policies consume only verifier-validated claims, not raw caller-controlled GUC values.
5. Verification fails closed on missing, malformed, expired, mismatched, or replayed assertions.

The exact mechanism is not selected yet. Before implementation, verify that the target PostgreSQL environment supports the required cryptographic primitive (for example, the approved `pgcrypto` functions), determine how the verification key is provisioned and rotated without exposing it to the processing login, and define assertion replay behavior. Do not assume an extension is installed or enable one as part of this design-only change.

A signed assertion protects against a database credential holder who can issue arbitrary SQL but does not possess the signing secret. It does not protect against compromise of the trusted worker process or its signing secret. That residual threat must be explicit.

### Lease binding and replay

The assertion must be tied to the current lease, not merely to a job ID and school ID. Reclaiming a job must invalidate assertions from the previous lease. A monotonic claim generation or fresh random lease nonce stored atomically at claim time is preferable to relying only on a timestamp. The verifier must compare the assertion's lease identifier with the current queue row while accounting for transaction concurrency.

The design must define whether a verified assertion is valid for one transaction only. Prefer transaction-scoped verification and context so a pooled connection cannot retain authority after commit or rollback.

## 4. Processing privilege inventory

The worker code review identified these operations. This is an application-code inventory, not a grant script:

| Table | Required operations |
|---|---|
| `users` | SELECT, INSERT |
| `students` | SELECT, INSERT |
| `teacher_profiles` | SELECT, INSERT |
| `parent_profiles` | SELECT, INSERT |
| `guardian_relationships` | SELECT, INSERT |
| `student_enrollments` | SELECT, INSERT |
| `academic_sessions` | SELECT |
| `school_classes` | SELECT |
| `sections` | SELECT |
| `bulk_import_jobs` | SELECT and narrowly scoped progress/lease UPDATE |

Before granting these privileges, inspect every actual GORM query, association write, hook, default, trigger, foreign-key check, and audit side effect. The inventory must include implicit SQL and any sequence privileges required by inserts. Remove any operation not needed by the worker.

For `bulk_import_jobs`, the processing login must not update tenant ownership, initiator, job kind, payload, or other immutable job fields. Separate the worker's progress fields from claim/lease fields and grant only the exact columns it needs. Lease ownership checks must remain part of each update predicate and transaction.

## 5. Tenant RLS requirements

Every tenant-owned table touched by the processing login must have an explicit classification and fail-closed policy before the login receives access.

The existing catalog review found important gaps in the approved audit database: several worker tables had RLS disabled, while `teacher_profiles` and `parent_profiles` had RLS enabled without policies. Those observations are environment-specific evidence, not proof of the live production catalog.

For each table:

- classify its tenant key or derive ownership through a trusted parent relationship;
- use both `USING` and `WITH CHECK` for applicable writes;
- reject missing or invalid verified context;
- prevent cross-school reads and writes;
- check user-level ownership where the operation requires an initiating user;
- consider composite foreign keys or equivalent constraints to prevent cross-tenant references;
- evaluate views, triggers, SECURITY DEFINER functions, and foreign-key side channels separately;
- apply FORCE ROW LEVEL SECURITY where appropriate after reviewing table ownership and maintenance paths.

Do not install broad permissive policies simply to make the import pass. Do not grant access before the policies are installed and tested.

## 6. Required isolated PostgreSQL tests

The CI integration environment must be able to create disposable roles and schemas. Tests must use actual PostgreSQL logins/role switching, not SQLite mocks, and must clean up all test resources.

### Role and function assertions

- Claim login can execute the claim function.
- Claim login cannot directly SELECT or mutate the queue table.
- Claim login cannot SET ROLE to queue owner, processing group, API runtime, or a privileged role.
- Unrelated logins and PUBLIC cannot execute the claim function.
- Function owner is NOLOGIN, NOSUPERUSER, NOBYPASSRLS, and has only required queue privileges.
- Processing login is LOGIN, NOSUPERUSER, NOBYPASSRLS, and has no privileged memberships.
- Processing login has no claim-function EXECUTE unless explicitly required and justified.

### Assertion verification

- Valid assertion for the current job and lease is accepted.
- Wrong school, initiator, worker, job, or lease generation is rejected.
- Missing, malformed, tampered, expired, and replayed assertions are rejected.
- A reclaimed job invalidates the prior worker's assertion.
- Processing login cannot read the signing/verifying secret or invoke a signing operation.
- Transaction commit/rollback and pooled-connection reuse do not preserve tenant authority.

### Tenant isolation and worker behavior

- School A can process School A records.
- School A cannot read, update, or reference School B records.
- Missing context denies access.
- Inserts with another school's tenant key are rejected.
- Worker lease loss prevents further row writes and progress updates.
- Concurrent claimers receive distinct jobs.
- Expired leases can be reclaimed; active leases cannot.
- A failed row transaction rolls back its database writes.
- Progress and final status updates cannot alter immutable job fields.
- Credential email failures are recorded without falsely reporting successful delivery.
- Tests cover all worker kinds: students, teachers, and parents.

CI must report a clear skip/failure if role-creation capability is absent. SQLite unit tests are useful for business logic but are not evidence of PostgreSQL role or RLS enforcement.

## 7. Implementation and rollout gates

1. **Code inventory:** inspect all worker SQL, model hooks, associations, triggers, and audit effects; finalize the exact privilege matrix.
2. **Claim-only boundary:** implement a dedicated claim executor group and login; remove API-runtime membership from claim authorization; prove no direct queue DML.
3. **Assertion design:** choose and threat-model the signed context format, secret storage, rotation, expiry, replay protection, and database verification mechanism.
4. **Verifier tests:** implement isolated PostgreSQL tests for valid, invalid, expired, forged, and stale-lease assertions.
5. **Tenant policy rollout:** add and test policies for every processing table before granting processing access.
6. **Processing identity:** provision a separate NOBYPASSRLS processing login with exact grants only in an approved isolated staging environment.
7. **Worker integration:** test the actual two-connection worker against isolated PostgreSQL with both restricted identities.
8. **Startup enforcement:** require the processing connection to be NOBYPASSRLS and reject privileged memberships; remove warning-only behavior for the dedicated worker after the staging credential has been proven.
9. **Operational readiness:** test rotation, rollback, observability, lease recovery, and secret isolation.
10. **Separate approval:** only after all evidence is reviewed should any staging installation or production cutover be proposed.

## 8. Explicit non-goals and safety constraints

This proposal does not:

- execute SQL against any Supabase project;
- create or alter database roles, grants, policies, functions, extensions, or secrets;
- change Render, GitHub Actions, or Supabase environment variables;
- enable a recurring worker schedule;
- deploy a background worker;
- change production credentials or enable `REQUIRE_NOBYPASSRLS`;
- approve or merge PRs #357, #358, or #359.

The current production application connection and worker processing connection must remain unchanged until a separately approved cutover plan has passed all required gates.

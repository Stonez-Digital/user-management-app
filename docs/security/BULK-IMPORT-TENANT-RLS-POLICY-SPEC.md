# Bulk-import tenant RLS policy specification

Status: **DRAFT — review required; not approved for migration or deployment**

Scope: the dedicated PostgreSQL bulk-import processing identity only. This document does not define the broader HTTP API tenant-RLS rollout.

## Security invariant

A processing transaction may read or write rows for exactly the school in the currently verified bulk-import context. The tenant identity must come from `stonez_internal.current_bulk_import_context()`, which binds the assertion to the current PostgreSQL backend, current transaction ID, expiry, job, initiating user, worker, and current lease generation. A caller-settable GUC by itself is not an authorization source.

If the function returns no context row, the policy predicate must evaluate to false. A missing, expired, replayed, transferred, or stale-lease assertion must therefore expose no tenant rows and permit no tenant writes.

## Context predicate

Candidate policy expressions should use the verified context function and match the row's `school_id`:

```sql
EXISTS (
    SELECT 1
      FROM stonez_internal.current_bulk_import_context() AS ctx
     WHERE ctx.school_id = <target_table>.school_id
)
```

This is a design expression only. Before use in migrations, reviewers must confirm function EXECUTE privileges, function ownership and `search_path`, RLS behavior inside the SECURITY DEFINER function, and the required SELECT privileges for any policy subqueries.

## Existing RLS baseline

The application migration history already enables RLS on `teacher_profiles` and `parent_profiles`. The candidate installer therefore permits RLS to be enabled already, but refuses any table that is already FORCE RLS. It does not remove or replace existing policies. Before operational use, a reviewed preflight manifest must record each table's original `relrowsecurity` and `relforcerowsecurity` flags so a rollback can restore the exact prior state.

## Table-by-table policy intent

| Table | SELECT | INSERT | UPDATE | DELETE | Policy predicate / review notes |
| --- | --- | --- | --- | --- | --- |
| `users` | Yes | Yes | No | No | School must match verified context. Existing lookup by lower-cased email remains RLS-filtered; never add cross-school visibility to handle global email uniqueness. |
| `students` | Yes | Yes | No | No | School must match context. Review that `user_id` references a user from the same school; prefer a composite database constraint where supported. |
| `teacher_profiles` | Yes | Yes | No | No | School must match context. Review same-school `user_id` relationship. |
| `parent_profiles` | Yes | Yes | No | No | School must match context. Review same-school `user_id` relationship. |
| `academic_sessions` | Yes | No | No | No | School must match context. |
| `school_classes` | Yes | No | No | No | School must match context. |
| `sections` | Yes | No | No | No | School must match context. Review that `class_id` belongs to the same school. |
| `student_enrollments` | Yes | Yes | No | No | School must match context. Review that student, academic session, class, and section all belong to the same school. |
| `guardian_relationships` | Yes | Yes | No | No | School must match context. Review that guardian user and student both belong to the same school. |

The processing role should receive no direct DELETE privilege and no UPDATE privilege on these nine tenant-content tables. The worker currently needs only SELECT/INSERT for the content tables. Queue progress UPDATE remains separately constrained by the signed job/worker/lease context and the queue policy in the signed-context installer.

## Policy command shape for review

For each table with SELECT intent, reviewers should consider a policy scoped to `stonez_bulk_process_group`:

- `FOR SELECT USING (verified school predicate)`
- `FOR INSERT WITH CHECK (verified school predicate)` only for the six write tables above.

Do not create permissive `FOR ALL` policies. Do not grant the processing role table ownership, BYPASSRLS, or direct access to `stonez_internal.bulk_context_keys` or `bulk_context_sessions`.

RLS should be enabled and forced for these tables when the migration owner/runtime ownership model has been validated. The migration/owner identity must remain separate from the processing login; FORCE RLS must not be used as a substitute for proving the correct owner and function execution behavior.

## Required PostgreSQL 17 acceptance tests

1. Apply `database.Migrate` twice in a disposable PostgreSQL 17 schema and inspect the actual table/column catalog.
2. Verify generated GORM SELECT, INSERT, and queue UPDATE SQL against the candidate column-privilege test.
3. Install the reviewed policies only in the disposable test schema and run the real worker processing path with a NOBYPASSRLS processing login.
4. Seed two schools with overlapping natural keys where the schema permits it; verify school A cannot SELECT, INSERT, UPDATE, or DELETE school B rows.
5. Verify a forged tenant GUC without a signed assertion returns no context and grants no row access.
6. Verify missing, expired, replayed, transaction-transferred, and stale-lease assertions fail closed.
7. Verify same-school student, teacher, parent, enrollment, and guardian operations succeed; cross-school relationship references fail.
8. Verify the processing role cannot read `users.password_hash`, queue payload/error columns, or private signing-key/assertion tables.
9. Verify the claim-only login can execute only the queue claim function and cannot directly read or mutate queue rows.
10. Run race detection and the existing worker PostgreSQL 17 integration suite.

## Open decisions required before approval

- Confirm actual composite foreign-key coverage for every cross-table school relationship listed above. Migrations 18 and 48 cover the currently identified import relationships; retain catalog assertions so future schema changes cannot silently remove those constraints.
- Confirm whether `users.email` remains globally unique. If so, make duplicate-collision responses generic so a hidden cross-school account cannot be used as an existence oracle.
- Confirm the policy function's behavior when called from RLS expressions under the restricted processing role, including function owner privileges and nested row-security semantics.
- Confirm the processing role's direct SELECT grants include all columns needed by RLS policy evaluation and foreign-key checks, but no unrelated personal or credential data.
- Confirm FORCE RLS compatibility with the migration owner and the SECURITY DEFINER verifier/current-context functions.

## Safety boundary

This specification is not an approved migration, grant script, or production rollout instruction. No policies or grants are created by this document. Production database state, credentials, secrets, and deployments remain untouched.

## Source-verified schema findings (2026-11-24)

Reviewed against `internal/database/postgres.go` migrations 16–24 and the current GORM model definitions. These findings narrow the open decisions above:

- **Global email uniqueness is present in the model.** `models.User.Email` carries GORM `uniqueIndex`; migration 16 deliberately avoids rebuilding the existing PostgreSQL users table because older databases may have a legacy email constraint/index. The worker's email lookup must remain tenant-filtered by RLS, and a hidden global-unique collision must be handled without disclosing another school's account.
- **Composite school-consistency constraints exist for several import relationships.** Migration 18 creates composite foreign keys for `students(school_id,user_id)`, `sections(school_id,class_id)`, all four `student_enrollments` references (student, session, class, section), and both `guardian_relationships` references (guardian user and student). These are stronger than application-only checks and should be asserted by catalog tests.
- **Teacher and parent profile user relationships were missing from migration 18's composite-FK inventory.** `teacher_profiles` and `parent_profiles` are introduced in migration 23, while migration 18's composite parent-table and constraint lists omit both profile tables. This branch now adds migration 48, `enforce_profile_user_school_consistency`, with preflight checks and composite `(school_id,user_id) -> users(school_id,id)` foreign keys for both tables. PostgreSQL 17 tests verify same-school inserts succeed, cross-school inserts fail, and migration preflight fails closed on mismatched legacy rows. The production read-only preflight found zero mismatches (68 teacher profiles and 63 parent profiles), but migration 48 has not been applied in production. This closes the schema-design gap on this branch; it does not by itself approve tenant INSERT policies.
- **Platform users are intentionally outside school tenancy.** Migration 21 sets `school_id = NULL` for `super_admin` users. A school-scoped `users` SELECT/INSERT policy must not grant access to platform users; worker initiator validation must require a non-null school ID equal to the verified context.
- **RLS policy evaluation has disposable-test coverage, not production approval.** `TestBulkImportWorkerTwoSchoolRLSAcceptance` runs the real signed-context installer, forces RLS on `bulk_import_jobs` and all nine tenant tables, and exercises the SECURITY DEFINER context helper under the restricted login. This closes the previously identified test gap for the current candidate policy shape; it does not approve a production policy migration or prove behavior under future schema/policy changes.

Required follow-up before policy approval:
1. Keep the actual-migration catalog test asserting the existing composite FKs and the migration-48 profile constraints, including same-school success and cross-school rejection.
2. Before scheduling migration 48, retain the reviewed production preflight evidence and apply it only in an explicitly approved low-traffic migration window with a finite lock timeout; verify both constraints are validated afterward.
3. The restricted-role global-email collision path is now covered by `TestBulkImportWorkerTwoSchoolRLSAcceptance`: School A cannot see School B's matching email, the global unique-index collision maps to the generic row error, and no partial School A student row is created. Keep this regression test as a required gate.
4. Verified-context policy evaluation with the actual signed-context function owner and FORCE RLS is now exercised in `TestBulkImportWorkerTwoSchoolRLSAcceptance`, which forces RLS on the queue and all nine tenant tables in disposable PostgreSQL 17. Keep this regression test as a required gate.

### Global email collision handling added to the worker

The createUser path no longer returns an explicit "belongs to another tenant" message. If RLS hides a School B account from School A and the attempted School A insert hits the global unique email index, the database error is mapped to the generic message "account could not be created; verify the email and try again". The same generic response is used for a visible same-school email that cannot be reused because of its role, avoiding a role/account detail leak. The lookup remains school-context filtered; no cross-school SELECT grant or bypass was added.

TestBulkImportWorkerTwoSchoolRLSAcceptance seeds a School B account, confirms the restricted School A login sees zero matching email rows under its signed context, then exercises the real student row processor. It asserts that the global unique-index collision returns only the generic message, contains no email/school/constraint detail, and creates no partial School A student record. This test runs against the actual migrations and test-only candidate RLS/grants in disposable PostgreSQL 17.

These are source-review findings only. No migration, grant, policy, production database, or deployment was changed.

### Disposable PostgreSQL 17 regression coverage added

`TestTeacherAndParentProfilesRequireCompositeSchoolUserForeignKeys` now applies the actual migration runner in a disposable PostgreSQL 17 schema and verifies migration 48 installs the composite `(school_id,user_id) -> users(school_id,id)` constraints on both profile tables. It proves same-school inserts succeed and cross-school inserts fail. The migration is present on this branch and included in the passing CI head; it has not been applied to production.


### Profile relationship migration candidate (migration 48)

Migration 48 adds two database-enforced composite foreign keys: `teacher_profiles(school_id,user_id) -> users(school_id,id)` and `parent_profiles(school_id,user_id) -> users(school_id,id)`. Before adding either constraint it counts mismatched or unassigned user-school relationships and fails with a table-specific error if any exist. It relies on the `(school_id,id)` unique indexes established by migration 18. PostgreSQL 17 integration coverage validates the constraints installed by the real migration runner and verifies same-school inserts succeed while cross-school inserts fail. Production has not been migrated.

## Policy-composition requirement from security review

PostgreSQL combines applicable **permissive** policies with OR and applicable **restrictive** policies with AND. Therefore, a tenant predicate placed only in a permissive SELECT or INSERT policy is not a durable boundary: a later or pre-existing broad permissive policy (for example, one using `USING (true)`) can make rows visible outside the signed school context.

Before approval, the implementation must use one of these explicitly reviewed controls:

1. Install a tenant-context **restrictive** policy for the processing role on every protected tenant table. Its `USING` predicate must require a non-null `school_id` matching `current_bulk_import_context()`; its `WITH CHECK` predicate must enforce the same condition for writes. Keep separate permissive policies limited to the required SELECT and INSERT commands; or
2. Prove, with a migration-time fail-closed catalog assertion and a continuing regression test, that no other applicable permissive policy can broaden access for the processing role. Any later policy addition must preserve that invariant.

The first option is preferred because it composes safely with future permissive policies. For `users`, the restrictive predicate must additionally reject `school_id IS NULL` so platform/super-admin accounts remain outside school processing. The test suite must deliberately add a broad permissive policy in the disposable PostgreSQL 17 schema and prove the restrictive guard still prevents School A from reading or inserting School B rows. It must also verify that no context, expired context, forged GUC, and stale lease remain denied under the composed policy set.

The current acceptance test installs restrictive guards on all nine tenant tables and deliberately adds a broad permissive policy on `users`; it verifies cross-school SELECT denial across all nine tables and cross-school INSERT denial on all six writable tables (`users`, `students`, `teacher_profiles`, `parent_profiles`, `student_enrollments`, and `guardian_relationships`). The four additional negative INSERT cases seed valid School B reference rows through the privileged fixture connection, then assert that School A's restricted processing context is rejected specifically by the RLS `WITH CHECK` policy—not by a foreign-key failure. All candidate grants and policies remain disposable-test fixtures only. No production policy or grant was changed.


## Candidate installer/verifier implementation (2026-11-25)

A separate draft implementation now exists in:

- `scripts/security/install_bulk_import_tenant_rls.sql`
- `scripts/security/verify_bulk_import_tenant_rls.sql`

The installer is designed to run only after the signed-context installer and role preparation. It validates the processing-group role, the verified-context function, all nine expected tables, each `school_id` column, and separation between the processing role and table ownership. It refuses to proceed if the dedicated processing group already has direct table/column ACLs on any of the nine tables; it does not reset or revoke existing ACLs. After that clean-baseline check, it grants the reviewed SELECT columns and GORM INSERT columns for the six writable tables, enables RLS without forcing it, preserving the existing application table-owner access model while keeping the separate non-owner processing role subject to RLS; it installs separate permissive SELECT/INSERT policies plus a restrictive `FOR ALL` guard on every table. The preflight rejects direct `PUBLIC` table or column ACLs because those grants are effective for every role. The `users` guard explicitly rejects NULL `school_id`.

The verifier is read-only and checks enabled-but-not-forced RLS, policy inventory/composition, context-function access, and the absence of UPDATE/DELETE and broad table-level INSERT privileges. PostgreSQL 17 acceptance coverage proves the installer rejects a pre-existing `PUBLIC` grant, verifies the application runtime retains table-owner visibility after RLS is enabled, invokes the actual installer twice, adds a deliberately broad permissive `users` policy, invokes the verifier, and checks the generated GORM INSERT columns against granted column privileges before exercising the existing two-school worker scenarios.

This is a code-review candidate only. The scripts have not been run against the Godswill database, any staging database, or production. Do not execute them operationally until the draft PR has passed CI and received independent SQL/security review. The current installer intentionally has no automatic rollback script; a rollback procedure must be separately reviewed so it cannot remove pre-existing policies or grants.

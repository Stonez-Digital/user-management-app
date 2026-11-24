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

- Confirm actual composite foreign-key coverage for every cross-table school relationship listed above. Add constraints or explicit policy checks where the database does not already enforce school consistency.
- Confirm whether `users.email` remains globally unique. If so, make duplicate-collision responses generic so a hidden cross-school account cannot be used as an existence oracle.
- Confirm the policy function's behavior when called from RLS expressions under the restricted processing role, including function owner privileges and nested row-security semantics.
- Confirm the processing role's direct SELECT grants include all columns needed by RLS policy evaluation and foreign-key checks, but no unrelated personal or credential data.
- Confirm FORCE RLS compatibility with the migration owner and the SECURITY DEFINER verifier/current-context functions.

## Safety boundary

This specification is not an approved migration, grant script, or production rollout instruction. No policies or grants are created by this document. Production database state, credentials, secrets, and deployments remain untouched.

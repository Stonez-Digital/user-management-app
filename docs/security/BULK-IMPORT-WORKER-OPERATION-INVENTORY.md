# Bulk-import worker database operation inventory

Status: source-derived implementation inventory, not an approved privilege or RLS policy specification.

Reviewed against:
- `internal/service/bulk_import_worker.go`
- `internal/service/bulk_import_service.go`
- `internal/models/bulk_import.go`
- the signed-context and queue-claim SQL installers in `scripts/security/`

This inventory exists to prevent guessed grants. It records the tables touched by the current worker code; it does not provision roles, grants, policies, or credentials.

## Connection boundaries

| Connection | Current operation | Intended capability |
| --- | --- | --- |
| Claim connection | Calls only `stonez_internal.claim_next_bulk_import_job(uuid)` inside a short transaction | Execute the narrowly scoped queue-claim function; no direct queue-table access is needed by the Go claim path |
| Processing connection | Verifies signed context and processes the claimed job in tenant-bound transactions | Read/write only the tenant-owned tables and columns listed below, subject to actual tenant RLS policies |
| API connection | Creates, previews, starts, lists, and reads import jobs | Separate API capability; not interchangeable with either worker connection |

The PostgreSQL claim path uses the SECURITY DEFINER function. The SQLite direct-claim implementation is test-only and is not a production privilege requirement.

## Processing table inventory

| Table (GORM model) | Operations in worker path | Scope and notes |
| --- | --- | --- |
| `bulk_import_jobs` (`BulkImportJob`) | Receives the claimed job and payload from the claim function; the processing connection reads only the `id`, `school_id`, `initiated_by`, and `worker_id` columns needed by its guarded progress UPDATE predicate, then updates progress/status/error fields | Processing updates are constrained by job ID, school ID, initiating user, and worker ID. The claim function owns queue selection/lease mutation. Do not grant processing login table-level SELECT or access to `rows_json`. |
| `users` (`User`) | Reads the initiating user; looks up an existing user by email; inserts a user when needed | Reads and writes must remain tenant-scoped by the verified school context. The email lookup predicate itself is global, but RLS must prevent cross-school row visibility; do not add a cross-tenant SELECT policy to preserve the duplicate-email message. A global unique constraint may still reject a duplicate hidden by RLS. |
| `students` (`Student`) | Checks admission-number duplicate; inserts student | School-scoped read/insert. |
| `teacher_profiles` (`TeacherProfile`) | Checks staff-ID duplicate; inserts teacher profile | School-scoped read/insert. |
| `parent_profiles` (`ParentProfile`) | Looks up parent identifier/profile; inserts parent profile | School-scoped read/insert. |
| `academic_sessions` (`AcademicSession`) | Resolves requested session and checks status | School-scoped SELECT only. |
| `school_classes` (`SchoolClass`) | Resolves requested class by name | School-scoped SELECT only. Confirm the physical table name from the model/migration catalog before writing any SQL grant. |
| `sections` (`Section`) | Resolves section by class and name | School-scoped SELECT only. |
| `student_enrollments` (`StudentEnrollment`) | Checks existing and conflicting active enrollment; inserts enrollment | School-scoped SELECT/INSERT. |
| `guardian_relationships` (`GuardianRelationship`) | Checks existing guardian/student link; inserts relationship | School-scoped SELECT/INSERT. |

The table list above is conditional on the corresponding row type being processed. Student imports may use academic session/class/section/enrollment tables; teacher imports use teacher profiles; parent imports use parent profiles, students, and guardian relationships.

## Important non-table operations

- The signed-context verifier uses private objects in schema `stonez_internal`; the processing login must not receive direct SELECT on signing-key or assertion tables.
- The worker sends credentials through the configured mailer. Email delivery is not a database grant and is not transactionally exactly-once.
- The worker does not need to create schools, change school ownership, manage platform records, or perform migrations.
- The claim login and processing login must be different authenticated PostgreSQL roles.

## Catalog and policy gate

The prior actual-migration catalog baseline (PR #368) found that the application migration path enabled RLS on only a subset of application tables, installed zero RLS policies, and did not FORCE RLS. Therefore this source inventory is not evidence that any listed table is currently usable by a NOBYPASSRLS processing login.

Before a processing role is granted table access:
1. Confirm every physical table name and column list from the actual migration catalog.
2. Confirm RLS is enabled and forced where required, and that tenant policies are installed from a reviewed policy specification.
3. Derive least-privilege table/column grants from the exact statements above and test both allowed and denied operations with the actual restricted login.
4. Test two schools with identical natural keys and ensure no cross-school reads, writes, updates, or deletes.
5. Verify that the claim login can execute the claim function but cannot directly read or mutate queue rows.
6. Run the tests against disposable PostgreSQL 17 using the actual application migrations, not synthetic replacement policies.

No production database state, credentials, roles, grants, policies, secrets, or deployments are changed by this inventory.

## Current test evidence boundary

The PostgreSQL claim-function test in `internal/service/bulk_import_worker_postgres_test.go` creates a private schema, applies migrations, then installs its own queue-table grants and policies for the test roles. This is useful evidence for the isolated claim-function boundary; it is not a test of the processing login against tenant policies installed by the real application migrations.

The signed-context PostgreSQL integration tests exercise the verifier and transaction-bound assertion under a restricted test login, but they do not establish that the processing login has the required least-privilege grants and tenant policies on all import target tables listed above.

Accordingly, current green CI does not clear the worker table-access gate. The next database test must apply the real application migrations, install only the reviewed candidate policy/grant set in a disposable PostgreSQL 17 database, and run actual student, teacher, and parent worker flows with two schools using the restricted processing login. Until that exists, keep the processing role unprovisioned for production and do not enable the worker against production.

## Queue payload confidentiality correction

The signed-context installer now explicitly revokes table-level SELECT and all known column-level SELECT privileges from `stonez_bulk_process_group`, then grants SELECT only on `id`, `school_id`, `initiated_by`, and `worker_id`. The claim-only connection receives the full claimed job through the SECURITY DEFINER claim function; the processing connection does not need to read `rows_json`. The verification SQL rejects table-level SELECT and SELECT on payload/metadata columns, and the restricted-login PostgreSQL test asserts that reading `rows_json` fails while the four predicate columns remain readable.

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

## Statement-level review of the processing path

Reviewed directly against the worker/service source and the GORM model definitions on this branch. This section records the operations that the code actually issues; it is still not a grant script or policy approval.

| Physical table | Processing-path statements | Required operation class | Important boundary |
| --- | --- | --- | --- |
| `bulk_import_jobs` | Reads the four predicate columns for guarded progress updates; updates progress, status, error, worker and lease timestamp fields | SELECT on `id`, `school_id`, `initiated_by`, `worker_id`; UPDATE on the exact progress fields | No processing SELECT on `rows_json`, `errors_json`, `kind`, `file_name`, or lease-generation/claim-selection fields. Payload arrives only from the claim function. |
| `users` | Reads the job initiator by `id + school_id`; looks up an existing account by lower-cased email; parent path also reads a user by ID; inserts new accounts | SELECT and INSERT | The current `createUser` lookup is `LOWER(email) = ?` without an explicit `school_id` predicate. It must rely on the tenant RLS policy to hide other schools. Do not solve this by adding a cross-school SELECT policy. The global unique email constraint may still reject an insert for an email hidden by RLS. |
| `students` | Checks admission number; inserts student; parent import resolves student by school and admission number | SELECT and INSERT | No worker UPDATE or DELETE. |
| `teacher_profiles` | Checks school + staff ID; inserts teacher profile | SELECT and INSERT | No worker UPDATE or DELETE. |
| `parent_profiles` | Looks up identifier, profile by user + school, and conflicting identifier; inserts parent profile | SELECT and INSERT | No worker UPDATE or DELETE. |
| `academic_sessions` | Resolves school + name and reads status | SELECT only | No worker write. |
| `school_classes` | Resolves school + case-insensitive name | SELECT only | Physical table name confirmed by `SchoolClass` model and migration inventory. |
| `sections` | Resolves school + class + name | SELECT only | No worker write. |
| `student_enrollments` | Checks existing enrollment and active enrollment in another session; inserts enrollment | SELECT and INSERT | No worker UPDATE or DELETE. |
| `guardian_relationships` | Checks school + guardian + student link; inserts relationship | SELECT and INSERT | No worker UPDATE or DELETE. |

### GORM projection and privilege implications

The source audit identified that unprojected `First(&model)` calls would hydrate unnecessary fields (including `users.password_hash`). The worker lookup queries have now been narrowed on this branch: user reads select only `id`, `school_id`, `email`, and `role`; the initiating-user membership check selects only `id`; profile, student, class, section, enrollment, and guardian-link existence checks select only their required identity columns; academic-session lookup selects only `id` and `status`. This reduces the eventual column-level SELECT grant surface. The actual generated SQL and privilege set still need PostgreSQL 17 integration verification; do not grant broad SELECT to bypass that gate.

The processing path has no direct DELETE statements against the ten listed tables. Student/teacher/parent/profile/enrollment/link operations are SELECT/INSERT; queue progress is the only UPDATE path. The separate claim function owns queue selection, lease assignment, attempts, and lease generation.

### Open policy blockers (not solved by this source audit)

1. The current migrations' zero-policy catalog result means the worker cannot safely be enabled under a NOBYPASSRLS login yet. RLS enabled without a matching policy is not tenant access.
2. Every target table needs a reviewed school predicate policy, and tests must prove both visibility and write checks for two schools. The policy must use the verified transaction-bound school context, not a caller-settable GUC.
3. `users` requires particular care because the model has a globally unique email and the current account lookup is email-only. Keep other schools invisible; decide separately how to handle a hidden global-unique collision without leaking whether the other school's account exists.
4. Before column-level grants are finalized, narrow the worker's unprojected `First` queries and capture the resulting SQL/column set in PostgreSQL 17 integration tests. Do not treat this document as authorization to install any role, grant, or policy.

No production database objects, credentials, roles, grants, policies, secrets, or deployments are changed by this audit.

## Candidate column sets for PostgreSQL privilege tests

The following column sets are derived from the current worker's explicit projections, predicates, and GORM create/update models. They are candidate test inputs, not approved grants. The PostgreSQL 17 tests must capture the actual generated SQL and verify each privilege with `has_column_privilege` before any provisioning script is authored.

| Table | Candidate SELECT columns (projection plus predicate/RLS context) | Candidate INSERT columns | Candidate UPDATE columns |
| --- | --- | --- | --- |
| `bulk_import_jobs` | `id, school_id, initiated_by, worker_id` | None | `created, skipped, failed, credentials_sent, credential_emails_failed, errors_json, claimed_at, status, worker_id, last_error` |
| `users` | `id, school_id, email, role` | `id, school_id, name, email, password_hash, role, active, created_at, updated_at` (confirm generated SQL; timestamps/defaults may be dialect/model dependent) | None |
| `students` | `id, school_id, admission_number` | `id, school_id, user_id, admission_number, enrollment_status, guardian_name, guardian_phone, guardian_email, gender, created_at, updated_at` (confirm generated SQL) | None |
| `teacher_profiles` | `id, school_id, staff_id` | `id, school_id, user_id, staff_id, phone, gender, department, designation, subjects, classes, status, created_at, updated_at` (confirm generated SQL) | None |
| `parent_profiles` | `id, school_id, parent_identifier, user_id` | `id, school_id, user_id, parent_identifier, phone, address, occupation, created_at, updated_at` (confirm generated SQL) | None |
| `academic_sessions` | `id, school_id, name, status` | None | None |
| `school_classes` | `id, school_id, name` | None | None |
| `sections` | `id, school_id, class_id, name` | None | None |
| `student_enrollments` | `id, school_id, student_id, academic_session_id, status` | `id, school_id, student_id, academic_session_id, class_id, section_id, status, enrolled_at, created_at, updated_at` (confirm generated SQL) | None |
| `guardian_relationships` | `id, school_id, guardian_user_id, student_id` | `id, school_id, guardian_user_id, student_id, relationship, primary, active, created_at, updated_at` (confirm generated SQL) | None |

Important details:
- PostgreSQL column privileges must cover columns read by predicates as well as columns returned by the projection. The table above includes both.
- The worker performs no direct DELETE on these tables and no UPDATE on them other than the queue progress columns listed above.
- `users.password_hash` is intentionally absent from the candidate SELECT set. User creation still needs INSERT on `password_hash` to persist the generated password hash; the worker does not need to read it.
- These candidate sets do not grant access to private `stonez_internal` signing-key/assertion tables. Those remain function-mediated.
- Do not convert these candidates into SQL grants until generated-SQL and two-school RLS tests pass on disposable PostgreSQL 17 with the actual migrations.

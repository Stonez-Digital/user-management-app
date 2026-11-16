# Tenant route and RLS readiness audit

Audit date: 2026-11-16
Scope: repository branch `security/bulk-import-queue-role-boundary` and read-only catalog inspection of staging Supabase project `school-media-recovery-test` (`upschwxqagjhmextcxyp`).

This is a preparation audit. It does not authorize production DDL, credential changes, Render changes, or a runtime cutover.

## Executive result

The bulk-import claim boundary is installed on staging, but the application-wide restricted API runtime is not ready.

- The application API still uses the PostgreSQL `postgres` login with `BYPASSRLS`.
- The route transaction middleware is deliberately opt-in. Every school-scoped route must use `TenantTransaction(db)` and every controller/service/repository/audit query in that request must use `TenantDBFromContext(c)`.
- The staging project currently has 42 public tables in the catalog snapshot. 27 have RLS disabled; 15 have RLS enabled. Of those 15 enabled tables, 14 have no policies. Only `bulk_import_jobs` has policies (the four expected queue/tenant policies). No inspected table has FORCE ROW LEVEL SECURITY.
- All inspected public application tables are owned by `postgres`.
- `stonez_api_runtime` is NOLOGIN/NOBYPASSRLS and has no login member. Its only observed application-table grants are on `bulk_import_jobs`. The worker login inherits this group with INHERIT=true, SET=false, ADMIN=false.
- The repository production posture document reports 45 public application tables. The 42-table staging catalog is not evidence of production schema parity; production needs its own read-only catalog comparison before any cutover.

## Route inventory

### Already using request-scoped tenant transactions

The inspected route wiring includes transaction middleware for:
- Student CRUD.
- Academic sessions, terms, academic context and readiness.
- Classes, sections, subjects and class-subject mappings.
- Teacher assignments and teacher workspace assignment/attendance/assessment/result/timetable routes.
- Enrollments.
- Attendance, assessments, assessment results and report cards.
- Fees, invoices and invoice payments.
- Bulk-import preview/start/list/get.
- Guardian links and parent portal child/attendance/report-card/timetable/invoice/payment reads.
- Student portal profile, enrollment, terms, attendance, timetable, report cards, invoices and payments.
- School dashboard.
- Person onboarding and school user CRUD.
- Role assignment and user activation/deactivation.
- Announcements and parent notifications.

This is route-level evidence only. The associated services, repositories, nested transactions, external side effects and audit writes still need code review and PostgreSQL integration tests under the exact restricted login.

### Tenant-scoped routes still missing request transaction wiring

The current route wiring shows these school/user-owned paths without `TenantTransaction(db)`:

- `GET/PUT /me`: user profile reads and updates. Must be classified for school members versus platform administrators.
- `GET /admin/school`, `PUT /admin/school`, `POST /admin/school/logo`: school profile and logo metadata.
- Billing: `GET /admin/billing/subscription`, `POST /admin/billing/checkout`, `POST /admin/billing/verify`. `GET /admin/billing/plans` is global plan catalogue data. Payment-provider calls must not be held inside a long database transaction; use a short transaction for state changes and explicit idempotency/reconciliation.
- Audit: `GET /admin/audit-logs`.
- School content: admin blog, gallery, advertisement and enquiry management routes, including media upload/delete metadata. Storage operations are external side effects and need an explicit compensating-action design around database transactions.
- Student CareerPath attempt routes were previously missing transaction wiring. They have now been changed in this branch to require a request transaction and use a request-scoped service. The static assessment catalogue endpoint remains database-free.

### Routes that need explicit non-tenant access models

These must not be blindly wrapped in school tenant middleware:
- Login, refresh, logout, forgot/reset password, password change, session list/revoke and logout-all. These are identity/session/token operations and need a dedicated user-identity transaction/access model.
- Platform super-admin routes for national curriculum management, school provisioning/management, monitoring and domain reconciliation. These are platform-global and need narrowly scoped platform permissions and database access.
- Public school pages, public blog/gallery reads and public enquiries. These require explicit safe public projections and bounded write permissions; they must not expose unrestricted school tables.
- Global national curriculum catalogue reads and role catalogue reads require a deliberate global-read policy/grant or a controlled backend service path.

## Staging RLS catalog snapshot

### RLS disabled (27)

`academic_sessions`, `announcements`, `assessment_results`, `assessments`, `attendance_records`, `audit_logs`, `career_path_attempts`, `fee_items`, `guardian_relationships`, `invoice_lines`, `invoices`, `migrations`, `notifications`, `password_reset_tokens`, `payments`, `refresh_tokens`, `role_change_audits`, `school_classes`, `sections`, `sessions`, `student_enrollments`, `students`, `subjects`, `teacher_assignments`, `terms`, `timetable_entries`, `users`.

### RLS enabled but no policies (14)

`blog_categories`, `blog_posts`, `class_subjects`, `gallery_albums`, `gallery_images`, `national_curricula`, `national_curriculum_classes`, `national_curriculum_levels`, `national_curriculum_subjects`, `parent_profiles`, `school_advertisements`, `school_enquiries`, `schools`, `teacher_profiles`.

### RLS enabled with policies

`bulk_import_jobs` only:
- `bulk_import_jobs_queue_owner_all`
- `bulk_import_jobs_tenant_select`
- `bulk_import_jobs_tenant_insert`
- `bulk_import_jobs_tenant_update`

All 42 inspected public tables were owned by `postgres`; none had FORCE RLS in the snapshot.

## Required remaining work

1. Finish tenant transaction migration for the missing school/user-owned routes above, with tests proving every service/repository/audit query uses the transaction handle.
2. Build a complete table-to-route/service ownership matrix for every application table, including indirect child ownership and cross-school foreign-key consistency.
3. Classify each table as tenant-scoped, user-scoped, platform-global, public projection, queue/system, migration-only, or token/session.
4. Write and review policies and grants for the exact restricted API role. Include SELECT policies for UPDATE paths, both USING and WITH CHECK, no-context denial, user-specific rules, and explicit public/platform exceptions.
5. Review views, SECURITY DEFINER functions, triggers, storage policies, sequences, and default privileges separately. Ensure no accidental PUBLIC execution or broad table grants.
6. Add PostgreSQL integration tests using the actual restricted runtime login and at least two schools. Test SELECT/INSERT/UPDATE/DELETE, no tenant context, pooled-connection context reset, rollback, cross-tenant foreign keys, user/session operations, public reads, platform-admin operations and background workers.
7. Compare staging and production schemas/catalogs read-only. Resolve the 42-versus-45 table discrepancy and verify production policy/role state independently.
8. Only after every gate passes, plan a separately approved cutover with a restricted runtime login, separate migration/bootstrap credentials, `REQUIRE_NOBYPASSRLS=true`, staging smoke tests, production smoke tests and a tested rollback path.

## Trust-boundary warning

The current `app.current_school_id` and `app.current_user_id` values are custom PostgreSQL settings set by trusted application code. They are transaction-local and prevent accidental cross-request context leakage, but a compromised runtime SQL credential can set them itself. They are not cryptographically authenticated tenant claims and do not alone protect against arbitrary SQL execution or SQL injection.

## Safety

No production database was queried or changed as part of this audit. No production role, policy, grant, secret, Render setting or deployment was changed. The staging catalog queries were read-only.

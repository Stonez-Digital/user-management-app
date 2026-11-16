# Tenant route and RLS readiness audit

Audit date: 2026-11-16
Scope: repository branch `security/bulk-import-queue-role-boundary` and read-only catalog inspection of staging Supabase project `school-media-recovery-test` (`upschwxqagjhmextcxyp`).

This is a preparation audit. It does not authorize production DDL, credential changes, Render changes, or a runtime cutover.

## Executive result

The bulk-import claim boundary is installed on staging, and several school-owned route families now use request-scoped transactions. The application-wide restricted API runtime is not ready.

- The application API still uses the PostgreSQL `postgres` login with `BYPASSRLS`.
- The route transaction middleware is deliberately opt-in. Every school-scoped route must use `TenantTransaction(db)` and every controller/service/repository/audit query in that request must use `TenantDBFromContext(c)`.
- School profile/logo, audit-log, and protected school-content/media admin handlers have now been migrated to request-scoped transactions on this branch. External object-storage mutations are deferred until commit, with upload compensation on rollback.
- The initial staging snapshot had 42 public tables. The two reviewed subscription schema migrations and billing RLS installer have since been applied to staging only. The four billing tables now have FORCE RLS and nine narrowly scoped policies. The original 42-table inventory predates these additions; refresh the full inventory before relying on its table count. The remaining original 27-RLS-disabled / 14-policyless counts are historical baseline figures, not a current complete catalog count.
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
- School profile reads/updates and logo metadata: `GET/PUT /admin/school`, `POST /admin/school/logo`.
- Audit-log reads: `GET /admin/audit-logs`.
- Protected admin blog, gallery, advertisement and enquiry management routes, including media upload/delete metadata.
- Student CareerPath attempt routes; the static assessment catalogue endpoint remains database-free.

The school profile/audit and content/media migrations bind service/database operations to the request transaction and include fail-closed handler tests. Content/media handling defers object deletion until after DB commit and registers cleanup for newly uploaded objects if the DB transaction rolls back. Cleanup callback errors are logged; they still require operational monitoring and reconciliation.

This is route-level and unit-test evidence only. The associated services, repositories, nested transactions, external side effects and audit writes still need PostgreSQL integration tests under the exact restricted login. The migrations do not prove that RLS policies or restricted-role grants are complete.

### Tenant-scoped routes still missing request transaction wiring

The current route wiring still needs migration/review for:
- `GET/PUT /me`: user profile reads and updates. Must be classified for school members versus platform administrators.
- Billing: `GET /admin/billing/subscription`, `POST /admin/billing/checkout`, and `POST /admin/billing/verify` do not use route-level `TenantTransaction(db)` middleware. However, the inspected billing service explicitly binds authenticated school/user context with `WithTenantContext` and opens its own short `tenantTransaction` around each DB phase, setting transaction-local GUCs. Provider initialization and verification remain outside those DB transactions. `GET /admin/billing/plans` is intentionally a global catalogue read. This service-level boundary is structurally in place, but it still needs integration tests using the actual restricted runtime login and staging term fixtures; do not label it runtime-verified yet.

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

## Staging billing schema and RLS installation (2026-11-16)

After explicit authorization, the exact repository SQL files below were applied to staging project `school-media-recovery-test` only, in order:

1. `migrations/20261112_school_subscription_foundation.sql`
2. `migrations/20261112_subscription_checkout_payments.sql`
3. `scripts/security/install_subscription_billing_rls.sql`

Verified after application:

- Four billing tables exist: `billing_plans`, `school_subscriptions`, `subscription_events`, and `subscription_payments`.
- Five plan catalogue rows exist.
- All four tables have RLS enabled and FORCE RLS enabled.
- Exactly nine expected policies exist across the four tables.
- All four tables remain owned by `postgres`, not the runtime role.
- `stonez_api_runtime` remains NOLOGIN, NOSUPERUSER, NOBYPASSRLS, NOCREATEDB, NOCREATEROLE, INHERIT.
- Runtime grants match the intended minimum: plans SELECT; subscriptions SELECT/INSERT/UPDATE; events SELECT/INSERT; payments SELECT/INSERT/UPDATE.
- No table privileges remain for `service_role` on the four billing tables.
- The two SQL files are intentionally outside the automatic GORM migration runner; the application migration ledger was not modified.

Important test limitations:

- The staging database currently has one school but zero terms. Therefore a valid school/term subscription fixture cannot be created for an end-to-end tenant-write test without adding staging data; use isolated fixtures rather than altering this shared staging project.
- An attempt to switch the shared staging SQL editor session to `stonez_bulk_worker` using `SET ROLE` was rejected with permission denied. That staging editor session therefore did not prove the staging worker login's runtime behavior. Separately, the isolated CI PostgreSQL RLS test now creates an ephemeral NOLOGIN group role plus a distinct NOBYPASSRLS LOGIN member and connects using the login's password; it asserts both `current_user` and `session_user` are that login.
- The isolated CI test passed against disposable PostgreSQL 16: the real restricted LOGIN inherited the policy role, read only each tenant's own rows for subscriptions/events/payments, was denied cross-school inserts and school-changing updates, was denied mismatched event actors, and received zero tenant rows plus rejected inserts without transaction-local tenant context. It also verified the intended global active-plan read and denied excess write privileges. This is strong policy/runtime-role evidence, but it does not prove the separately provisioned `stonez_bulk_worker` login on the shared staging project can authenticate; that staging-specific login smoke test remains outstanding.
- The installer policies trust transaction-local `app.current_school_id` / `app.current_user_id` values supplied by application code. They are not authenticated database claims.

No production database, credentials, Render service or deployment was changed.

## Required remaining work

1. Complete restricted-login PostgreSQL integration tests for billing's service-managed tenant transactions, including no-context denial, cross-school isolation and provider-call transaction boundaries. Provision realistic staging term fixtures in an isolated test setup (not by editing live staging school data). Also migrate and test `/me` with explicit user/school ownership boundaries. Keep provider calls outside DB transactions and preserve idempotent verification/reconciliation.
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

No production database was queried or changed as part of this audit. No production role, policy, grant, secret, Render setting or deployment was changed. The initial staging catalog queries were read-only; the separately authorized billing DDL installation was limited to staging.

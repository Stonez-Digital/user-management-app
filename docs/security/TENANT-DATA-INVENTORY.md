# Tenant data ownership and access inventory

**Status:** first-pass engineering inventory; security-owner review is required before policy SQL is authored.  
**Source reviewed:** `internal/database/postgres.go` migrations 1–47 and the current `internal/models` definitions for school, user, audit, academic, student, finance, content, career-path, notification, subscription, curriculum, and bulk-import records.

This document classifies ownership. It does **not** claim PostgreSQL currently enforces these boundaries. “Public read” means a deliberately designed application endpoint, never a direct database grant to PostgreSQL `PUBLIC`, Supabase `anon`, or `authenticated`.

## Ownership classes

- **School-owned:** one school owns the row directly or through a parent. Require transaction-local school context; deny missing context and cross-school reads/writes. Child records must verify parent ownership.
- **User-owned:** one account owns the row. Require server-derived authenticated user identity; school context alone is insufficient.
- **Platform-global:** Stonez Digital configuration or platform-only records. School roles must not mutate them; platform operations need a distinct authorization path.
- **Mixed / dual-scope:** legitimately contains school and platform records or multiple ownership dimensions. Use distinct predicates; NULL school ownership must never mean visible to every school.
- **Public-read surface:** a published subset exposed through application handlers only. Publication state and trusted school/domain resolution are required.
- **Queue / migration / system:** worker or schema-operation state. Use separate identities and narrowly scoped capabilities.

## School-owned records

| Table | Ownership key | Additional invariant |
|---|---|---|
| `students` | `school_id` | Linked user must belong to the same school. |
| `academic_sessions` | `school_id` | Parent of terms. |
| `terms` | `school_id` | Must match academic session school. |
| `school_classes` | `school_id` | Parent of sections. |
| `sections` | `school_id` | Must match parent class school. |
| `subjects` | `school_id` | School-specific catalog. |
| `class_subjects` | class/subject relationship | Both referenced rows must belong to the same school. |
| `student_enrollments` | `school_id` | Must match student and academic session. |
| `attendance_records` | `school_id` | Verify enrollment and term school. |
| `teacher_assignments` | `school_id` | Teacher, subject, class, section, session, and term must match. |
| `assessments` | `school_id` | Verify assignment school. |
| `assessment_results` | `school_id` | Verify assessment and enrollment school. |
| `fee_items` | `school_id` | Verify term school. |
| `invoices` | `school_id` | Verify enrollment and term school. |
| `invoice_lines` | `school_id` | Verify invoice; optional fee item must match. |
| `payments` | `school_id` | Verify invoice school. |
| `timetable_entries` | `school_id` | Verify all referenced academic and staffing records match. |
| `guardian_relationships` | `school_id` | Parent access additionally requires relationship ownership. |
| `announcements` | `school_id` | Audience/class targeting is an additional authorization dimension. |
| `notifications` | `school_id` + `user_id` | Require both school and recipient-user match. |
| `career_path_attempts` | `school_id` + `student_id` + `user_id` | All three ownership links must agree; student data is private. |
| `blog_posts`, `blog_categories` | `school_id` | Public only through published/visible school-specific handlers. |
| `gallery_albums`, `gallery_images` | `school_id` | Image must match album school; media object keys must be school-scoped. |
| `school_advertisements` | `school_id` | Public only when active and within schedule; management is school-admin only. |
| `school_enquiries` | `school_id` | Public create is an application endpoint; staff manage only their own school’s enquiries. |
| `bulk_import_jobs` | `school_id` + `initiated_by` | Require school and initiating-user match; worker lease fields are system-managed. |
| `school_subscriptions` | `school_id` | School may read own subscription; billing mutations need explicit platform/service authorization. |
| `subscription_events` | `school_id` | School-scoped history; platform-generated events may require a distinct platform capability. |
| `subscription_payments` | `school_id` + `subscription_id` | Present only in the separate review-only checkout SQL migration; verify parent subscription school. |

## User-owned / account-security records

| Table | Ownership key | Required boundary |
|---|---|---|
| `sessions` | user/account | Session lookup and revocation scoped to authenticated account. |
| `refresh_tokens` | user/account | Hash-only token storage; not a school-wide readable resource. |
| `password_reset_tokens` | user/account | Reset lifecycle only; no school-wide access. |
| `parent_profiles` | user plus school relationship | Verify linked user and guardian relationship. |
| `teacher_profiles` | user plus school relationship | Verify linked user and school-specific teacher assignment. |

## Mixed / dual-scope records

| Table | Scope | Required separation |
|---|---|---|
| `users` | School users have `school_id`; platform super-admin users are intentionally NULL-school. | School predicates require matching school; NULL-school platform users are not visible to school roles. Platform identity operations are separate. |
| `audit_logs` | School events plus platform-global events (NULL `school_id`). | A/B schools see only their own events; neither sees NULL-school events. Platform audit access is separately privileged and tested. |
| `role_change_audits` | School role changes and potentially platform role changes. | Explicit school scope; platform role changes are platform-only. NULL is never a wildcard. |

## Platform-global records

| Table | Scope | Required separation |
|---|---|---|
| `schools` | Platform-managed tenant directory and school profile root. | School admins may update only their own profile through scoped application operations; cross-school administration is platform-only. |
| `billing_plans` | Platform plan catalog. | Optional school read is a narrow published/read surface; mutations platform-only. |
| `national_curricula` | Shared national curriculum catalog. | Read through approved handlers; mutations platform-only. |
| `national_curriculum_levels` | Curriculum child | Inherit parent curriculum scope. |
| `national_curriculum_classes` | Level child | Verify parent chain. |
| `national_curriculum_subjects` | Class child | Verify parent chain. |

## Queue / migration / system records

| Table | Scope | Required separation |
|---|---|---|
| `migrations` | Migration ledger | Migration identity only; unavailable to school API roles. |
| `bulk_import_jobs` | Operational queue state with school/user ownership | Worker claim/lease is a system capability. The existing global queue-claim path is privileged and blocks NOBYPASSRLS cutover until replaced with a narrowly scoped mechanism. |

## Public-read surfaces are not table grants

Candidates for unauthenticated application-mediated reads: published school blog posts/categories, visible gallery albums/images, active advertisements, explicitly public school-profile fields, and active national curriculum data. Public school enquiry submission is a write surface, not public read access. Each handler must resolve the school from a trusted hostname/slug, enforce publication/status rules, and return only explicitly public columns.

No table should receive broad direct privileges merely because a public page consumes some of its data.

## Migration provenance and coverage boundary

There is more than one migration source in this repository; they must not be conflated:

- `internal/database/postgres.go` is the automatic Go/GORM application migration runner (currently 47 numbered steps).
- `migrations/20261112_school_subscription_foundation.sql` explicitly says **REVIEW ONLY** and is intentionally not registered in the automatic runner. It defines `billing_plans`, `school_subscriptions`, and `subscription_events`, enables RLS on those three tables, and revokes broad Data API privileges.
- `migrations/20261112_subscription_checkout_payments.sql` also explicitly says **REVIEW ONLY** and is not registered in the GORM runner. It defines `subscription_payments`, enables RLS, and revokes broad privileges.
- `supabase/migrations/20261013120721_careerpath_attempts_backend_only_access.sql` is a separate Supabase migration source. It enables RLS on `career_path_attempts`, revokes `anon`/`authenticated`, and grants `service_role`; it does not create a tenant-aware policy or force RLS.
- The Go migration runner itself creates `career_path_attempts` and adds its composite school/student/user integrity constraint. That Go migration and the separate Supabase access migration are distinct operations.

Therefore a test that calls only `database.Migrate` proves the automatic Go migration path only. It must not claim to have applied the review-only SQL files or the Supabase migration history. Those sources need separate, explicitly approved disposable-database coverage with schema-qualified assumptions resolved. In particular, the checkout SQL hardcodes `public`, so a search_path-only temporary schema is not sufficient to safely execute it.

### Observed automatic Go migration RLS baseline

The disposable catalog test confirms the automatic Go runner currently enables RLS on these 15 tables:

- Migration 24: `schools`, `bulk_import_jobs`, `teacher_profiles`, `parent_profiles`.
- Migration 32: `blog_posts`, `blog_categories`, `gallery_albums`, `gallery_images`, `school_advertisements`.
- Migration 36: `class_subjects`.
- Migration 37: `national_curricula`, `national_curriculum_levels`, `national_curriculum_classes`, `national_curriculum_subjects`.
- Migration 41: `school_enquiries`.

The Go migration catalog currently contains **zero** `pg_policies` entries for its migrated schema and does not set `relforcerowsecurity` on these tables. These are catalog facts, not a tenant-isolation guarantee. RLS enablement without tenant policies does not establish school boundaries, and the rollout notes identify the current API database connection as privileged/BYPASSRLS.

Migration 32 specifically protects school public-content tables by enabling RLS and revoking `PUBLIC`, `anon`, and `authenticated` privileges; it does not create a tenant policy. Migration 46 separately revokes broad Data API privileges from its application-table inventory, but does not create a restricted API role or grant its least-privilege DML.

### Production-documented posture is a separate evidence source

The existing `docs/SECURITY-RLS-ROLLOUT.md` reports that the deployed production catalog had RLS enabled on all 45 public application tables, RLS was not forced, and the existing `deny_client_access` policies applied only to Supabase `anon` and `authenticated`. That is a reported production-catalog snapshot, not something this disposable Go-migration test can independently verify.

The clean automatic Go migration schema has 42 tables and enables RLS on 15 of them, with zero policies. Three subscription tables are absent because their SQL files are explicitly review-only and not registered in the Go runner. The different counts/postures must be treated as **migration/provisioning drift to reconcile**, not as proof that either source is wrong. Possible causes include separately applied Supabase/dashboard DDL, the separately managed subscription SQL, or stale rollout documentation.

Before policy cutover, capture a read-only catalog export from the approved staging environment (and reconcile with the latest production evidence through the established review process): all application tables, owners, RLS flags, `pg_policies` definitions, role memberships, schema/table/sequence privileges, and grants. Compare that export against both the automatic Go migration baseline and separately managed SQL/Supabase migration sources. Do not connect this test to production or attempt to repair the difference with production DDL.

## Migration-parity assertions

The disposable PostgreSQL suite must apply the real `database.Migrate` implementation and inspect catalogs. It must not create replacement policies in test setup.

1. Compare actual application tables with this inventory; fail on unclassified tables and stale entries.
2. For every school-owned and mixed-scope table, assert RLS enabled and forced, plus exact reviewed tenant policy names and `USING` / `WITH CHECK` expressions.
3. Assert user-owned tables have explicit account predicates and no school-wide disclosure.
4. Assert platform-global tables have explicit role boundaries; do not silently treat them as tenant or public tables.
5. Assert `migrations` is unavailable to runtime roles.
6. Inspect table ownership, role attributes, memberships, schema/table/sequence privileges, and absence of unintended `TRUNCATE`, `REFERENCES`, and `TRIGGER`.
7. Exercise two-school SELECT/INSERT/UPDATE/DELETE, school reassignment, missing context, user ownership, NULL-school audit isolation, and approved public-read behavior.
8. Exercise worker queue claims separately from school request transactions.

## Decisions required before policy SQL

- Whether `schools` profile reads are tenant-self-only or use a separate public projection/view.
- Whether `billing_plans` and national curriculum data are readable by all school roles or only selected application handlers.
- The exact platform audit authorization mechanism for NULL-school `audit_logs` and platform `role_change_audits`.
- Whether `parent_profiles` and `teacher_profiles` are user-owned with school relationships or school-owned profile records; both dimensions must be checked either way.
- Whether any subscription event types are generated by platform billing services and need a distinct platform-only representation.

**Safety conclusion:** this is a classification baseline, not evidence that production migrations enforce it. Measure actual migration state with the disposable parity suite. Do not create production roles, grants, policies, secrets, or deploy as part of this work.

# CareerPath — Repository Inspection and Implementation Plan

**Status:** Phase 1 inspection completed against the repository's current `main` snapshot; implementation not yet started.  
**Reference inspiration:** https://www.careerexplorer.com/career-test/  
**Product owner:** Stonez Digital

## Verified repository findings

- Repository: `Stonez-Digital/user-management-app`; default branch `main`.
- Inspected `main` HEAD: `78e003ef3160dad897f0153503a86e156dfa5933` (2026-10-03).
- Frontend: Next.js 16 / React 19, under `frontend/app`, with dashboard route groups and shared CSS/components. The student portal is `frontend/app/dashboard/student/page.tsx`; frontend API calls use `/backend` and the bearer token in `localStorage`.
- Backend: Go 1.25, Gin, GORM. Startup and route registration are in `cmd/server/main.go`; existing features use controller/service/repository layers.
- Database: PostgreSQL (production) and SQLite (tests/local) configured in `internal/database/postgres.go`. Schema evolution is an ordered, versioned migration list using `MigrationStep` and GORM migrations. The inspected list includes tenant-scoping migrations; add a new version rather than modifying an already-applied migration.
- Identity and access: `internal/middleware/auth.go` validates bearer JWTs and reloads the current role from the database. `internal/middleware/tenant.go` derives school context from the authenticated user's database record. `internal/middleware/rbac.go` and `internal/authz/rbac.go` provide role/permission enforcement.
- Existing student identity: `models.Student` ties `UserID` to `SchoolID`; `StudentPortalService` resolves the student using both school and authenticated user IDs. CareerPath should follow this pattern and must not accept a client-supplied student/user ID as authority.
- Parent links exist in `models.GuardianRelationship`; parent access must verify active same-school guardian relationships in the service, not rely on a submitted student ID.
- Student portal routes are registered in `cmd/server/main.go` under authenticated `/student` routes with school context and `PermissionStudentPortalRead`. Existing API error helpers are in `internal/httpx`.
- Frontend scripts include build and a focused dashboard regression test; Go CI is defined in `.github/workflows/go-test.yml`.
- One open PR was observed at inspection: PR #225, an unrelated school-image storage fix. It must not be altered by CareerPath work.
- The remote GitHub API cannot reveal a developer's uncommitted local working-tree changes. No claim is made that a local working tree was inspected.

## Implementation plan

### Phase A — Safe, student-owned vertical slice

1. Add CareerPath domain models for a versioned assessment, questions/options, student attempts/responses, curated career catalogue, and saved result/match explanations. Keep the initial catalogue intentionally small and reviewed.
2. Add a new, additive migration after the latest existing migration version. Include school ownership, student ownership, foreign keys, unique constraints for question keys and one response per question per attempt, and indexes for school/student/status lookups. Avoid destructive backfills or deleting existing records.
3. Add a dedicated service layer for assessment retrieval, create/resume, response validation and saving, submission, deterministic matching, and explanation generation. Make submission idempotent and reject attempts to change submitted responses.
4. Add authenticated student endpoints under the existing student route group. Derive school and user identity from middleware context; resolve the matching student record server-side. Scope every query by school and student. Never expose other students' answers or results.
5. Add a responsive CareerPath page to the existing student dashboard, with an overview, original multi-step questions, save/resume, review/submit, and results with several matches. Use the existing CSS classes/components and API proxy; do not add a second auth system.
6. Add Go unit/integration tests for scoring determinism, missing/invalid answers, attempts, idempotent submission, tenant/student isolation, and unauthorized requests. Add frontend regression coverage where the existing test harness supports it.
7. Run the repository's documented Go tests and frontend checks through CI. Open a draft PR; do not merge or deploy before required checks and review.

### Phase B — Expand after the first slice is reviewed

- Career catalogue search/details, saved/dismissed careers, comparison, and career goals/milestones.
- Learning pathway activities and verified, dated learning resources.
- Counsellor workflows and parent summaries only after a specific authorization policy is designed and tested. Do not grant teachers or school admins blanket access to private responses.
- Data export/correction/deletion and retention policy, privacy notice, and school-approved lawful-basis/consent workflow before production rollout.
- A provider adapter interface for future licensed assessment services; no provider credentials or secrets in browser code.

## Initial data and matching rules

- Original, age-appropriate questions across interests, working preferences, subjects/skills/activities, and goals.
- Do not ask for sensitive personal attributes or infer diagnoses, ability, socioeconomic status, or family circumstances.
- Use transparent tag/attribute overlap with deterministic tie-breaking and a documented scoring version. Show plain-language reasons tied to the student's selected answers, not unsupported scientific labels or precision.
- Return multiple exploratory options. Clearly state that matches are advisory and never determine admissions, placement, eligibility, or future opportunities.
- Seed only a manageable catalogue of reviewed entries with career descriptions, relevant subjects/skills, a practical activity, and cautious education-route guidance. Do not invent salaries or labour-market claims.

## Release gates

- [ ] Confirm latest migration version before assigning the next version.
- [ ] Review database foreign keys and tenant/student ownership constraints.
- [ ] Test student A cannot read/write student B's attempts within the same school.
- [ ] Test a user from another school cannot access attempts or catalogue-owned records across tenants.
- [ ] Test parent/teacher/school-admin access is denied unless an explicit reviewed policy permits it.
- [ ] Run Go tests, frontend regression tests, lint/type checks and production build; record actual outputs.
- [ ] CI green and independent review approved before merge; no production deployment from this branch.

## Scope note

This document records inspection findings and the intended implementation sequence. It is not evidence that the CareerPath runtime feature, migration, endpoints, or tests have been implemented or deployed.

# CareerPath — Repository Inspection and Implementation Plan

**Status:** Phase 1 inspection completed; the first student-owned assessment vertical slice is implemented on the feature branch and awaiting CI/review.  
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

Implemented on the feature branch (pending CI and review):

1. Added `models.CareerPathAttempt` for versioned, school/student-owned attempts with response/result payloads, state and submission timestamp. GORM relationships constrain attempts to existing school and student records.
2. Added migration 39, `careerpath_assessment_attempts`, and a partial unique index limiting each student/version to one in-progress draft. The migration is additive and does not delete or rewrite existing records.
3. Added `CareerPathService` with original questions, a reviewed starter catalogue of eight careers, strict option/question validation, draft resume, deterministic tag-overlap matching with plain-language explanations, idempotent submission, and immutable submitted attempts.
4. Added authenticated student endpoints for assessment retrieval, create/resume, attempt retrieval, saving responses, and submission. Identity and school context come from existing middleware; service queries scope by school, user and student.
5. Added a responsive student CareerPath route and dashboard entry using the existing `/backend` proxy and bearer-token session. The first slice shows several career possibilities, subjects, skills and a practical activity.
6. Added backend tests for deterministic/idempotent matching, incomplete and invalid answers, immutability, and student/cross-school access boundaries. Migration tests now expect the added migration.
7. CI must be observed and any failures fixed before this PR can be marked ready; no merge or production deployment is authorized before review.

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

This document records verified inspection findings and implementation scope. The first vertical slice exists only on this feature branch until CI, review, and merge are complete; it has not been deployed to production.

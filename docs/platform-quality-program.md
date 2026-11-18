# Platform-Wide Quality and Reliability Programme

## Objective

Make every school workflow dependable for a newly onboarded school and for established schools. A school administrator must be able to complete a workflow without hidden prerequisites, stale selectors, ambiguous validation, duplicate submissions, or silent failures. Backend tenant authorization and data integrity remain authoritative.

This is a living audit and delivery plan. The route inventory is a starting scope, not a claim that every route has already passed acceptance testing.

## Engineering rules

1. **Tenant boundary:** every school-owned read, create, update, delete, export, and relationship lookup is scoped to the authenticated school on the server. Never trust a school ID supplied by the browser as authorization.
2. **Relationship integrity:** selected records must belong to the same school and satisfy their business relationship (for example, a term belongs to the selected session; a section belongs to the selected class; an enrollment belongs to the selected student/session).
3. **Authoritative validation:** validate on the server even when the UI validates. Return stable error codes and actionable field-level details where possible; do not expose cross-tenant record existence.
4. **Safe forms:** prevent duplicate submissions synchronously, preserve entered values after recoverable errors, reset only after confirmed success, and show a clear success or failure state.
5. **Dependent selectors:** clear invalid downstream selections when a parent selection changes; cancel or ignore stale asynchronous responses; never submit IDs from a previous selection.
6. **Loading and empty states:** distinguish loading, empty, unavailable, and failed states. Do not turn failed requests into an apparently valid empty list.
7. **Dates and money:** use school-local calendar semantics for date-only fields; validate date ranges and currency/amount precision at the service boundary.
8. **Auditability:** sensitive actions (role changes, activation, deletion, financial records, results publication, student placement) are authorized, confirmed where destructive, and auditable.
9. **Accessibility and responsive use:** forms and tables remain usable by keyboard, screen readers, touch, and narrow mobile screens.
10. **Evidence:** report code tests, CI, isolated tenant tests, browser acceptance, and production verification separately. Passing one does not imply the others.

## Module audit inventory

| Area | Primary routes | Critical dependencies / acceptance focus |
|---|---|---|
| School onboarding and identity | `/dashboard/onboarding`, `/dashboard/school`, `/platform/schools` | approval gate, school profile, resumable setup, logo and contact data, role access |
| Academic foundation | `/dashboard/academic`, `/dashboard/curriculum` | session → terms → classes → sections → subjects; date boundaries; selector refresh |
| People and enrollment | `/dashboard/students`, `/dashboard/users`, `/dashboard/enrollments` | profile/user consistency, admission IDs, guardian links, class placement, promotion history |
| Teaching operations | `/dashboard/teacher-assignments`, `/dashboard/timetable`, `/dashboard/teacher` | teacher/subject/session/term/class/section consistency, assignment conflicts, stale selectors |
| Attendance | `/dashboard/attendance`, `/dashboard/teacher/attendance` | active enrollment, school-local date, duplicate attendance, edits and audit trail |
| Assessment and results | `/dashboard/assessments`, `/dashboard/teacher/assessments`, `/dashboard/results`, `/dashboard/teacher/results` | assignment scope, configurable grading/CA weights, score validation, publication and report cards |
| Finance | `/dashboard/finance`, `/dashboard/operations` | fee → invoice → payment → receipt/ledger; balance and concurrency; manual/provider trust boundary |
| Communication and content | `/dashboard/notifications`, `/dashboard/content` | audience scope, delivery state, attachment validation, privacy and retention |
| Portals and audit | `/dashboard/parent`, `/dashboard/student`, `/dashboard/audit` | guardian authorization, student ownership, least privilege, event history |
| Platform administration | platform routes and admin APIs | platform-vs-school role separation, approval controls, safe operational actions |

## Initial evidence from the code review

These are concrete review observations, not production incident claims:

- Dashboard pages implement their own `api()` wrappers. The wrappers differ in response parsing and error handling: some parse JSON directly, some read text first, and some silently fall back to an empty object. This creates inconsistent behavior for validation details, non-JSON gateway errors, and session expiry.
- The teacher-assignment, academic, enrollment, attendance, finance, assessment, and user pages each own substantial request/form state. Their duplicate-submit, loading, error, and refresh behavior is not yet governed by one shared contract.
- The academic page catches individual term-fetch failures and substitutes an empty term list. That can make a failed request look like a session with no configured terms.
- The teacher-assignment service correctly checks teacher, subject, session, term, class, and optional section against the authenticated school, and verifies that the term belongs to the selected session. The current form's helper copy is not itself proof of a backend failure. The specific failing record in the reported request still needs a reproducible test or actionable server diagnostic.
- Existing service tests already cover important tenant boundaries and relationship constraints in several domains. The audit will extend those tests where coverage is missing rather than weakening existing checks.

## Delivery sequence

### Phase 1 — Shared request and form reliability
- Introduce one typed frontend API client with consistent JSON/text parsing, stable API error objects, field-level validation rendering, and consistent 401 handling.
- Migrate high-traffic dashboard workflows in small batches; avoid a risky all-at-once rewrite.
- Establish reusable patterns for submit locks, pending states, success notices, preserving form values, and refreshing dependent data.
- Add unit/regression tests for validation response parsing, malformed responses, network errors, and expired sessions.

### Phase 2 — Academic and people dependency chains
- Verify session/term/class/section/subject selectors and all dependent refresh behavior.
- Verify student/profile/guardian/enrollment relationships and promotion history.
- Add cross-school negative tests for every relationship boundary.
- Test rapid selector changes and out-of-order API responses.

### Phase 3 — Teaching, attendance, assessment and results
- Verify teacher assignment creation/update, class-teacher conflict rules, timetable links, attendance date semantics, assessment configuration, score entry, grading calculations, publication, and printable report cards.
- Add deterministic fixtures for edge cases, duplicate submissions, date boundaries, and authorization.

### Phase 4 — Finance and communication
- Verify fee/invoice/payment/receipt/ledger flows, concurrent payments, duplicate references, refunds, reporting totals, and provider verification trust boundaries.
- Verify message audience scoping, notification delivery states, content permissions, and retention behavior.
- Never create fake production payments or alter real customer records for acceptance tests.

### Phase 5 — Administration, security and full acceptance
- Verify role matrix, school approval, account lifecycle, audit events, exports, and platform-vs-school boundaries.
- Run isolated seeded test-school browser journeys for each role and module.
- Verify mobile, keyboard, accessibility, and deployment behavior.

## Required gate for each implementation PR

- Focused scope and documented root cause.
- Backend unit/service tests for success, invalid relationships, cross-school access, and concurrency where relevant.
- Frontend regression tests for loading, empty, validation, duplicate submit, and stale-response behavior.
- Go tests, frontend checks/build, security checks, and production QA all green.
- Diff reviewed for tenant-scope regressions and accidental schema/data changes.
- No production migration/data mutation unless separately reviewed and explicitly authorized.
- After an authorized merge: verify the Render deployment, frontend deployment, health, and a safe production read-only smoke check.
- State clearly what was tested and what remains unverified in a real browser.

## Current work tracking

- Teacher assignment request validation is tracked separately in PR #309. Its request-binding, frontend error-display, and service relationship behavior must be reviewed together. It is not the platform-wide fix and must not be treated as proof that every module meets this standard.
- The next implementation batch should address the shared API/error contract, then migrate academic and enrollment workflows first because they are prerequisites for teaching, attendance, assessment, and finance.

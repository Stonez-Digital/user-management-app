# Production Pilot Acceptance Suite

Issue: #142 — Production pilot readiness: role and tenant acceptance suite

This is the repeatable acceptance procedure for the first controlled school pilot. It is designed for temporary QA accounts and must not contain real passwords, access tokens, or other secrets.

## Production endpoints

- Frontend: https://stonez-school-management.onojamondayojonugba.workers.dev/
- API: https://stonez-digital-school-api.onrender.com

## QA roles

Use temporary production QA accounts for:

| Role | Expected scope |
|---|---|
| super_admin | Stonez Digital platform administration |
| school_admin | One school tenant only |
| teacher | Assigned school teaching workflows |
| student | Own student portal and academic access |
| parent | Linked child/children only |
| accountant/staff | School operations permitted by assigned role |

Record only the account role and test result. Never commit credentials.

## Acceptance sequence

### 1. Authentication and profile

For every QA role:

- Sign in with the temporary QA account.
- Confirm login succeeds.
- Confirm the authenticated profile loads.
- Confirm the displayed role is correct.
- Confirm school-scoped roles display the expected school identity.
- Refresh the page and confirm the session remains valid.
- Sign out.
- Confirm protected requests no longer succeed with the revoked session.
- Sign in again and confirm a fresh session works.

Expected result: no role reaches a dashboard with an incorrect or missing authenticated profile.

### 2. Dashboard routing

| Role | Expected destination |
|---|---|
| super_admin | Platform administration |
| school_admin | School administration |
| teacher | Teacher workspace |
| student | Student portal |
| parent | Parent portal |
| accountant/staff | School operations appropriate to role |

Attempt to open another role's restricted dashboard directly.

Expected result: access is denied or the user is redirected to an allowed destination.

### 3. Bedrock school workflow

For the pilot school:

1. Confirm the school-admin account belongs to the correct school.
2. Confirm school identity appears on the school dashboard.
3. Confirm existing academic session/term data is visible.
4. Confirm classes and sections are school-scoped.
5. Confirm teacher accounts can access assigned teaching workflows.
6. Confirm student accounts can access only their own academic information.
7. Confirm parent accounts can access only their linked student's information.

### 4. Student lifecycle

Using a temporary pilot student:

1. Create/admit the student.
2. Enroll the student into a session, class and section.
3. Open the enrollment history.
4. Promote the student to a target session/class/section.
5. Confirm the source enrollment becomes `completed`.
6. Confirm the target enrollment becomes `active`.
7. Confirm both records remain visible in history.
8. Refresh and repeat the history check.

For a completed/withdrawn student, verify re-enrollment creates a new active enrollment without incorrectly modifying historical records.

### 5. Tenant isolation

Use two test school tenants where available.

For a school-scoped QA account:

- Request another school's users.
- Request another school's students.
- Request another school's academic sessions/classes.
- Attempt to access another school's enrollment by ID.
- Attempt to modify another school's record by ID.
- Attempt to use another school's target session/class/section during enrollment placement.

Expected result: cross-school data is not returned and cross-school mutations are rejected.

Do not use production real-user data to perform destructive isolation tests.

### 6. Unauthorized API access

Without an access token:

- Call a protected endpoint.
- Confirm HTTP 401.

With a valid token for one role:

- Call an endpoint requiring a different role/permission.
- Confirm access is denied.
- Confirm school-scoped endpoints cannot be used to escape the authenticated school.

### 7. Academic access

For teacher:

- Load assigned classes/subjects.
- Load timetable if assigned.
- Load assessments/results workflows.
- Confirm access is limited to permitted school data.

For student:

- Load profile.
- Load enrollment.
- Load terms/academic information.
- Load attendance where available.
- Confirm another student's records cannot be accessed.

For parent:

- Load linked child information.
- Confirm only linked child records are available.

### 8. Cloudflare → Render connectivity

From the live frontend:

- Sign in.
- Load `/me`.
- Load a role dashboard.
- Perform at least one authenticated read.
- Perform one permitted school-admin write using a QA account.
- Confirm the request completes through the Cloudflare frontend proxy to the Render API.

Expected result: no browser-side direct database access and no API-origin/CORS failure.

### 9. Responsive smoke test

Repeat the dashboard and one core workflow on:

- Desktop
- Tablet width
- Mobile width

Check:

- navigation
- login form
- dashboard cards
- tables
- forms
- buttons/touch targets
- horizontal overflow
- modal/confirmation dialogs

### 10. Acceptance record

Record results outside the repository using this structure:

| Area | Role | Result | Notes |
|---|---|---|---|
| Login/profile | role | PASS/FAIL | |
| Dashboard routing | role | PASS/FAIL | |
| Academic access | role | PASS/FAIL | |
| Tenant isolation | school role | PASS/FAIL | |
| Unauthorized access | role | PASS/FAIL | |
| Logout/session | role | PASS/FAIL | |
| Mobile smoke | role | PASS/FAIL | |
| Cloudflare → Render | role | PASS/FAIL | |

A pilot is ready for the next controlled stage only after all critical authentication, authorization, tenant-isolation, and core workflow checks pass.

## Safety rules

- Never commit passwords, refresh tokens, access tokens, API keys, or real student/parent information.
- Use temporary QA accounts.
- Prefer non-destructive test records.
- Remove or deactivate temporary QA accounts after the pilot test window.
- Do not treat a successful UI response as proof of tenant isolation; verify both positive access and denied cross-tenant access.


## Automated two-school acceptance

The repository includes a manual GitHub Actions workflow at `.github/workflows/production-acceptance.yml`.

It runs `scripts/production-acceptance.mjs` against the live Render API and:

- verifies the Super Admin session;
- verifies both configured school-admin accounts and their `/me.school_name`;
- creates uniquely named temporary teacher/student/parent QA accounts inside each school;
- creates missing QA academic session/class/section records when required;
- enrolls the temporary student and executes promotion;
- verifies active target enrollment and preserved history;
- signs in as teacher, student and parent;
- checks student portal, teacher assignments and parent-child access;
- checks unauthenticated access;
- checks role restrictions;
- checks cross-school user, student and academic-session isolation;
- checks refresh/logout/re-login.

### GitHub Actions secrets

Configure these repository secrets before running the workflow manually:

- `PRODUCTION_QA_SUPER_ADMIN_EMAIL`
- `PRODUCTION_QA_SUPER_ADMIN_PASSWORD`
- `PRODUCTION_QA_SCHOOLS_JSON`

`PRODUCTION_QA_SCHOOLS_JSON` must contain exactly two objects with this shape:

```json
[
  {"school_code":"001","admin_email":"...","admin_password":"..."},
  {"school_code":"EVERGREEN","admin_email":"...","admin_password":"..."}
]
```

Passwords are consumed only as GitHub Actions secrets. The script generates temporary QA account passwords at runtime and never prints them.

The workflow is deliberately `workflow_dispatch` only because it creates temporary production QA records. Its result is uploaded as a private GitHub Actions artifact for seven days.

The responsive desktop/tablet/mobile portion remains a browser smoke test; the automated workflow focuses on API, authentication, authorization, tenant isolation and lifecycle behavior.

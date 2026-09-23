# Bedrock International Academy — Pilot Test Pack

**Pilot issue:** #151  
**Environment:** Production  
**Academic session:** 2026/2027  
**Term:** First Term  
**Frontend:** https://stonez-school-management.onojamondayojonugba.workers.dev/  
**API:** https://stonez-digital-school-api.onrender.com/

## 1. Pilot objective

Validate the production school-management workflow with a small, controlled set of Bedrock users before broader school adoption.

This is a QA document, not a production data-import document. Use temporary/test identities unless the school explicitly authorizes real pilot accounts.

## 2. Pilot account matrix

| Role | Target count | Account | Login | Status | Notes |
|---|---:|---|---|---|---|
| School Admin | 1 | Admin 1 | TBD | ☐ | |
| Teacher | 2–5 | Teacher 1 | TBD | ☐ | |
| Teacher | 2–5 | Teacher 2 | TBD | ☐ | |
| Student | 10–30 | Student 1 | TBD | ☐ | |
| Student | 10–30 | Student 2 | TBD | ☐ | |
| Parent | 5–15 | Parent 1 | TBD | ☐ | |
| Parent | 5–15 | Parent 2 | TBD | ☐ | |

**Credential rule:** Do not commit passwords, reset tokens, API tokens, or other secrets to GitHub.

## 3. Sample academic data

Use a small representative dataset first.

### Suggested classes

- JSS 1
- JSS 2
- SS 1

### Suggested sections

- A
- B

### Suggested subjects

- English Language
- Mathematics
- Basic Science
- Computer Science
- Social Studies

### Suggested pilot allocation

At minimum, assign each pilot teacher to an appropriate class/subject for the active 2026/2027 First Term. Expand coverage before wider pilot use.

## 4. QA sequence

### Gate A — Platform access

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| AUTH-01 | School admin logs in | Login succeeds and account loads | ☐ |
| AUTH-02 | Teacher logs in | Teacher dashboard loads | ☐ |
| AUTH-03 | Student logs in | Student dashboard loads | ☐ |
| AUTH-04 | Parent logs in | Parent dashboard loads | ☐ |
| AUTH-05 | Invalid credentials | Access denied without exposing sensitive details | ☐ |
| AUTH-06 | Logout | Session is invalidated/cleared | ☐ |

### Gate B — School and academic setup

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| ACAD-01 | Admin opens school context | Bedrock context is shown | ☐ |
| ACAD-02 | View active session | 2026/2027 is active | ☐ |
| ACAD-03 | View active term | First Term is active | ☐ |
| ACAD-04 | View classes | Pilot classes are available | ☐ |
| ACAD-05 | View sections | Sections belong to the correct classes | ☐ |
| ACAD-06 | View subjects | Pilot subjects are available | ☐ |
| ACAD-07 | View teacher coverage | Active assignments are visible | ☐ |

### Gate C — User onboarding

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| USER-01 | Create/import teacher | Teacher is created in Bedrock tenant | ☐ |
| USER-02 | Create/import student | Student is created in Bedrock tenant | ☐ |
| USER-03 | Create/import parent | Parent is created in Bedrock tenant | ☐ |
| USER-04 | Enroll student | Student is linked to the correct class/section/session | ☐ |
| USER-05 | Link parent to student | Parent sees only linked student(s) | ☐ |
| USER-06 | Deactivate test account | Account cannot continue protected access | ☐ |

### Gate D — Teacher workflow

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| TEACH-01 | Teacher views assigned classes | Only assigned teaching scope is shown | ☐ |
| TEACH-02 | Teacher views assigned subjects | Correct subjects are shown | ☐ |
| TEACH-03 | Teacher records attendance | Attendance saves for permitted students | ☐ |
| TEACH-04 | Teacher records assessment | Assessment saves against correct academic context | ☐ |
| TEACH-05 | Teacher views results | Only permitted academic data is visible | ☐ |

### Gate E — Student workflow

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| STUD-01 | Student views profile | Own profile loads | ☐ |
| STUD-02 | Student views enrollment | Correct class/section is shown | ☐ |
| STUD-03 | Student views academic information | Only own permitted records are visible | ☐ |
| STUD-04 | Student cannot access admin area | Protected admin routes are denied | ☐ |

### Gate F — Parent workflow

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| PAR-01 | Parent views child | Correct linked child appears | ☐ |
| PAR-02 | Parent views academic information | Child-related data loads | ☐ |
| PAR-03 | Parent cannot access another student's data | Cross-student access is denied | ☐ |

### Gate G — Tenant isolation

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| ISO-01 | Bedrock user requests another school's record | Access denied / record not exposed | ☐ |
| ISO-02 | Bedrock admin queries school-scoped resources | Only Bedrock records returned | ☐ |
| ISO-03 | Cross-school ID manipulation | Backend rejects unauthorized tenant access | ☐ |
| ISO-04 | Platform super_admin accesses platform scope | Platform role remains separate from school tenant roles | ☐ |

### Gate H — Production and usability

| ID | Scenario | Expected result | Status |
|---|---|---|---|
| PROD-01 | Frontend loads production API | No CORS/network failure | ☐ |
| PROD-02 | API health check | Healthy response | ☐ |
| PROD-03 | Mobile login | Usable on phone-sized screen | ☐ |
| PROD-04 | Tablet workflow | Core workflows remain usable | ☐ |
| PROD-05 | Browser refresh while authenticated | Session/account state remains correct | ☐ |
| PROD-06 | API/server errors | User receives safe, useful error state | ☐ |

## 5. Defect log

| ID | Date | Severity | Area | Scenario | Expected | Actual | Owner | Status | Retest |
|---|---|---|---|---|---|---|---|---|---|
| DEF-001 | | Blocker/Critical/Major/Minor | | | | | | Open | |
| DEF-002 | | | | | | | | Open | |
| DEF-003 | | | | | | | | Open | |

### Severity guide

- **Blocker:** Pilot cannot proceed.
- **Critical:** Security, tenant isolation, authentication, data corruption, or major workflow failure.
- **Major:** Important pilot workflow fails but a controlled workaround exists.
- **Minor:** Cosmetic or low-impact usability issue.

## 6. Evidence to capture

For each failed scenario, record:

1. Test account/role (never the password).
2. Exact page or API operation.
3. Steps to reproduce.
4. Expected result.
5. Actual result.
6. Timestamp.
7. Browser/device.
8. Screenshot or screen recording where useful.
9. Relevant Render/GitHub evidence when it is a backend/deployment issue.

## 7. Pilot exit criteria

The Bedrock pilot should remain **in progress** until:

- All critical authentication scenarios pass.
- All pilot roles can log in and load their accounts.
- Academic session and term are correct.
- Pilot classes, sections and subjects are configured.
- Teacher coverage is sufficient for the workflows being tested.
- Student enrollment and parent relationships work.
- Attendance, assessment and result flows pass the agreed scenarios.
- Tenant isolation tests pass.
- No unresolved blocker or critical defect remains.
- Production frontend/API communication is stable.
- Defects found during pilot are either fixed and retested or explicitly accepted by the pilot owner.

## 8. Pilot status

**Current status: QA NOT STARTED**

Do not mark the pilot successful from infrastructure readiness alone. Record evidence for each gate above.

## 9. Sign-off

| Role | Name | Date | Result |
|---|---|---|---|
| Stonez Digital QA | | | ☐ Pass ☐ Conditional ☐ Fail |
| Bedrock School Admin | | | ☐ Pass ☐ Conditional ☐ Fail |
| Pilot Owner | | | ☐ Pass ☐ Conditional ☐ Fail |

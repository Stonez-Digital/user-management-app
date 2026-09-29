# Stonez Digital School Management System

A full-stack, multi-school school management platform developed by **Stonez Digital** to help schools manage administration, academics, finance, communication, teachers, students, parents, and operational workflows from one platform.

The project has evolved from a Go-based user-management application into a **multi-tenant School Management SaaS foundation** with production infrastructure, tenant isolation, academic workflows, finance, portals, communication, and automated quality controls.

## Current Product Level

**Current stage: Production Hardening / Pilot-Readiness**

The platform is beyond the MVP/CRUD stage. The current development focus is on **security, tenant isolation, data integrity, financial integrity, production verification, workflow reliability, and controlled real-school pilot preparation**.

The major product layers are established:

- Platform administration
- Multi-school tenant architecture
- School onboarding and approval
- School administration
- Authentication and session management
- Role-based access control
- Student enrollment
- Academic sessions, terms, classes, sections, subjects and academic workflows
- Teacher assignments and teacher workflows
- Assessments, results and report cards
- Finance administration
- Parent and student portals
- Guardian relationships
- Communication workflows
- Audit logging
- Responsive web interface
- Cloudflare frontend deployment
- Render Go/Gin API deployment
- PostgreSQL/Supabase production database
- Automated CI and production QA

The platform is now being treated as a **production-hardened pilot SaaS**, not simply as a feature-development project.

## Recent Development

### Production hardening and tenant isolation

The latest audit cycle focused on finding weaknesses and regression risks in the existing production system rather than rebuilding features that already work.

Recent work strengthened:

- Cross-school data isolation across student, academic, finance, guardian and portal workflows
- School-scoped reads and mutations
- Tenant-aware relationships and uniqueness rules
- Database-level protection for public-content tables
- Role/profile consistency
- School-scoped user and profile validation
- Academic session and term handling
- Financial integrity and active-enrollment requirements
- Guardian lifecycle and primary-guardian semantics

The tenant-isolation review traced application flows from controllers through services and repositories and checked both reads and mutations for school context.

### User and profile consistency

Teacher, parent and student account handling has received additional production hardening.

Recent changes include:

- Repairing missing parent profiles during relevant user processing
- Rejecting unsafe same-school user creation when the requested role conflicts with an existing role
- Preventing teacher/parent role changes that would leave incompatible profile records
- School-scoped teacher and parent profile validation
- Parent identifier uniqueness within a school
- Database-level unique protection for parent identifiers
- Verification that teacher and parent profile relationships remain consistent with user roles

Production verification found no remaining teacher/parent profile mismatches in the audited data set.

### Guardian relationship lifecycle

Guardian access was hardened to prevent stale relationships from granting access through inactive parent accounts.

Recent changes include:

- Explicit guardian-link deactivation
- Explicit guardian-link reactivation
- Reactivation restricted to an active same-school parent account
- Deactivation clears both active and primary status
- Atomic replacement of an existing primary guardian
- Database-level protection for one active primary guardian per student
- Guardian lookups requiring an active guardian user
- School-scoped guardian lifecycle operations
- Audit records for guardian lifecycle changes

Existing inactive-parent relationships were preserved as historical data; the authorization path now prevents inactive parent accounts from using those relationships.

### Financial integrity and payment trust boundary

Finance has undergone a dedicated integrity audit covering:

- Active enrollment requirements
- Closed/archived academic-session restrictions
- Cross-school invoice/payment protection
- Invoice row locking during payment creation
- Payment concurrency
- Duplicate payment references
- Invoice balance calculation
- Payment status handling
- Financial reporting coverage
- Payment trust boundaries

The latest finance hardening requires the normal payment-creation path to be an explicit **manual payment** path. A provider payment cannot simply be marked successful by supplying a provider name and succeeded status.

A separate verified-provider service path now:

- Requires a provider verification implementation
- Verifies the provider reference
- Requires a positive verified amount
- Requires a successful verified status
- Stores verified payment metadata
- Records the payment only after successful verification
- Reuses the existing transactional invoice/payment protections

The production finance smoke harness was updated to reflect the explicit manual-payment trust boundary.

> **Important:** provider verification infrastructure is now hardened at the service boundary, but concrete production payment-provider integration and currency verification remain separate implementation/verification work where applicable.

### Academic sessions and terms

Academic data is no longer treated as a fixed three-term system.

The platform supports:

- First Term
- Second Term
- Third Term
- Custom school-defined terms such as Michaelmas or other names

Term validation now accepts school-defined names rather than enforcing hardcoded term labels.

Financial operations also validate the associated academic session and prevent new financial activity against closed or archived sessions.

### Production QA and CI

The development workflow has matured into a consistent:

    Feature Branch
          |
    Implementation
          |
    Tests / Build
          |
    Pull Request
          |
    CI / Security / Production QA
          |
    Review
          |
    Merge to main
          |
    Render / Cloudflare deployment
          |
    Production verification

Recent hardening PRs have repeatedly passed:

- Go Tests
- Backend Checks
- Frontend Checks where applicable
- Security Checks
- Production QA
- Production-oriented finance smoke tests where applicable

Obsolete historical pull requests were also reviewed and closed rather than being merged blindly into the current production codebase.

## Current Product Capabilities

### Platform administration

- Platform-level administration
- School onboarding
- School approval workflow
- Platform operations monitoring
- Separation between Stonez Digital administration and school tenants

### School administration

- School dashboard
- School identity/context
- User management
- Role management
- Account activation/deactivation
- Administrative controls
- Audit logs
- School-scoped data access

### Authentication and security

- JWT authentication
- Access and refresh tokens
- Refresh-token rotation
- Refresh-token reuse protection
- Session management
- Logout
- Logout-all
- Password change
- Password reset
- Protected routes
- RBAC
- Authenticated account/profile loading
- Active-account enforcement
- Audit logging
- Tenant-aware authorization
- Production security and regression checks

### Academic management

- Academic sessions
- Academic terms
- Custom academic term names
- Classes
- Sections
- Subjects
- Student enrollment
- Teacher assignment
- Teacher academic workspace
- Teacher operational workflows
- Assessments
- Results
- Report cards
- Academic tenant isolation

### Student management

- Student profiles
- Session-scoped enrollment
- School-scoped student data
- Student portal
- Academic access foundation

### Teacher management

- Teacher profiles
- Teacher academic workspace
- Teacher operational workflows
- Assigned academic responsibilities
- Teacher-facing UI
- Role/profile consistency controls

### Parent and guardian management

- Parent profiles
- Parent portal
- Student/guardian relationships
- Guardian lifecycle controls
- Primary guardian rules
- Parent identifier uniqueness
- Active-parent access enforcement

### Finance

- Finance administration foundation
- Invoice management
- Enrollment-aware invoice creation
- Payment management
- Payment concurrency protection
- Payment reference uniqueness
- Manual-payment trust boundary
- Provider verification service boundary
- Financial reporting
- Session and enrollment integrity checks

### Communication

- Communication center
- School communication foundation
- Notifications and communication workflows

### Platform operations

- Platform monitoring foundation
- Operational visibility for the platform owner
- Administrative monitoring workflows
- Production QA workflows

## School Roles

The platform currently recognizes these major roles:

| Role | Scope | Purpose |
|---|---|---|
| super_admin | Platform | Stonez Digital platform administration |
| school_admin | School | School administration and management |
| teacher | School | Teaching and academic workflows |
| student | School | Student access and academic participation |
| parent | School | Parent/student monitoring |
| accountant | School | Finance administration |
| staff | School | General school operations |

Role access is enforced by the backend. School-scoped roles are isolated from other school tenants.

## Current Architecture

### Production architecture

    Internet
       |
       v
    Cloudflare Workers
    Next.js / vinext frontend
       |
       | HTTPS
       v
    Go / Gin API on Render
       |
    GORM + migrations
       |
       v
    PostgreSQL / Supabase

### Application layers

    Frontend
       |
       v
    Next.js / React / TypeScript
       |
       v
    HTTP API
       |
       v
    Gin Controllers
       |
       v
    Authentication / RBAC Middleware
       |
       v
    Service Layer
       |
       v
    Repository Layer
       |
       v
    GORM
       |
       v
    PostgreSQL / SQLite

## Technology Stack

### Backend

- Go 1.25+
- Gin
- GORM
- PostgreSQL
- SQLite
- JWT
- UUID
- bcrypt/password hashing

### Frontend

- Next.js 16
- React 19
- TypeScript
- Next.js App Router
- Responsive web UI

### Infrastructure

- Cloudflare Workers
- vinext
- Render
- PostgreSQL / Supabase PostgreSQL
- GitHub
- GitHub Actions

## Deployment

### Frontend

The production frontend is prepared for deployment on Cloudflare Workers.

Current Worker:

    stonez-school-management

Production URL:

    https://stonez-school-management.onojamondayojonugba.workers.dev/

### Backend

The Go/Gin API is deployed separately on Render.

Production API:

    https://stonez-digital-school-api.onrender.com

The backend is responsible for:

- Authentication
- Authorization
- RBAC
- Business logic
- Tenant isolation
- Database access
- Migrations
- School and academic workflows
- Financial integrity controls

### Database

Production database support is PostgreSQL.

Supabase PostgreSQL can be used as the managed PostgreSQL provider because the Go backend connects through a standard PostgreSQL connection string.

The frontend does **not** connect directly to PostgreSQL.

## Database and Tenant Isolation

The database layer includes production-oriented tenant controls:

- PostgreSQL migrations
- UUID identifiers
- School-scoped records
- school_id tenant boundaries
- Composite tenant-aware relationships
- Tenant-scoped unique indexes
- Controlled delete/update behavior
- School-scoped audit records
- Backend authorization checks
- RLS protection for protected public-content tables

The application audit has specifically checked cross-school access and mutation paths across:

- Students
- Enrollments
- Classes and sections
- Subjects
- Teacher assignments
- Attendance
- Assessments and results
- Academic sessions and terms
- Invoices and payments
- Guardian relationships
- Student/parent portal access
- Bulk-import workflows

The objective is to ensure that a user operating inside one school cannot access or mutate another school's protected data through normal application requests.

## Authentication Architecture

Authentication remains owned by the Go backend.

The platform does **not** currently depend on Supabase Auth.

    User
      |
    Next.js frontend
      |
    Go authentication API
      |
    JWT access/refresh session
      |
    RBAC + school tenant authorization
      |
    Protected application resources

This keeps authentication, authorization, and school-tenant rules in one backend security boundary.

## Development Workflow

Development follows a pull-request based workflow:

    Feature Branch
          |
    Implementation
          |
    Tests / Build
          |
    Pull Request
          |
    Code Review
          |
    CI Checks
          |
    Merge to main
          |
    Production Deployment
          |
    Production Verification

Production changes should go through pull requests and automated checks. A successful CI run is treated as a gate before merge, followed by deployment and live verification.

## CI and Quality Controls

The repository uses GitHub Actions for automated validation.

Current checks include:

- Go tests
- Frontend checks
- Backend checks
- Security checks
- Production QA workflows
- Frontend production builds
- Cloudflare deployment validation
- Production-oriented finance smoke testing

The quality process now emphasizes regression detection and production hardening in addition to feature correctness.

## Local Development

### Requirements

Install:

- Go 1.25+
- Node.js 22+
- npm
- PostgreSQL for production-style development, or SQLite for local development

### Backend

From the project root:

    go mod tidy
    go test ./...
    go run ./cmd/server

The API runs on:

    http://localhost:8080

### Frontend

    cd frontend
    npm install
    npm run dev

The dashboard runs on:

    http://localhost:3000

### Windows launcher

From PowerShell:

    .\start.ps1

## Current Release Readiness

The platform should currently be evaluated in four layers:

### 1. Product foundation — established

The major school-management domains and role model are established.

### 2. Architecture — production-oriented

Multi-tenancy, backend authorization, migrations, PostgreSQL support, Cloudflare frontend infrastructure, and a separate Go API are established.

### 3. Production hardening — substantially advanced

Recent work has hardened:

- Cross-school isolation
- Role/profile consistency
- Guardian access lifecycle
- Financial integrity
- Payment trust boundaries
- Academic session/term rules
- Database constraints
- CI and production QA

### 4. Pilot readiness — next major gate

The next stage is controlled real-school operation. This requires end-to-end validation of real workflows, operational monitoring, user feedback, support processes, and remaining production defects.

### 5. Commercial readiness — not yet complete

Before broad market launch, the platform still needs:

- Controlled real-school pilots
- Deeper workflow validation
- Customer onboarding processes
- Subscription/billing strategy
- Stronger reporting and analytics
- Support and maintenance processes
- Continued security and performance testing
- Operational scaling validation

## Immediate Priorities

The next development cycle should focus on **stabilization, verification, and pilot readiness rather than adding disconnected features**.

Priority order:

1. Verify the latest finance hardening deployment on Render.
2. Complete production role-by-role QA.
3. Continue multi-school isolation verification with representative school data.
4. Verify student, teacher, parent and accountant workflows end-to-end.
5. Complete production Cloudflare-to-Render communication checks.
6. Validate PostgreSQL migrations and production data integrity.
7. Complete responsive UI QA across phone, tablet and desktop widths.
8. Fix defects discovered during controlled pilot testing.
9. Improve operational reporting, support tooling and school administration workflows.
10. Run a controlled real-school pilot.
11. Capture pilot feedback before broader commercial deployment.

## Product Roadmap

### Phase 1 — Platform Foundation

- [x] Authentication
- [x] JWT and refresh sessions
- [x] RBAC
- [x] Audit logging
- [x] Database migrations
- [x] PostgreSQL support
- [x] Student management foundation
- [x] Administrative dashboard foundation

### Phase 2 — Academic Engine

- [x] Academic sessions and terms
- [x] Custom academic term names
- [x] Classes and sections
- [x] Subjects
- [x] Student enrollment
- [x] Teacher assignment
- [x] Assessments
- [x] Results
- [x] Report cards
- [x] Academic tenant isolation

### Phase 3 — Teacher Operations

- [x] Teacher academic workspace
- [x] Teacher operational workflows
- [x] Teacher-facing academic UI
- [x] Teacher/profile consistency controls

### Phase 4 — School Operations

- [x] Finance administration foundation
- [x] Parent portal
- [x] Student portal
- [x] Guardian relationship management
- [x] Communication center
- [x] Platform operations monitoring
- [x] School self-onboarding and platform approval

### Phase 5 — Production Hardening

- [x] Production authentication stabilization
- [x] Authenticated profile response contract
- [x] Responsive mobile/tablet UI
- [x] Cloudflare frontend deployment
- [x] Render Go API deployment
- [x] PostgreSQL production path
- [x] Separate platform and school login routing
- [x] School-wide tenant branding
- [x] School-branded report cards and report headers
- [x] School logo upload route and Cloudflare multipart proxy support
- [x] Production school-logos storage bucket configuration
- [x] Cross-school isolation audit and hardening
- [x] Teacher/parent profile consistency hardening
- [x] Parent identifier uniqueness
- [x] Guardian lifecycle and primary-guardian hardening
- [x] Financial integrity audit and hardening
- [x] Manual payment trust-boundary enforcement
- [x] Provider payment verification service boundary
- [x] Production finance smoke-test alignment
- [x] CI/security/production-QA regression gates
- [ ] Verify latest Render deployment and live finance behavior
- [ ] Complete end-to-end school logo upload QA
- [ ] Complete role-by-role production QA
- [ ] Complete representative multi-school isolation QA
- [ ] Complete controlled pilot readiness review

### Phase 6 — Commercial Expansion

- [ ] Real-school pilot
- [ ] Customer onboarding workflow
- [ ] Subscription/billing
- [ ] Advanced reporting
- [ ] Support and maintenance processes
- [ ] Product analytics
- [ ] Performance/scaling validation
- [ ] Broader commercial rollout

## Project Structure

    user-management-app/
    ├── cmd/
    │   └── server/
    ├── internal/
    │   ├── auth/
    │   ├── authz/
    │   ├── controller/
    │   ├── database/
    │   ├── httpx/
    │   ├── middleware/
    │   ├── models/
    │   ├── repository/
    │   └── service/
    ├── migrations/
    ├── frontend/
    ├── scripts/
    ├── .github/
    │   └── workflows/
    ├── start.ps1
    ├── go.mod
    ├── go.sum
    └── README.md

## Project Status

**Status: Active development — production hardening, pilot preparation, and operational verification**

The project has progressed from a user-management backend into a substantial multi-tenant school-management platform.

The major product domains are represented in the codebase. The current challenge is **not simply adding more modules**; it is making the existing modules reliable enough to operate together in real schools.

The immediate product goal is:

> **Stabilize → verify → pilot → learn → harden → commercialize.**

### Latest verified development checkpoint — 29 September 2026

Recent production-hardening work has included:

- Cross-school tenant-isolation audit across core academic, student, finance, guardian and portal domains.
- Database-level protection for selected public-content tables.
- Academic-session restrictions for financial operations.
- Flexible/custom academic term names.
- Financial reporting and payment-state hardening.
- Payment concurrency and duplicate-reference protection.
- Active-enrollment and school/session validation for financial operations.
- Teacher and parent profile consistency fixes.
- Parent identifier uniqueness within a school.
- Role-change protection for teacher/parent profile integrity.
- Guardian-link lifecycle hardening.
- Active-parent authorization for guardian access.
- One-active-primary-guardian database protection.
- Manual payment trust-boundary enforcement.
- Verified-provider payment service boundary.
- Production finance smoke-test alignment.
- Repeated successful Go, backend, security and production-QA CI gates.
- Review and cleanup of obsolete historical pull requests.
- PR #214 merged into main after all final CI gates passed.

Latest finance hardening merge:

    525e87d7ef190788d93097883f44999e79e0ee9b

The current platform state is best described as **production-hardened pilot preparation**. The next major proof point is controlled real-school usage rather than another large architectural rewrite.

## Roadmap Summary

    Platform Administration       ██████████  Established
    Multi-Tenant Architecture     ██████████  Established
    Authentication & Security     ██████████  Hardened
    Academic Management            ██████████  Established
    Teacher Workflows              ██████████  Established
    Finance Foundation             ██████████  Hardened
    Parent & Student Portals       █████████░  Established
    Guardian Management             █████████░  Hardened
    Communication Center           █████████░  Established
    Production Infrastructure      █████████░  Established
    Production QA / Hardening      █████████░  Advanced
    Real-School Pilot              ███░░░░░░░  Next
    Commercial SaaS                ░░░░░░░░░░  Future

## Author

**Onoja Monday Ojonugba**

Software Engineer & Founder, **Stonez Digital**

GitHub: https://github.com/Onoja217

---

Built by **Stonez Digital** with the goal of helping schools move from fragmented administration to a connected digital school management platform.

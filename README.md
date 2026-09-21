# Stonez Digital School Management System

A full-stack, multi-school school management platform being developed by **Stonez Digital** to help schools manage administration, academics, finance, communication, teachers, students, parents, and operational workflows from one platform.

The project has evolved from a Go-based user-management application into a **multi-tenant School Management SaaS foundation**. The current codebase has passed the foundational CRUD stage and now contains substantial school, academic, finance, portal, communication, tenant-isolation, authentication, and production-deployment work.

## Current Product Level

**Current stage: Production Hardening / Pilot-Readiness**

The platform is currently at the level of a **serious school-management product foundation**, rather than a simple demo or user-management CRUD application.

The major product layers are already present:

- Platform administration
- Multi-school tenant architecture
- School onboarding and approval
- School administration
- Authentication and session management
- Role-based access control
- Student enrollment
- Academic sessions, terms, classes, sections, subjects and academic workflows
- Teacher academic and operational workflows
- Assessments, results and report cards
- Finance administration foundation
- Parent portal
- Student portal
- Communication center
- Platform operations monitoring
- Responsive web interface
- Cloudflare frontend deployment
- Public Go/Gin API deployment
- PostgreSQL production database support
- Automated CI and production QA

The immediate work is no longer about proving that the platform can manage users. The priority is **production verification, workflow completeness, security, usability, data integrity, deployment reliability, and preparation for a controlled school pilot**.

### What the current product is

> **A multi-tenant digital operating system for schools that is approaching pilot deployment, with production infrastructure and the major school-management domains already established.**

It is not yet described as a fully mature commercial SaaS product. Real-school pilot usage, operational feedback, deeper reporting, billing/subscription workflows, support processes, and additional hardening are still required before broad commercial rollout.

## Recent Development

The repository has gone through a major expansion in the current development cycle.

### Academic management

The academic engine has progressed through:

- School and academic readiness
- Core academic tenant isolation
- Student enrollment
- Teacher assignment
- Assessment operations
- Results processing
- Report cards
- Teacher academic workspace
- Teacher operational workflows
- Teacher-facing academic UI

This establishes the academic workflow layer connecting schools, sessions, terms, classes, subjects, teachers, students, assessments, and results.

### School operations and portals

The product has also expanded into:

- Finance administration
- Parent portal
- Student portal
- Communication center
- Platform operations monitoring
- School self-onboarding and platform approval

These additions move the product toward a complete school operating platform rather than an administration dashboard.

### Multi-tenant platform architecture

The system now separates **Stonez Digital platform administration** from individual school tenants.

The architecture supports:

- Platform-level super_admin
- School-level administration
- School-scoped users and records
- School-scoped academic data
- School-scoped audit records
- Tenant-aware database relationships
- Tenant-scoped uniqueness and integrity controls
- School onboarding and approval workflows

This is an important architectural milestone because the same application can be used by multiple schools without treating all school data as one shared tenant.

### Production authentication hardening

Recent production work stabilized authentication across roles.

The current authentication layer includes:

- JWT access tokens
- Refresh tokens
- Refresh-token rotation
- Refresh-token reuse protection
- Active-account checks
- Password change and reset flows
- Logout and logout-all controls
- Protected routes
- RBAC
- Role-aware landing pages
- Explicit authenticated-account API responses
- Production fixes for /me profile loading
- School-admin and platform-admin separation

The recent authentication work specifically addressed a production issue where login could succeed while the authenticated account failed to load correctly in the frontend.

### Responsive web application

The web interface was refined for:

- Android phones
- iPhone-sized screens
- Tablets
- Desktop screens

The responsive work preserved the existing application structure and business logic while improving:

- Small-screen layouts
- Touch targets
- Safe-area handling
- Forms
- Cards
- Wide tables
- Horizontal overflow behavior

### Branding and theme work

The Stonez Digital application logo was added to the platform.

The later Light/Dark theme experiment was subsequently reverted because the color changes were not producing the intended result and were interfering with the desired visual presentation.

The current product therefore keeps the established platform styling and Stonez Digital branding while preserving the mobile responsiveness work.

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
- Reuse protection
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

### Academic management

- Academic sessions
- Academic terms
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
- Enrollment records
- School-scoped student data
- Student portal
- Academic access foundation

### Teacher management

- Teacher academic workspace
- Teacher operational workflows
- Assigned academic responsibilities
- Teacher-facing UI

### Parent management

- Parent portal
- Parent access to student-related information
- Parent/student relationship foundation

### Finance

- Finance administration foundation
- Finance-related school administration workflows

### Communication

- Communication center
- School communication foundation
- Notifications and communication workflows

### Platform operations

- Platform monitoring foundation
- Operational visibility for the platform owner
- Administrative monitoring workflows

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
    Go / Gin API
       |
    GORM + migrations
       |
       v
    PostgreSQL database

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

The frontend uses Cloudflare as the edge/web runtime and communicates with the Go API over HTTPS.

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

### Database

Production database support is PostgreSQL.

Supabase PostgreSQL can be used as the managed PostgreSQL provider because the Go backend connects through a standard PostgreSQL connection string.

The frontend does **not** connect directly to PostgreSQL.

## Database and Tenant Isolation

The database layer has been expanded beyond a simple shared user table.

Current production-oriented controls include:

- PostgreSQL migrations
- UUID identifiers
- School-scoped records
- school_id tenant boundaries
- Composite tenant-aware relationships
- Tenant-scoped unique indexes
- Controlled delete/update behavior
- School-scoped audit records
- Backend authorization checks

The objective is to ensure that a user operating inside one school cannot access another school's protected data through normal application requests.

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

The main branch is intended to be protected. Direct production changes should go through pull requests and automated checks.

Recommended protection rules include:

- Pull request required before merge
- At least one approval
- Required CI checks
- Branch must be up to date before merge
- Force pushes blocked
- Branch deletion blocked

## CI and Quality Controls

The repository uses GitHub Actions for automated validation.

Current checks include:

- Go tests
- Frontend checks
- Production QA workflows
- Frontend production builds
- Cloudflare deployment validation

Production changes should not be treated as complete until the relevant CI and deployment checks pass.

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

### 3. Production hardening — active

Authentication, tenant isolation, deployment behavior, responsive UI, CI, QA, and operational monitoring are still being verified and refined.

### 4. Commercial readiness — not yet complete

Before broad market launch, the platform still needs controlled real-school pilots, deeper workflow validation, customer onboarding, subscription/billing strategy, support processes, stronger reporting, and continued security/performance testing.

## Immediate Priorities

The next development cycle should focus on **stabilization rather than adding disconnected features**.

Priority order:

1. Protect main with GitHub branch rules.
2. Complete production authentication QA for all supported roles.
3. Verify school-admin tenant isolation with multiple schools.
4. Verify student, teacher, parent and accountant workflows end-to-end.
5. Verify production Cloudflare to Render API communication.
6. Validate PostgreSQL migrations and production data integrity.
7. Complete responsive UI QA across phone, tablet and desktop widths.
8. Fix production defects discovered during pilot testing.
9. Improve operational reporting and school administration workflows.
10. Prepare a controlled pilot with a real school.
11. Capture pilot feedback before broad commercial deployment.

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

### Phase 4 — School Operations

- [x] Finance administration foundation
- [x] Parent portal
- [x] Student portal
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
- [ ] Complete role-by-role production QA
- [ ] Complete multi-school isolation QA
- [ ] Complete pilot readiness review

### Phase 6 — Commercial Expansion

- [ ] Real-school pilot
- [ ] Customer onboarding workflow
- [ ] Subscription/billing
- [ ] Advanced reporting
- [ ] Support and maintenance processes
- [ ] Product analytics
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

**Status: Active development — production hardening and pilot preparation**

The project has progressed from a user-management backend into a substantial multi-tenant school-management platform.

The major product domains are now represented in the codebase. The current challenge is **not simply adding more modules**; it is making the existing modules reliable enough to operate together in a real school environment.

The immediate product goal is therefore:

> **Stabilize → verify → pilot → learn → harden → commercialize.**

## Roadmap Summary

    Platform Administration       ██████████  Established
    Multi-Tenant Architecture     ██████████  Established
    Authentication & Security     ██████████  Established
    Academic Management            ██████████  Established
    Teacher Workflows              ██████████  Established
    Finance Foundation             █████████░  Established
    Parent & Student Portals       █████████░  Established
    Communication Center           █████████░  Established
    Production Infrastructure      █████████░  Established
    Production QA / Hardening      ██████░░░░  In progress
    Real-School Pilot              ██░░░░░░░░  Next
    Commercial SaaS                ░░░░░░░░░░  Future

## Author

**Onoja Monday Ojonugba**

Software Engineer & Founder, **Stonez Digital**

GitHub: https://github.com/Onoja217

---

Built by **Stonez Digital** with the goal of helping schools move from fragmented administration to a connected digital school management platform.

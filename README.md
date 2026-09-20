# Stonez Digital School Management System

A full-stack school management platform being developed by **Stonez Digital** to help schools centralize administration, student records, academic operations, and communication in one system.

The project started as a Go-based user management API and has evolved into the foundation of a **market-ready School Management SaaS platform**. The current release combines a secure backend with an administrative web dashboard, while the next development phase focuses on the academic and operational workflows schools use every day.

## Current Product Level

**Current stage: School Management MVP — Foundation & Administration**

The platform has moved beyond basic user CRUD and now includes the core security, administration, student management, database, and dashboard foundations required for a real school management product.

### Current capabilities

- Secure user authentication
- JWT access and refresh token authentication
- Refresh-token rotation and reuse protection
- Logout and logout-all session controls
- Password change and password reset flows
- Role-based access control (RBAC)
- Seven school roles:
  - `super_admin`
  - `school_admin`
  - `teacher`
  - `student`
  - `parent`
  - `accountant`
  - `staff`
- Administrative user management
- Role assignment and account activation/deactivation
- Administrative audit logging
- Standardized API validation and error responses
- Versioned database migrations
- SQLite support for local development
- PostgreSQL support for production deployments
- Student profiles and enrollment records
- Student CRUD operations
- Next.js administrative dashboard
- Dashboard overview and school statistics
- Student search
- User and role management interface
- Audit log viewer
- Session expiry and sign-out handling
- GitHub Actions CI for backend and frontend checks
- One-command Windows local development launcher

## Product Vision

The goal is to build more than a user-management application.

Stonez Digital is developing this platform as a **complete digital operating system for schools**, allowing administrators, teachers, students, parents, and other staff to manage their daily activities from one central platform.

The long-term platform will cover:

**Administration → Students → Academics → Attendance → Assessments → Results → Fees → Timetable → Communication → Parent Access**

This creates an opportunity to package the system as a scalable school technology product that can be piloted with schools, refined from real-world usage, and eventually offered to multiple institutions.

## Architecture

### Backend

Built with Go using a layered architecture:

```
HTTP Request
     ↓
Gin Controller
     ↓
Middleware / RBAC
     ↓
Service Layer
     ↓
Repository Layer
     ↓
GORM
     ↓
Database
```

### Frontend

The administrative dashboard is being built with:

- Next.js 16
- React 19
- TypeScript
- Responsive dashboard UI
- Local API proxy during development

### Database

- PostgreSQL for production
- SQLite for local development
- Versioned migration system
- GORM ORM
- UUID-based identifiers

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

### Engineering & DevOps

- GitHub
- GitHub Actions
- Pull-request based development
- Protected main branch
- Automated backend testing
- Frontend production-build checks

## School Roles

| Role | Purpose |
|---|---|
| `super_admin` | Platform-level administration and security control |
| `school_admin` | School administration and user management |
| `teacher` | Teaching and future academic workflows |
| `student` | Student access and academic profile |
| `parent` | Future parent portal and student monitoring |
| `accountant` | Future financial and fee-management workflows |
| `staff` | General school staff operations |

Administrative permissions are enforced through middleware and the current database role is checked on authenticated requests so role changes take effect immediately.

## Authentication & Security

The authentication foundation includes:

- JWT access tokens
- Refresh tokens
- Refresh-token rotation
- Refresh-token reuse protection
- Active-account checks
- Password hashing
- Password change
- Password reset
- Logout
- Logout from all sessions
- Protected routes
- Role-based authorization
- Administrative permission checks
- Audit logging for sensitive administrative actions
- Environment-based secrets and database configuration

## Student Management

The current student management foundation supports:

- Student profiles
- Unique admission numbers
- Date of birth
- Gender
- Guardian information
- Enrollment status
- Student-to-user relationship
- Create, read, update, and delete operations
- Administrative student management endpoints

Current student management provides the foundation for the next academic modules.

## Administrative Dashboard

The web dashboard currently provides the foundation for school administration.

Current areas include:

- Dashboard overview
- Student statistics
- User statistics
- Active account statistics
- Role distribution
- Recent students
- Student search
- User and role management
- Account activation/deactivation
- Audit log viewing
- Session expiry handling
- Sign-out

The dashboard is currently being finalized and tested as the first major web administration release.

## API Endpoints

### Authentication

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `POST /auth/logout-all`
- `POST /auth/change-password`
- `POST /auth/forgot-password`
- `POST /auth/reset-password`

### Current User

- `GET /me`
- `PUT /me`
- `GET /sessions`
- `DELETE /sessions/:id`

### Administration

- `GET /admin/roles`
- `GET /admin/audit-logs`
- `POST /admin/users`
- `GET /admin/users`
- `GET /admin/users/:id`
- `PUT /admin/users/:id/role`
- `DELETE /admin/users/:id`
- `POST /admin/users/:id/activate`
- `POST /admin/users/:id/deactivate`

### Students

- `POST /admin/students`
- `GET /admin/students`
- `GET /admin/students/:id`
- `PUT /admin/students/:id`
- `DELETE /admin/students/:id`

## Database & Migrations

The application uses versioned database migrations rather than relying only on automatic schema creation.

Development:

```env
DB_DRIVER=sqlite
DB_PATH=users.db
```

Production:

```env
DB_DRIVER=postgres
DATABASE_URL=...
```

PostgreSQL can also be configured using:

```env
DB_HOST=
DB_PORT=5432
DB_USER=
DB_PASSWORD=
DB_NAME=
DB_SSLMODE=require
```

Never commit production secrets to the repository.

## Local Development

### Requirements

Install:

- Go 1.25+
- Node.js 22+
- npm
- PostgreSQL for production-style development, or SQLite for local development

### Backend

From the project root:

```bash
go mod tidy
go test ./...
go run ./cmd/server
```

The API runs on:

```
http://localhost:8080
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The dashboard runs on:

```
http://localhost:3000
```

### Windows One-Command Launcher

From the project root in PowerShell:

```powershell
.\\start.ps1
```

This starts the local Go API and Next.js frontend with development configuration.

## Cloudflare Deployment

The frontend is prepared for deployment to **Cloudflare Workers** using **vinext** for the current Next.js 16 stack.

### Current production architecture

```text
Browser
   ↓
Cloudflare Worker
   │  Next.js 16 + vinext
   │
   └── /backend/*
          ↓
      Go/Gin API
          ↓
      PostgreSQL
```

The Cloudflare Worker is **frontend/edge infrastructure only**. The Go/Gin API is deployed separately as a public HTTPS backend. The current production API origin is `https://stonez-digital-school-api.onrender.com`.

The frontend rewrite in `frontend/next.config.ts` maps `/backend/*` to `API_SERVER_URL/*`. Local development defaults to `http://localhost:8080`; production uses `API_SERVER_URL`.

### Cloudflare Worker

- Worker name: `stonez-school-management`
- Configuration: `frontend/wrangler.jsonc`
- Runtime entry: `vinext/server/fetch-handler`
- Compatibility flag: `nodejs_compat`
- Compatibility date: `2026-09-20`
- Observability: enabled
- Deployment: `npm run deploy`

From `frontend/`:

```bash
npm install
npm run build
npm run build:vinext
npm run cf-typegen
npm run deploy
```

Preview deployment:

```bash
npm run preview
```

### Automated deployment

Production deployment is defined in `.github/workflows/cloudflare-deploy.yml`.

A qualifying push to `main` runs the normal Next.js build, Cloudflare/vinext compatibility build, deployment configuration validation, Worker deployment, and Wrangler deployment verification.

The workflow uses the `production` GitHub environment and a concurrency lock.

Required production environment secrets/variables:

- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`
- `API_SERVER_URL`

`CLOUDFLARE_WORKER_URL` is not currently required by the workflow; deployment verification is performed through Wrangler against `stonez-school-management`.

### Cloudflare deployment boundary

Cloudflare Workers host the Next.js frontend. They are not the Go/Gin hosting environment for this project. The backend requires a Go-compatible server runtime and PostgreSQL.

The intended production separation is:

```text
Cloudflare Workers
    = Next.js frontend / edge

Render or another Go-compatible host
    = Go/Gin API

PostgreSQL / Supabase Postgres
    = relational database
```

## Supabase / PostgreSQL

The application currently uses **PostgreSQL as its production database layer**, with GORM and the project's versioned migration system. The repository does **not currently use the Supabase JavaScript client or Supabase Auth**.

Supabase can be used as the managed PostgreSQL provider because the backend connects through a standard PostgreSQL `DATABASE_URL`.

```text
Cloudflare Worker
       ↓
Go/Gin API
       ↓
Supabase PostgreSQL
```

### Current database model

- Local development: SQLite
- Production: PostgreSQL
- ORM: GORM
- Identifiers: UUID
- Application migrations: **20**
- Tenant isolation: school-scoped data using `school_id`
- PostgreSQL tenant integrity: composite foreign keys, tenant-scoped unique indexes, and controlled delete/update semantics
- Audit records: school-scoped using `school_id`

### Supabase configuration

If Supabase is selected as the production PostgreSQL provider, configure the backend with the Supabase Postgres connection string:

```env
DB_DRIVER=postgres
DATABASE_URL=...
```

Keep database credentials private. Never put the Supabase database password, service-role key, or other private credentials in frontend code or GitHub source.

The Go API should be the only application layer connecting to PostgreSQL. The Cloudflare frontend communicates with the API and does not connect directly to the database.

### Migrations and Supabase

The Go application's migration system owns the database schema:

```bash
go test ./...
go run ./cmd/server
```

When the API starts against PostgreSQL, pending application migrations are applied. The current schema includes the completed multi-school tenant boundary and audit-log isolation work.

**Important:** Supabase is currently a **PostgreSQL hosting option**, not the application's authentication provider. Authentication remains implemented by the Go backend using JWT access/refresh tokens, RBAC, and session controls.

### Production responsibility split

| Component | Responsibility |
|---|---|
| Cloudflare Workers | Next.js frontend, edge delivery and frontend routing |
| Go/Gin API | Authentication, RBAC, business logic and API |
| Supabase PostgreSQL | Managed PostgreSQL database, if selected |
| GitHub Actions | CI and Cloudflare deployment automation |

This keeps database credentials and school-tenant authorization inside the backend while allowing Cloudflare to serve the web application globally.

## Testing

Run backend tests:

```bash
go test ./...
```

Run the frontend production build:

```bash
cd frontend
npm run build
```

GitHub Actions runs automated checks for backend and frontend changes.

## Current Development Roadmap

### Phase 1 — Platform Foundation

- [x] Authentication
- [x] JWT and refresh tokens
- [x] RBAC
- [x] Audit logging
- [x] API validation
- [x] Database migrations
- [x] PostgreSQL support
- [x] Student management foundation
- [x] Administrative dashboard foundation
- [ ] Finalize and merge the current dashboard release
- [ ] Complete frontend automated checks

### Phase 2 — Academic Management

The next major product phase is the academic engine:

1. Academic sessions and terms
2. Classes and sections
3. Subjects
4. Teacher assignments
5. Student enrollment
6. Attendance
7. Assessments and examinations
8. Scores and grading
9. Results processing
10. Automated report cards

### Phase 3 — School Operations

- Fees and payment tracking
- Receipts
- Timetable management
- Notifications
- School announcements
- Parent portal
- Teacher workflows
- Student portal
- Administrative reporting

### Phase 4 — Commercial Product

Once the core platform is stable:

- Pilot with selected schools
- Collect operational feedback
- Improve onboarding
- Define subscription/pricing plans
- Build product demonstrations
- Establish customer support workflows
- Prepare production deployment
- Expand to additional schools

## Business Direction

The platform is being developed with a commercial product mindset.

Instead of selling isolated software development work, Stonez Digital can use this platform as a reusable **School Management SaaS product** that can be configured and deployed for different schools.

The business model can eventually support:

- School subscription plans
- Institution-based pricing
- Optional premium modules
- Implementation/onboarding services
- Custom integrations
- Support and maintenance packages

The immediate priority remains product quality: build the core workflows, test them with real school operations, and refine the platform before broad market deployment.

## Project Structure

```
user-management-app/
├── cmd/
│   └── server/
│       └── main.go
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
│   └── app/
├── scripts/
│   ├── dev.ps1
│   └── ...
├── .github/
│   └── workflows/
├── start.ps1
├── go.mod
├── go.sum
└── README.md
```

## Development Workflow

Development follows a pull-request based workflow:

```
Feature Branch
      ↓
Implementation
      ↓
Tests / Build
      ↓
Pull Request
      ↓
Code Review
      ↓
CI Checks
      ↓
Merge to main
```

The `main` branch is protected, and feature work should be developed through dedicated branches and pull requests.

## Project Status

**Status: Active development**

The system currently has a strong backend and administration foundation. The first web dashboard release is being finalized, after which development will move into the academic management engine.

The immediate objective is to transform the current foundation into a usable school platform that can support a pilot institution and provide a solid base for commercial expansion.

## Roadmap Summary

```
Authentication & Security       ██████████  Complete
RBAC & Administration           ██████████  Complete
Audit & Validation              ██████████  Complete
Database & PostgreSQL            ██████████  Complete
Student Management              █████████░  Foundation complete
Admin Dashboard                 ████████░░  In progress
Academic Management             ██░░░░░░░░  Next
School Operations               ░░░░░░░░░░  Planned
Commercial SaaS                 ░░░░░░░░░░  Planned
```

## Author

**Onoja Monday Ojonugba**

Software Engineer & Founder, **Stonez Digital**

GitHub: https://github.com/Onoja217

---

Built by **Stonez Digital** with the goal of helping schools move from fragmented administration to a connected digital school management platform.

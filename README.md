# User Management API (Go + Gin + PostgreSQL)

A RESTful User Management API built with Go (Golang), Gin framework, and GORM ORM, following a clean layered architecture (MVC-like).
The project supports full CRUD operations and is structured for scalability and production readiness.

## Features:
Create, read, update, and delete users (CRUD)
REST API built with Gin
PostgreSQL integration using GORM
Clean architecture (Controller → Service → Repository)
UUID-based user IDs
Environment-ready database configuration
Structured project layout for scalability

+ Tech Stack
Go 1.25+
Gin (HTTP framework)
GORM (ORM)
PostgreSQL
UUID (github.com/google/uuid)
## Project Structure
```
backend/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── controller/
│   ├── service/
│   ├── repository/
│   ├── models/
│   └── database/
├── migrations/
├── tests/
├── go.mod
└── go.sum
```
### Setup Instructions

1. Clone Repository
```
git clone https://github.com/Onoja217/user-management-app.git
```
```
cd user-management-app/backend
```
2. Install Dependencies
go mod tidy

3. Configure Database

### Update PostgreSQL connection in:

internal/database/postgres.go

### Example:

dsn := "host=localhost user=postgres password=postgres dbname=usersdb port=5432 sslmode=disable"

4. Run the Application
go run cmd/server/main.go

### Server runs on:
```
http://localhost:8080
```
### API Endpoints
Create User
POST /users

### Request body:
```
{
  "name": "John Doe",
  "email": "john@test.com"
}
Get All Users
GET /users
Get User by ID
GET /users/{id}
Update User
PUT /users/{id}
Delete User
DELETE /users/{id}
```
### Architecture Overview
```
Client → Controller → Service → Repository → Database
```
### Controller: 

+ Handles HTTP requests

### Service: 

+ Business logic

### Repository: 

+ Database operations

### Model: 

+ Data structures

### Database

PostgreSQL is used for persistence

### GORM handles migrations automatically:

db.AutoMigrate(&models.User{})

+ Testing

Tests are located in the tests/ folder.

### Run tests:

```
go test ./...
```
### Future Improvements
JWT Authentication
Swagger API documentation (/docs)
Docker support
AWS SQS messaging integration
React frontend
CI/CD pipeline

### Author

Built by Onoja217

## School RBAC

The API defines these roles:

- `super_admin`
- `school_admin`
- `teacher`
- `student`
- `parent`
- `accountant`
- `staff`

Administrative permissions are enforced by middleware rather than trusting the role in the JWT alone. The current database role is loaded on every authenticated request, so a role change takes effect immediately.

### Administrative endpoints

- `GET /admin/roles` — list roles and permissions
- `POST /admin/users` — create a user
- `GET /admin/users` — list users
- `GET /admin/users/:id` — view a user
- `PUT /admin/users/:id/role` — assign a role (super admin)
- `DELETE /admin/users/:id` — delete a user
- `POST /admin/users/:id/activate` — activate an account
- `POST /admin/users/:id/deactivate` — deactivate an account

Role changes are recorded with actor, target, previous role, new role, IP address, and timestamp.


### Production database

Production deployments can use PostgreSQL through GORM's PostgreSQL driver. Set `DB_DRIVER=postgres` and either provide `DATABASE_URL` or the `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `DB_SSLMODE` settings. SQLite remains available for local development with `DB_DRIVER=sqlite`.

The GORM PostgreSQL driver uses pgx underneath and supports a PostgreSQL DSN such as `host=localhost user=postgres password=... dbname=usersdb port=5432 sslmode=require`. citeturn1search1turn2view0

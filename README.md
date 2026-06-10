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
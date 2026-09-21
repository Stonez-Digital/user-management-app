package service

import (
    "errors"
    "strings"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/auth"
    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/gorm"
)

var ErrInvalidSchoolUserRole = errors.New("school administrators cannot provision this role")

type UserService struct { repo repository.UserRepository; db *gorm.DB }

func NewUserService(r repository.UserRepository, db *gorm.DB) *UserService { return &UserService{repo: r, db: db} }

func (s *UserService) CreateUser(schoolID uuid.UUID, name, email, password, role string) (models.User, error) {
    role = strings.TrimSpace(role)
    if role == "" { role = authz.RoleStudent }
    if !authz.IsValidRole(role) || role == authz.RoleSuperAdmin || role == authz.RoleSchoolAdmin {
        return models.User{}, ErrInvalidSchoolUserRole
    }
    hash, err := auth.HashPassword(password)
    if err != nil { return models.User{}, err }
    return s.repo.Create(schoolID, models.User{
        ID: uuid.New(), Name: strings.TrimSpace(name),
        Email: strings.ToLower(strings.TrimSpace(email)),
        PasswordHash: hash, Role: role, Active: true,
    })
}
func (s *UserService) GetUsers(schoolID uuid.UUID) ([]models.User, error) { return s.repo.GetAll(schoolID) }
func (s *UserService) GetUser(schoolID, id uuid.UUID) (models.User, error) { return s.repo.GetByID(schoolID, id) }
func (s *UserService) DeleteUser(schoolID, id uuid.UUID) error { return s.repo.Delete(schoolID, id) }
func (s *UserService) UpdateUser(schoolID uuid.UUID, user models.User) error { return s.repo.Update(schoolID, user) }
func (s *UserService) DB() *gorm.DB { return s.db }

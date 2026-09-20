package service

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/gorm"
)

type UserService struct { repo repository.UserRepository; db *gorm.DB }

func NewUserService(r repository.UserRepository, db *gorm.DB) *UserService { return &UserService{repo: r, db: db} }

func (s *UserService) CreateUser(schoolID uuid.UUID, name, email string) (models.User, error) {
	return s.repo.Create(schoolID, models.User{ID: uuid.New(), Name: name, Email: email})
}
func (s *UserService) GetUsers(schoolID uuid.UUID) ([]models.User, error) { return s.repo.GetAll(schoolID) }
func (s *UserService) GetUser(schoolID, id uuid.UUID) (models.User, error) { return s.repo.GetByID(schoolID, id) }
func (s *UserService) DeleteUser(schoolID, id uuid.UUID) error { return s.repo.Delete(schoolID, id) }
func (s *UserService) UpdateUser(schoolID uuid.UUID, user models.User) error { return s.repo.Update(schoolID, user) }
func (s *UserService) DB() *gorm.DB { return s.db }

package service

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/gorm"
)

type UserService struct {
	repo repository.UserRepository
	db *gorm.DB
}

func NewUserService(r repository.UserRepository, db *gorm.DB) *UserService {
	return &UserService{repo: r, db: db}
}

func (s *UserService) CreateUser(name, email string) (models.User, error) {
	user := models.User{
		ID:    uuid.New(),
		Name:  name,
		Email: email,
	}

	return s.repo.Create(user)
}

func (s *UserService) GetUsers() ([]models.User, error) {
	return s.repo.GetAll()
}

func (s *UserService) GetUser(id uuid.UUID) (models.User, error) {
	return s.repo.GetByID(id)
}

func (s *UserService) DeleteUser(id uuid.UUID) error {
	return s.repo.Delete(id)
}
func (s *UserService) UpdateUser(user models.User) error {
	return s.repo.Update(user)
}

func (s *UserService) DB() *gorm.DB { return s.db }

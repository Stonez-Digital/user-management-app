package service

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(r repository.UserRepository) *UserService {
	return &UserService{repo: r}
}

func (s *UserService) CreateUser(name, email string) models.User {
	user := models.User{
		ID:    uuid.New(),
		Name:  name,
		Email: email,
	}

	return s.repo.Create(user)
}

func (s *UserService) GetUsers() []models.User {
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

package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user models.User) models.User
	GetAll() []models.User
	GetByID(id uuid.UUID) (models.User, error)
	Delete(id uuid.UUID) error
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(user models.User) models.User {
	r.db.Create(&user)
	return user
}

func (r *userRepo) GetAll() []models.User {
	var users []models.User
	r.db.Find(&users)
	return users
}

func (r *userRepo) GetByID(id uuid.UUID) (models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	return user, err
}

func (r *userRepo) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.User{}, "id = ?", id).Error
}
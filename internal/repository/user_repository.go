package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user models.User) (models.User, error)
	GetAll() ([]models.User, error)
	GetByID(id uuid.UUID) (models.User, error)
	Delete(id uuid.UUID) error
	Update(user models.User) error
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepo{db: db}
}
func (r *userRepo) Update(user models.User) error {
	return r.db.Save(&user).Error
}

func (r *userRepo) Create(user models.User) (models.User, error) {
	if err := r.db.Create(&user).Error; err != nil { return user, err }
	return user, nil
}

func (r *userRepo) GetAll() ([]models.User, error) {
	var users []models.User
	if err := r.db.Find(&users).Error; err != nil { return nil, err }
	return users, nil
}

func (r *userRepo) GetByID(id uuid.UUID) (models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	return user, err
}

func (r *userRepo) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.User{}, "id = ?", id).Error
}
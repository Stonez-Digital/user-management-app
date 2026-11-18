package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(uuid.UUID, models.User) (models.User, error)
	GetAll(uuid.UUID) ([]models.User, error)
	GetByID(uuid.UUID, uuid.UUID) (models.User, error)
	Delete(uuid.UUID, uuid.UUID) error
	Update(uuid.UUID, models.User) error
}

type userRepo struct { db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepo{db: db} }

func (r *userRepo) Update(schoolID uuid.UUID, user models.User) error {
    user.SchoolID = &schoolID
    result := r.db.Model(&models.User{}).Where("school_id = ? AND id = ?", schoolID, user.ID).Updates(map[string]interface{}{
        "name":          user.Name,
        "email":         user.Email,
        "password_hash": user.PasswordHash,
        "role":          user.Role,
        "active":        user.Active,
    })
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected == 0 {
        return gorm.ErrRecordNotFound
    }
    return nil
}

func (r *userRepo) Create(schoolID uuid.UUID, user models.User) (models.User, error) {
	user.SchoolID = &schoolID
	if err := r.db.Create(&user).Error; err != nil { return user, err }
	return user, nil
}

func (r *userRepo) GetAll(schoolID uuid.UUID) ([]models.User, error) {
	var users []models.User
	if err := r.db.Where("school_id = ?", schoolID).Find(&users).Error; err != nil { return nil, err }
	return users, nil
}

func (r *userRepo) GetByID(schoolID, id uuid.UUID) (models.User, error) {
	var user models.User
	err := r.db.Where("school_id = ? AND id = ?", schoolID, id).First(&user).Error
	return user, err
}

func (r *userRepo) Delete(schoolID, id uuid.UUID) error {
	result := r.db.Where("school_id = ? AND id = ?", schoolID, id).Delete(&models.User{})
	if result.Error != nil { return result.Error }
	if result.RowsAffected == 0 { return gorm.ErrRecordNotFound }
	return nil
}

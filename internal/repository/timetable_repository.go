package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type TimetableRepository interface {
    Create(models.TimetableEntry)(models.TimetableEntry,error)
    List()([]models.TimetableEntry,error)
    Get(uuid.UUID)(models.TimetableEntry,error)
    Update(models.TimetableEntry)error
    Delete(uuid.UUID)error
}
type timetableRepo struct{db *gorm.DB}
func NewTimetableRepository(db *gorm.DB)TimetableRepository{return &timetableRepo{db}}
func(r *timetableRepo)Create(v models.TimetableEntry)(models.TimetableEntry,error){return v,r.db.Create(&v).Error}
func(r *timetableRepo)List()([]models.TimetableEntry,error){var v []models.TimetableEntry;e:=r.db.Order("day_of_week,start_time").Find(&v).Error;return v,e}
func(r *timetableRepo)Get(id uuid.UUID)(models.TimetableEntry,error){var v models.TimetableEntry;e:=r.db.First(&v,"id=?",id).Error;return v,e}
func(r *timetableRepo)Update(v models.TimetableEntry)error{return r.db.Save(&v).Error}
func(r *timetableRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.TimetableEntry{},"id=?",id).Error}
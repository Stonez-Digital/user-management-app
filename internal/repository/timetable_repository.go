package repository

import ("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type TimetableRepository interface{Create(uuid.UUID,models.TimetableEntry)(models.TimetableEntry,error);List(uuid.UUID)([]models.TimetableEntry,error);Get(uuid.UUID,uuid.UUID)(models.TimetableEntry,error);Update(uuid.UUID,models.TimetableEntry)error;Delete(uuid.UUID,uuid.UUID)error}
type timetableRepo struct{db *gorm.DB}
func NewTimetableRepository(db *gorm.DB)TimetableRepository{return &timetableRepo{db}}
func(r *timetableRepo)Create(schoolID uuid.UUID,v models.TimetableEntry)(models.TimetableEntry,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *timetableRepo)List(schoolID uuid.UUID)([]models.TimetableEntry,error){var v []models.TimetableEntry;e:=r.db.Where("school_id=?",schoolID).Order("day_of_week,start_time").Find(&v).Error;return v,e}
func(r *timetableRepo)Get(schoolID,id uuid.UUID)(models.TimetableEntry,error){var v models.TimetableEntry;e:=r.db.Where("school_id=? AND id=?",schoolID,id).First(&v).Error;return v,e}
func(r *timetableRepo)Update(schoolID uuid.UUID,v models.TimetableEntry)error{return r.db.Where("school_id=? AND id=?",schoolID,v.ID).Save(&v).Error}
func(r *timetableRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Where("school_id=? AND id=?",schoolID,id).Delete(&models.TimetableEntry{}).Error}
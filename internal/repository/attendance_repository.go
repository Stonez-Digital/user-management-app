package repository

import (
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)
type AttendanceRepository interface {
 Create(uuid.UUID, models.AttendanceRecord)(models.AttendanceRecord,error)
 List(uuid.UUID)([]models.AttendanceRecord,error)
 Get(uuid.UUID,uuid.UUID)(models.AttendanceRecord,error)
 Update(uuid.UUID,models.AttendanceRecord)error
 Delete(uuid.UUID,uuid.UUID)error
}
type attendanceRepo struct{db *gorm.DB}
func NewAttendanceRepository(db *gorm.DB)AttendanceRepository{return &attendanceRepo{db}}
func(r *attendanceRepo)Create(schoolID uuid.UUID,v models.AttendanceRecord)(models.AttendanceRecord,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *attendanceRepo)List(schoolID uuid.UUID)([]models.AttendanceRecord,error){var out []models.AttendanceRecord;e:=r.db.Where("school_id = ?",schoolID).Preload("Enrollment.Student.User","school_id = ?",schoolID).Preload("Enrollment.AcademicSession","school_id = ?",schoolID).Preload("Enrollment.SchoolClass","school_id = ?",schoolID).Preload("Enrollment.Section","school_id = ?",schoolID).Preload("Term","school_id = ?",schoolID).Order("date DESC").Find(&out).Error;return out,e}
func(r *attendanceRepo)Get(schoolID,id uuid.UUID)(models.AttendanceRecord,error){var v models.AttendanceRecord;e:=r.db.Where("id = ? AND school_id = ?",id,schoolID).Preload("Enrollment.Student.User","school_id = ?",schoolID).Preload("Enrollment.AcademicSession","school_id = ?",schoolID).Preload("Enrollment.SchoolClass","school_id = ?",schoolID).Preload("Enrollment.Section","school_id = ?",schoolID).Preload("Term","school_id = ?",schoolID).First(&v).Error;return v,e}
func(r *attendanceRepo)Update(schoolID uuid.UUID,v models.AttendanceRecord)error{return r.db.Model(&models.AttendanceRecord{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *attendanceRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Delete(&models.AttendanceRecord{},"id = ? AND school_id = ?",id,schoolID).Error}

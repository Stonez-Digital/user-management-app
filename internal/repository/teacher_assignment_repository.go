package repository

import("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type TeacherAssignmentRepository interface{Create(models.TeacherAssignment)(models.TeacherAssignment,error);List()([]models.TeacherAssignment,error);Get(uuid.UUID)(models.TeacherAssignment,error);Update(models.TeacherAssignment)error;Delete(uuid.UUID)error}
type teacherAssignmentRepo struct{db *gorm.DB}
func NewTeacherAssignmentRepository(db *gorm.DB)TeacherAssignmentRepository{return &teacherAssignmentRepo{db}}
func(r *teacherAssignmentRepo)Create(v models.TeacherAssignment)(models.TeacherAssignment,error){e:=r.db.Create(&v).Error;return v,e}
func(r *teacherAssignmentRepo)List()([]models.TeacherAssignment,error){var v []models.TeacherAssignment;e:=r.db.Preload("Teacher").Preload("Subject").Order("created_at DESC").Find(&v).Error;return v,e}
func(r *teacherAssignmentRepo)Get(id uuid.UUID)(models.TeacherAssignment,error){var v models.TeacherAssignment;e:=r.db.Preload("Teacher").Preload("Subject").First(&v,"id = ?",id).Error;return v,e}
func(r *teacherAssignmentRepo)Update(v models.TeacherAssignment)error{return r.db.Save(&v).Error}
func(r *teacherAssignmentRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.TeacherAssignment{},"id = ?",id).Error}

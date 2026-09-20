package repository

import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")

type AcademicSessionRepository interface { Create(uuid.UUID, models.AcademicSession)(models.AcademicSession,error); GetAll(uuid.UUID)([]models.AcademicSession,error); GetByID(uuid.UUID,uuid.UUID)(models.AcademicSession,error); Update(uuid.UUID,models.AcademicSession)error; Delete(uuid.UUID,uuid.UUID)error }
type academicSessionRepo struct{db *gorm.DB}
func NewAcademicSessionRepository(db *gorm.DB) AcademicSessionRepository{return &academicSessionRepo{db}}
func(r *academicSessionRepo)Create(schoolID uuid.UUID,s models.AcademicSession)(models.AcademicSession,error){s.SchoolID=schoolID;return s,r.db.Create(&s).Error}
func(r *academicSessionRepo)GetAll(schoolID uuid.UUID)([]models.AcademicSession,error){var v []models.AcademicSession;err:=r.db.Where("school_id = ?",schoolID).Preload("Terms", "school_id = ?",schoolID).Order("start_date DESC").Find(&v).Error;return v,err}
func(r *academicSessionRepo)GetByID(schoolID,id uuid.UUID)(models.AcademicSession,error){var v models.AcademicSession;err:=r.db.Where("school_id = ?",schoolID).Preload("Terms", "school_id = ?",schoolID).First(&v,"id = ?",id).Error;return v,err}
func(r *academicSessionRepo)Update(schoolID uuid.UUID,v models.AcademicSession)error{v.SchoolID=schoolID;return r.db.Model(&models.AcademicSession{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *academicSessionRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Where("school_id = ?",schoolID).Delete(&models.AcademicSession{},"id = ?",id).Error}

type TermRepository interface { Create(uuid.UUID,models.Term)(models.Term,error); GetAll(uuid.UUID,uuid.UUID)([]models.Term,error); GetByID(uuid.UUID,uuid.UUID)(models.Term,error); Update(uuid.UUID,models.Term)error; Delete(uuid.UUID,uuid.UUID)error }
type termRepo struct{db *gorm.DB}
func NewTermRepository(db *gorm.DB) TermRepository{return &termRepo{db}}
func(r *termRepo)Create(schoolID uuid.UUID,v models.Term)(models.Term,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *termRepo)GetAll(schoolID,sid uuid.UUID)([]models.Term,error){var v []models.Term;err:=r.db.Where("school_id = ? AND academic_session_id = ?",schoolID,sid).Order("start_date ASC").Find(&v).Error;return v,err}
func(r *termRepo)GetByID(schoolID,id uuid.UUID)(models.Term,error){var v models.Term;err:=r.db.Where("school_id = ?",schoolID).First(&v,"id = ?",id).Error;return v,err}
func(r *termRepo)Update(schoolID uuid.UUID,v models.Term)error{v.SchoolID=schoolID;return r.db.Model(&models.Term{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *termRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Where("school_id = ?",schoolID).Delete(&models.Term{},"id = ?",id).Error}

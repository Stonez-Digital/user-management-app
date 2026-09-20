package repository

import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")

type ClassRepository interface { Create(uuid.UUID,models.SchoolClass)(models.SchoolClass,error); List(uuid.UUID)([]models.SchoolClass,error); Get(uuid.UUID,uuid.UUID)(models.SchoolClass,error); Update(uuid.UUID,models.SchoolClass)error; Delete(uuid.UUID,uuid.UUID)error }
type classRepo struct{db *gorm.DB}
func NewClassRepository(db *gorm.DB) ClassRepository{return &classRepo{db}}
func(r *classRepo)Create(schoolID uuid.UUID,v models.SchoolClass)(models.SchoolClass,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *classRepo)List(schoolID uuid.UUID)([]models.SchoolClass,error){var v []models.SchoolClass;e:=r.db.Where("school_id = ?",schoolID).Preload("Sections","school_id = ?",schoolID).Order("level ASC, name ASC").Find(&v).Error;return v,e}
func(r *classRepo)Get(schoolID,id uuid.UUID)(models.SchoolClass,error){var v models.SchoolClass;e:=r.db.Where("school_id = ?",schoolID).Preload("Sections","school_id = ?",schoolID).First(&v,"id = ?",id).Error;return v,e}
func(r *classRepo)Update(schoolID uuid.UUID,v models.SchoolClass)error{v.SchoolID=schoolID;return r.db.Model(&models.SchoolClass{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *classRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Where("school_id = ?",schoolID).Delete(&models.SchoolClass{},"id = ?",id).Error}

type SectionRepository interface { Create(uuid.UUID,models.Section)(models.Section,error); List(uuid.UUID,uuid.UUID)([]models.Section,error); Get(uuid.UUID,uuid.UUID)(models.Section,error); Update(uuid.UUID,models.Section)error; Delete(uuid.UUID,uuid.UUID)error }
type sectionRepo struct{db *gorm.DB}
func NewSectionRepository(db *gorm.DB) SectionRepository{return &sectionRepo{db}}
func(r *sectionRepo)Create(schoolID uuid.UUID,v models.Section)(models.Section,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *sectionRepo)List(schoolID,classID uuid.UUID)([]models.Section,error){var v []models.Section;e:=r.db.Where("school_id = ? AND class_id = ?",schoolID,classID).Order("name ASC").Find(&v).Error;return v,e}
func(r *sectionRepo)Get(schoolID,id uuid.UUID)(models.Section,error){var v models.Section;e:=r.db.Where("school_id = ?",schoolID).First(&v,"id = ?",id).Error;return v,e}
func(r *sectionRepo)Update(schoolID uuid.UUID,v models.Section)error{v.SchoolID=schoolID;return r.db.Model(&models.Section{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *sectionRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Where("school_id = ?",schoolID).Delete(&models.Section{},"id = ?",id).Error}

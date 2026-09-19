package repository

import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")

type ClassRepository interface { Create(models.SchoolClass)(models.SchoolClass,error); List()([]models.SchoolClass,error); Get(uuid.UUID)(models.SchoolClass,error); Update(models.SchoolClass) error; Delete(uuid.UUID) error }
type classRepo struct{db *gorm.DB}
func NewClassRepository(db *gorm.DB) ClassRepository{return &classRepo{db}}
func(r *classRepo)Create(v models.SchoolClass)(models.SchoolClass,error){return v,r.db.Create(&v).Error}
func(r *classRepo)List()([]models.SchoolClass,error){var v []models.SchoolClass;e:=r.db.Preload("Sections").Order("level ASC, name ASC").Find(&v).Error;return v,e}
func(r *classRepo)Get(id uuid.UUID)(models.SchoolClass,error){var v models.SchoolClass;e:=r.db.Preload("Sections").First(&v,"id = ?",id).Error;return v,e}
func(r *classRepo)Update(v models.SchoolClass)error{return r.db.Save(&v).Error}
func(r *classRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.SchoolClass{},"id = ?",id).Error}

type SectionRepository interface { Create(models.Section)(models.Section,error); List(uuid.UUID)([]models.Section,error); Get(uuid.UUID)(models.Section,error); Update(models.Section)error; Delete(uuid.UUID)error }
type sectionRepo struct{db *gorm.DB}
func NewSectionRepository(db *gorm.DB) SectionRepository{return &sectionRepo{db}}
func(r *sectionRepo)Create(v models.Section)(models.Section,error){return v,r.db.Create(&v).Error}
func(r *sectionRepo)List(classID uuid.UUID)([]models.Section,error){var v []models.Section;e:=r.db.Where("class_id = ?",classID).Order("name ASC").Find(&v).Error;return v,e}
func(r *sectionRepo)Get(id uuid.UUID)(models.Section,error){var v models.Section;e:=r.db.First(&v,"id = ?",id).Error;return v,e}
func(r *sectionRepo)Update(v models.Section)error{return r.db.Save(&v).Error}
func(r *sectionRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.Section{},"id = ?",id).Error}

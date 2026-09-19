
package repository
import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")
type AcademicSessionRepository interface { Create(models.AcademicSession)(models.AcademicSession,error); GetAll()([]models.AcademicSession,error); GetByID(uuid.UUID)(models.AcademicSession,error); Update(models.AcademicSession)error; Delete(uuid.UUID)error }
type academicSessionRepo struct{db *gorm.DB}
func NewAcademicSessionRepository(db *gorm.DB) AcademicSessionRepository{return &academicSessionRepo{db}}
func(r *academicSessionRepo)Create(s models.AcademicSession)(models.AcademicSession,error){return s,r.db.Create(&s).Error}
func(r *academicSessionRepo)GetAll()([]models.AcademicSession,error){var v []models.AcademicSession;err:=r.db.Preload("Terms").Order("start_date DESC").Find(&v).Error;return v,err}
func(r *academicSessionRepo)GetByID(id uuid.UUID)(models.AcademicSession,error){var v models.AcademicSession;err:=r.db.Preload("Terms").First(&v,"id = ?",id).Error;return v,err}
func(r *academicSessionRepo)Update(v models.AcademicSession)error{return r.db.Save(&v).Error}
func(r *academicSessionRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.AcademicSession{},"id = ?",id).Error}
type TermRepository interface { Create(models.Term)(models.Term,error); GetAll(uuid.UUID)([]models.Term,error); GetByID(uuid.UUID)(models.Term,error); Update(models.Term)error; Delete(uuid.UUID)error }
type termRepo struct{db *gorm.DB}
func NewTermRepository(db *gorm.DB) TermRepository{return &termRepo{db}}
func(r *termRepo)Create(v models.Term)(models.Term,error){return v,r.db.Create(&v).Error}
func(r *termRepo)GetAll(sid uuid.UUID)([]models.Term,error){var v []models.Term;err:=r.db.Where("academic_session_id = ?",sid).Order("start_date ASC").Find(&v).Error;return v,err}
func(r *termRepo)GetByID(id uuid.UUID)(models.Term,error){var v models.Term;err:=r.db.First(&v,"id = ?",id).Error;return v,err}
func(r *termRepo)Update(v models.Term)error{return r.db.Save(&v).Error}
func(r *termRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.Term{},"id = ?",id).Error}

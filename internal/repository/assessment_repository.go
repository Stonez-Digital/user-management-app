package repository
import("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type AssessmentRepository interface{Create(models.Assessment)(models.Assessment,error);List()([]models.Assessment,error);Get(uuid.UUID)(models.Assessment,error);Update(models.Assessment)error;Delete(uuid.UUID)error}
type assessmentRepo struct{db *gorm.DB}
func NewAssessmentRepository(db *gorm.DB)AssessmentRepository{return &assessmentRepo{db}}
func(r *assessmentRepo)Create(v models.Assessment)(models.Assessment,error){return v,r.db.Create(&v).Error}
func(r *assessmentRepo)List()([]models.Assessment,error){var v []models.Assessment;e:=r.db.Order("assessment_date DESC").Find(&v).Error;return v,e}
func(r *assessmentRepo)Get(id uuid.UUID)(models.Assessment,error){var v models.Assessment;e:=r.db.First(&v,"id = ?",id).Error;return v,e}
func(r *assessmentRepo)Update(v models.Assessment)error{return r.db.Save(&v).Error}
func(r *assessmentRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.Assessment{},"id = ?",id).Error}
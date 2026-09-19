package repository
import("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type AssessmentResultRepository interface{Create(models.AssessmentResult)(models.AssessmentResult,error);List()([]models.AssessmentResult,error);Get(uuid.UUID)(models.AssessmentResult,error);Update(models.AssessmentResult)error;Delete(uuid.UUID)error}
type assessmentResultRepo struct{db *gorm.DB}
func NewAssessmentResultRepository(db *gorm.DB)AssessmentResultRepository{return &assessmentResultRepo{db}}
func(r *assessmentResultRepo)Create(v models.AssessmentResult)(models.AssessmentResult,error){return v,r.db.Create(&v).Error}
func(r *assessmentResultRepo)List()([]models.AssessmentResult,error){var v []models.AssessmentResult;e:=r.db.Order("created_at DESC").Find(&v).Error;return v,e}
func(r *assessmentResultRepo)Get(id uuid.UUID)(models.AssessmentResult,error){var v models.AssessmentResult;e:=r.db.First(&v,"id = ?",id).Error;return v,e}
func(r *assessmentResultRepo)Update(v models.AssessmentResult)error{return r.db.Save(&v).Error}
func(r *assessmentResultRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.AssessmentResult{},"id = ?",id).Error}
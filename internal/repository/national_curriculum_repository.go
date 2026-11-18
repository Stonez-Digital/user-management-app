package repository
import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")
type NationalCurriculumRepository interface {
 List()([]models.NationalCurriculum,error); Get(uuid.UUID)(models.NationalCurriculum,error)
 Create(models.NationalCurriculum)(models.NationalCurriculum,error)
}
type nationalCurriculumRepo struct{db *gorm.DB}
func NewNationalCurriculumRepository(db *gorm.DB) NationalCurriculumRepository{return &nationalCurriculumRepo{db}}
func(r *nationalCurriculumRepo)List()([]models.NationalCurriculum,error){var v []models.NationalCurriculum; e:=r.db.Preload("Levels.Classes.Subjects").Order("created_at DESC").Find(&v).Error; return v,e}
func(r *nationalCurriculumRepo)Get(id uuid.UUID)(models.NationalCurriculum,error){var v models.NationalCurriculum; e:=r.db.Preload("Levels.Classes.Subjects").First(&v,"id = ?",id).Error; return v,e}
func(r *nationalCurriculumRepo)Create(v models.NationalCurriculum)(models.NationalCurriculum,error){return v,r.db.Create(&v).Error}

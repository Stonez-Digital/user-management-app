package service
import ("errors"; "strings"; "github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "github.com/onoja217/users-management-app/internal/repository")
var ErrInvalidNationalCurriculum=errors.New("invalid national curriculum")
type NationalCurriculumService struct{repo repository.NationalCurriculumRepository}
func NewNationalCurriculumService(repo repository.NationalCurriculumRepository)*NationalCurriculumService{return &NationalCurriculumService{repo}}
func(s *NationalCurriculumService)List()([]models.NationalCurriculum,error){return s.repo.List()}
func(s *NationalCurriculumService)Get(id string)(models.NationalCurriculum,error){v,e:=uuid.Parse(strings.TrimSpace(id));if e!=nil{return models.NationalCurriculum{},ErrInvalidNationalCurriculum};return s.repo.Get(v)}
func(s *NationalCurriculumService)Create(v models.NationalCurriculum)(models.NationalCurriculum,error){v.Code=strings.ToUpper(strings.TrimSpace(v.Code));v.Name=strings.TrimSpace(v.Name);v.Authority=strings.TrimSpace(v.Authority);v.Version=strings.TrimSpace(v.Version);if v.Code==""||v.Name==""||v.Authority==""||v.Version==""{return v,ErrInvalidNationalCurriculum};if v.Status==""{v.Status=models.NationalCurriculumStatusDraft};switch v.Status{case models.NationalCurriculumStatusDraft,models.NationalCurriculumStatusActive,models.NationalCurriculumStatusRetired:default:return v,ErrInvalidNationalCurriculum};return s.repo.Create(v)}

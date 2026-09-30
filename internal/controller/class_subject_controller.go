package controller

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/service"
)

type ClassSubjectController struct{ service *service.ClassSubjectService }
func NewClassSubjectController(s *service.ClassSubjectService) *ClassSubjectController { return &ClassSubjectController{s} }

type classSubjectRequest struct {
    AcademicSessionID uuid.UUID `json:"academic_session_id" binding:"required"`
    ClassID uuid.UUID `json:"class_id" binding:"required"`
    SubjectID uuid.UUID `json:"subject_id" binding:"required"`
    CurriculumVersion string `json:"curriculum_version" binding:"required,max=100"`
    Category string `json:"category" binding:"required,max=20"`
    Required bool `json:"required"`
    SelectionGroup string `json:"selection_group" binding:"max=50"`
    Active *bool `json:"active"`
}

func classSubjectError(c *gin.Context, e error) {
    switch {
    case errors.Is(e, service.ErrClassSubjectNotFound):
        httpx.Error(c, 404, "class_subject_not_found", "class subject not found")
    case errors.Is(e, service.ErrClassSubjectDuplicate):
        httpx.Error(c, 409, "class_subject_duplicate", "subject is already mapped to this class for this curriculum version")
    case errors.Is(e, service.ErrClassSubjectInvalid):
        httpx.Error(c, 400, "invalid_class_subject", "invalid class-subject mapping or cross-school relationship")
    default:
        httpx.Error(c, 500, "class_subject_operation_failed", "class-subject operation failed")
    }
}

func (ctrl *ClassSubjectController) List(c *gin.Context) {
    schoolID, ok := requireSchoolID(c); if !ok { return }
    sessionID, err := uuid.Parse(c.Query("academic_session_id"))
    if err != nil { httpx.Error(c, 400, "invalid_academic_session_id", "academic_session_id is required"); return }
    var classID *uuid.UUID
    if raw := c.Query("class_id"); raw != "" {
        id, e := uuid.Parse(raw); if e != nil { httpx.Error(c, 400, "invalid_class_id", "invalid class_id"); return }
        classID = &id
    }
    v, err := ctrl.service.List(schoolID, sessionID, classID)
    if err != nil { classSubjectError(c, err); return }
    c.JSON(http.StatusOK, v)
}

func (ctrl *ClassSubjectController) Get(c *gin.Context) {
    schoolID, ok := requireSchoolID(c); if !ok { return }
    id, err := uuid.Parse(c.Param("id")); if err != nil { httpx.Error(c, 400, "invalid_class_subject_id", "invalid class-subject id"); return }
    v, err := ctrl.service.Get(schoolID, id)
    if err != nil { classSubjectError(c, err); return }
    c.JSON(http.StatusOK, v)
}

func (ctrl *ClassSubjectController) Create(c *gin.Context) {
    schoolID, ok := requireSchoolID(c); if !ok { return }
    var r classSubjectRequest
    if err := c.ShouldBindJSON(&r); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
    active := true; if r.Active != nil { active = *r.Active }
    v, err := ctrl.service.Create(schoolID, models.ClassSubject{
        AcademicSessionID:r.AcademicSessionID, ClassID:r.ClassID, SubjectID:r.SubjectID,
        CurriculumVersion:r.CurriculumVersion, Category:r.Category, Required:r.Required,
        SelectionGroup:r.SelectionGroup, Active:active,
    })
    if err != nil { classSubjectError(c, err); return }
    actor,_ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(), c, &actor, "class_subject.create", "class_subject", &v.ID, nil)
    c.JSON(http.StatusCreated, v)
}

func (ctrl *ClassSubjectController) Update(c *gin.Context) {
    schoolID, ok := requireSchoolID(c); if !ok { return }
    id, err := uuid.Parse(c.Param("id")); if err != nil { httpx.Error(c, 400, "invalid_class_subject_id", "invalid class-subject id"); return }
    v, err := ctrl.service.Get(schoolID, id); if err != nil { classSubjectError(c, err); return }
    var r classSubjectRequest
    if err = c.ShouldBindJSON(&r); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
    v.AcademicSessionID,v.ClassID,v.SubjectID = r.AcademicSessionID,r.ClassID,r.SubjectID
    v.CurriculumVersion,v.Category,v.Required,v.SelectionGroup = r.CurriculumVersion,r.Category,r.Required,r.SelectionGroup
    if r.Active != nil { v.Active = *r.Active }
    if err = ctrl.service.Update(schoolID, v); err != nil { classSubjectError(c, err); return }
    actor,_ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(), c, &actor, "class_subject.update", "class_subject", &v.ID, nil)
    c.JSON(http.StatusOK, v)
}

func (ctrl *ClassSubjectController) Delete(c *gin.Context) {
    schoolID, ok := requireSchoolID(c); if !ok { return }
    id, err := uuid.Parse(c.Param("id")); if err != nil { httpx.Error(c, 400, "invalid_class_subject_id", "invalid class-subject id"); return }
    if err = ctrl.service.Delete(schoolID, id); err != nil { classSubjectError(c, err); return }
    actor,_ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(), c, &actor, "class_subject.delete", "class_subject", &id, nil)
    c.JSON(http.StatusOK, gin.H{"message":"class-subject mapping deleted"})
}

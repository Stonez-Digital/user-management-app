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

type AssessmentResultController struct{ service *service.AssessmentResultService }

func NewAssessmentResultController(s *service.AssessmentResultService) *AssessmentResultController { return &AssessmentResultController{service: s} }

type assessmentResultRequest struct {
    AssessmentID uuid.UUID `json:"assessment_id"`
    StudentEnrollmentID uuid.UUID `json:"student_enrollment_id"`
    Score float64 `json:"score"`
}

func assessmentResultError(c *gin.Context, e error) {
    switch {
    case errors.Is(e, service.ErrResultNotFound): httpx.Error(c, 404, "result_not_found", "assessment result not found")
    case errors.Is(e, service.ErrResultAssessmentMissing): httpx.Error(c, 404, "assessment_not_found", "assessment not found")
    case errors.Is(e, service.ErrResultEnrollmentMissing): httpx.Error(c, 404, "enrollment_not_found", "student enrollment not found")
    case errors.Is(e, service.ErrResultDuplicate): httpx.Error(c, 409, "result_duplicate", "result already exists")
    case errors.Is(e, service.ErrResultInvalid): httpx.Error(c, 400, "invalid_result", "invalid assessment result")
    default: httpx.Error(c, 500, "result_operation_failed", "assessment result operation failed")
    }
}

func (ctrl *AssessmentResultController) List(c *gin.Context) {
    v, e := ctrl.service.List(); if e != nil { assessmentResultError(c, e); return }; c.JSON(http.StatusOK, v)
}

func (ctrl *AssessmentResultController) Get(c *gin.Context) {
    id, e := uuid.Parse(c.Param("id")); if e != nil { httpx.Error(c, 400, "invalid_result_id", "invalid result id"); return }
    v, e := ctrl.service.Get(id); if e != nil { assessmentResultError(c, e); return }; c.JSON(http.StatusOK, v)
}

func (ctrl *AssessmentResultController) Create(c *gin.Context) {
    var r assessmentResultRequest
    if e := c.ShouldBindJSON(&r); e != nil || r.AssessmentID == uuid.Nil || r.StudentEnrollmentID == uuid.Nil || r.Score < 0 {
        httpx.Error(c, 400, "invalid_result_request", "assessment, enrollment and non-negative score are required"); return
    }
    v, e := ctrl.service.Create(models.AssessmentResult{AssessmentID:r.AssessmentID, StudentEnrollmentID:r.StudentEnrollmentID, Score:r.Score})
    if e != nil { assessmentResultError(c, e); return }
    actor, _ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(), c, &actor, "assessment_result.create", "assessment_result", &v.ID, nil)
    c.JSON(http.StatusCreated, v)
}

func (ctrl *AssessmentResultController) Update(c *gin.Context) {
    id, e := uuid.Parse(c.Param("id")); if e != nil { httpx.Error(c, 400, "invalid_result_id", "invalid result id"); return }
    var r assessmentResultRequest
    if e = c.ShouldBindJSON(&r); e != nil || r.AssessmentID == uuid.Nil || r.StudentEnrollmentID == uuid.Nil || r.Score < 0 {
        httpx.Error(c, 400, "invalid_result_request", "assessment, enrollment and non-negative score are required"); return
    }
    v, e := ctrl.service.Get(id); if e != nil { assessmentResultError(c, e); return }
    v.AssessmentID=r.AssessmentID; v.StudentEnrollmentID=r.StudentEnrollmentID; v.Score=r.Score
    if e = ctrl.service.Update(v); e != nil { assessmentResultError(c, e); return }
    actor, _ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(), c, &actor, "assessment_result.update", "assessment_result", &v.ID, nil)
    c.JSON(http.StatusOK, v)
}

func (ctrl *AssessmentResultController) Delete(c *gin.Context) {
    id, e := uuid.Parse(c.Param("id")); if e != nil { httpx.Error(c, 400, "invalid_result_id", "invalid result id"); return }
    if e = ctrl.service.Delete(id); e != nil { assessmentResultError(c, e); return }
    actor, _ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(), c, &actor, "assessment_result.delete", "assessment_result", &id, nil)
    c.JSON(http.StatusOK, gin.H{"message":"assessment result deleted"})
}

func (ctrl *AssessmentResultController) ReportCard(c *gin.Context) {
    enrollmentID, e := uuid.Parse(c.Param("enrollmentId")); if e != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
    termID, e := uuid.Parse(c.Query("term_id")); if e != nil { httpx.Error(c, 400, "invalid_term_id", "term_id is required"); return }
    v, e := ctrl.service.ReportCard(enrollmentID, termID); if e != nil { assessmentResultError(c, e); return }
    c.JSON(http.StatusOK, v)
}

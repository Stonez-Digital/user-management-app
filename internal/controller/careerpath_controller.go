package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/service"
)

type CareerPathController struct { service *service.CareerPathService }
func NewCareerPathController(s *service.CareerPathService) *CareerPathController { return &CareerPathController{service:s} }
func careerPathError(c *gin.Context, err error) {
	switch {
	case errors.Is(err,service.ErrCareerPathForbidden): httpx.Error(c,403,"careerpath_forbidden","CareerPath is available to the authenticated student only")
	case errors.Is(err,service.ErrCareerPathNotFound): httpx.Error(c,404,"careerpath_not_found","CareerPath attempt not found")
	case errors.Is(err,service.ErrCareerPathSubmitted): httpx.Error(c,409,"careerpath_attempt_submitted","Submitted assessments cannot be changed")
	case errors.Is(err,service.ErrCareerPathInvalidResponse): httpx.Error(c,400,"careerpath_invalid_response","One or more assessment responses are invalid")
	case errors.Is(err,service.ErrCareerPathIncomplete): httpx.Error(c,400,"careerpath_incomplete","Answer each required question before submitting")
	case errors.Is(err,service.ErrCareerPathVersionMismatch): httpx.Error(c,409,"careerpath_version_mismatch","This draft uses an older assessment version and cannot be edited with the current questions")
	default: httpx.Error(c,500,"careerpath_operation_failed","CareerPath request could not be completed")
	}
}
func (ctrl *CareerPathController) context(c *gin.Context)(uuid.UUID,uuid.UUID,bool){uid,e:=uuid.Parse(c.GetString("user_id"));if e!=nil{httpx.Error(c,401,"invalid_user","invalid authenticated user");return uuid.Nil,uuid.Nil,false};sid,ok:=middleware.SchoolIDFromContext(c);if !ok{httpx.Error(c,403,"school_context_required","school context required");return uuid.Nil,uuid.Nil,false};return uid,sid,true}
func (ctrl *CareerPathController) Assessment(c *gin.Context){version:=c.Query("version");c.JSON(http.StatusOK,ctrl.service.AssessmentForVersion(version))}
func (ctrl *CareerPathController) CreateOrResume(c *gin.Context){uid,sid,ok:=ctrl.context(c);if !ok{return};v,e:=ctrl.service.CreateOrResume(sid,uid);if e!=nil{careerPathError(c,e);return};c.JSON(http.StatusOK,v)}
func (ctrl *CareerPathController) Get(c *gin.Context){uid,sid,ok:=ctrl.context(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_attempt_id","invalid assessment attempt id");return};v,e:=ctrl.service.Get(sid,uid,id);if e!=nil{careerPathError(c,e);return};c.JSON(http.StatusOK,v)}
func (ctrl *CareerPathController) SaveResponses(c *gin.Context){uid,sid,ok:=ctrl.context(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_attempt_id","invalid assessment attempt id");return};var body struct{Responses map[string][]string `json:"responses"`};dec:=json.NewDecoder(c.Request.Body);dec.DisallowUnknownFields();if e=dec.Decode(&body);e!=nil||body.Responses==nil{httpx.Error(c,400,"invalid_request","provide a valid responses object");return};v,e:=ctrl.service.SaveResponses(sid,uid,id,body.Responses);if e!=nil{careerPathError(c,e);return};c.JSON(http.StatusOK,v)}
func (ctrl *CareerPathController) Submit(c *gin.Context){uid,sid,ok:=ctrl.context(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_attempt_id","invalid assessment attempt id");return};v,e:=ctrl.service.Submit(sid,uid,id);if e!=nil{careerPathError(c,e);return};c.JSON(http.StatusOK,v)}

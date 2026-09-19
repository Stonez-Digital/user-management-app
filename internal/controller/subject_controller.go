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

type SubjectController struct{service *service.SubjectService}
func NewSubjectController(s *service.SubjectService)*SubjectController{return &SubjectController{s}}
type subjectRequest struct{Code string `json:"code" binding:"required,max=30"`;Name string `json:"name" binding:"required,max=100"`;Description string `json:"description" binding:"max=500"`;Active *bool `json:"active"`}
func subjectError(c *gin.Context,e error){switch{case errors.Is(e,service.ErrSubjectNotFound):httpx.Error(c,404,"subject_not_found","subject not found");case errors.Is(e,service.ErrSubjectDuplicate):httpx.Error(c,409,"subject_duplicate","subject code or name already exists");case errors.Is(e,service.ErrSubjectInvalid):httpx.Error(c,400,"invalid_subject","subject code and name are required");default:httpx.Error(c,500,"subject_operation_failed","subject operation failed")}}
func(ctrl *SubjectController)List(c *gin.Context){v,e:=ctrl.service.List();if e!=nil{subjectError(c,e);return};c.JSON(200,v)}
func(ctrl *SubjectController)Get(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_subject_id","invalid subject id");return};v,e:=ctrl.service.Get(id);if e!=nil{subjectError(c,e);return};c.JSON(200,v)}
func(ctrl *SubjectController)Create(c *gin.Context){var r subjectRequest;if e:=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};active:=true;if r.Active!=nil{active=*r.Active};v,e:=ctrl.service.Create(models.Subject{Code:r.Code,Name:r.Name,Description:r.Description,Active:active});if e!=nil{subjectError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"subject.create","subject",&v.ID,nil);c.JSON(http.StatusCreated,v)}
func(ctrl *SubjectController)Update(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_subject_id","invalid subject id");return};var r subjectRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};v,e:=ctrl.service.Get(id);if e!=nil{subjectError(c,e);return};v.Code,v.Name,v.Description=r.Code,r.Name,r.Description;if r.Active!=nil{v.Active=*r.Active};if e=ctrl.service.Update(v);e!=nil{subjectError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"subject.update","subject",&v.ID,nil);c.JSON(200,v)}
func(ctrl *SubjectController)Delete(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_subject_id","invalid subject id");return};if e=ctrl.service.Delete(id);e!=nil{subjectError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"subject.delete","subject",&id,nil);c.JSON(200,gin.H{"message":"subject deleted"})}
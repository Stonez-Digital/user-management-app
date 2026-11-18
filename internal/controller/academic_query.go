package controller

import (
 "github.com/gin-gonic/gin"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/httpx"
)

func parseAcademicQuery(c *gin.Context)(uuid.UUID,uuid.UUID,bool){
 sid,e:=uuid.Parse(c.Query("academic_session_id"));if e!=nil||sid==uuid.Nil{httpx.Error(c,400,"invalid_session_id","academic_session_id is required");return uuid.Nil,uuid.Nil,false}
 tid,e:=uuid.Parse(c.Query("term_id"));if e!=nil||tid==uuid.Nil{httpx.Error(c,400,"invalid_term_id","term_id is required");return uuid.Nil,uuid.Nil,false}
 return sid,tid,true
}

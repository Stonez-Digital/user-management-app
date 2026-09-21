package controller

import (
	"errors"
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/gorm"
)

type UserController struct { service *service.UserService }
type UpdateMeRequest struct { Name string `json:"name" binding:"omitempty,min=2,max=100"`; Email string `json:"email" binding:"omitempty,email,max=255"` }
type CreateUserRequest struct { Name string `json:"name" binding:"required,min=2,max=100"`; Email string `json:"email" binding:"required,email,max=255"` }

func NewUserController(s *service.UserService) *UserController { return &UserController{service:s} }

func schoolID(c *gin.Context) (uuid.UUID,bool) {
	id,ok:=middleware.SchoolIDFromContext(c)
	if !ok { httpx.Error(c,http.StatusForbidden,"school_context_required","school context required"); return uuid.Nil,false }
	return id,true
}

func (ctrl *UserController) CreateUser(c *gin.Context) {
	school,ok:=schoolID(c); if !ok{return}
	var req CreateUserRequest
	if err:=c.ShouldBindJSON(&req);err!=nil{httpx.Validation(c,httpx.ValidationErrors(err));return}
	user,err:=ctrl.service.CreateUser(school,req.Name,req.Email)
	if err!=nil{httpx.Error(c,500,"user_create_failed","failed to create user");return}
	actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"user.create","user",&user.ID,nil)
	c.JSON(http.StatusCreated,user)
}
func (ctrl *UserController) GetUsers(c *gin.Context) {
	school,ok:=schoolID(c);if !ok{return};users,err:=ctrl.service.GetUsers(school)
	if err!=nil{httpx.Error(c,500,"user_list_failed","failed to load users");return};c.JSON(200,users)
}
func (ctrl *UserController) GetUser(c *gin.Context) {
	school,ok:=schoolID(c);if !ok{return};id,err:=uuid.Parse(c.Param("id"))
	if err!=nil{httpx.Error(c,400,"invalid_user_id","invalid user id");return}
	user,err:=ctrl.service.GetUser(school,id);if err!=nil{httpx.Error(c,404,"user_not_found","user not found");return}
	actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"user.read","user",&user.ID,nil);c.JSON(200,user)
}
func (ctrl *UserController) DeleteUser(c *gin.Context) {
	school,ok:=schoolID(c);if !ok{return};id,err:=uuid.Parse(c.Param("id"));if err!=nil{httpx.Error(c,400,"invalid_user_id","invalid user id");return}
	if err:=ctrl.service.DeleteUser(school,id);err!=nil{if errors.Is(err,gorm.ErrRecordNotFound){httpx.Error(c,404,"user_not_found","user not found")}else{httpx.Error(c,500,"user_delete_failed","failed to delete user")};return}
	actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"user.delete","user",&id,nil);c.JSON(200,gin.H{"message":"deleted"})
}
func (ctrl *UserController) GetMe(c *gin.Context) {
    id,err:=uuid.Parse(c.GetString("user_id"));if err!=nil{httpx.Error(c,400,"invalid_user_id","invalid user id");return}
    var user models.User
    if err:=ctrl.service.DB().First(&user,"id = ?",id).Error;err!=nil{httpx.Error(c,404,"user_not_found","user not found");return}
    c.JSON(200,user)
}
func (ctrl *UserController) UpdateMe(c *gin.Context) {
    id,err:=uuid.Parse(c.GetString("user_id"));if err!=nil{httpx.Error(c,400,"invalid_user_id","invalid user id");return}
    var req UpdateMeRequest;if err:=c.ShouldBindJSON(&req);err!=nil{httpx.Validation(c,httpx.ValidationErrors(err));return}
    if c.GetString(middleware.RoleKey)==authz.RoleSuperAdmin {
        var user models.User
        if err:=ctrl.service.DB().First(&user,"id = ?",id).Error;err!=nil{httpx.Error(c,404,"user_not_found","user not found");return}
        updates:=map[string]interface{}{}
        if req.Name!=""{updates["name"]=req.Name};if req.Email!=""{updates["email"]=req.Email}
        if len(updates)>0{if err:=ctrl.service.DB().Model(&user).Updates(updates).Error;err!=nil{httpx.Error(c,500,"user_update_failed","failed to update user");return};if err:=ctrl.service.DB().First(&user,id).Error;err!=nil{httpx.Error(c,500,"user_read_failed","failed to load updated profile");return}}
        _=audit.Record(ctrl.service.DB(),c,&id,"user.update","user",&id,nil);c.JSON(200,user);return
    }
    school,ok:=schoolID(c);if !ok{return}
    user,err:=ctrl.service.GetUser(school,id);if err!=nil{httpx.Error(c,404,"user_not_found","user not found");return}
    if req.Name!=""{user.Name=req.Name};if req.Email!=""{user.Email=req.Email}
    if err:=ctrl.service.UpdateUser(school,user);err!=nil{httpx.Error(c,500,"user_update_failed","failed to update user");return}
    _=audit.Record(ctrl.service.DB(),c,&id,"user.update","user",&id,nil);c.JSON(200,user)
}

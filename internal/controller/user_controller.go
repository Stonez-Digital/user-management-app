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
type CreateUserRequest struct { Name string `json:"name" binding:"required,min=2,max=100"`; Email string `json:"email" binding:"required,email,max=255"`; Password string `json:"password" binding:"required,min=8,max=128"`; Role string `json:"role" binding:"required"` }

func NewUserController(s *service.UserService) *UserController { return &UserController{service:s} }

func (ctrl *UserController) requestService(c *gin.Context) (*service.UserService, bool) {
    tx, ok := middleware.TenantDBFromContext(c)
    if !ok || tx == nil { httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation"); return nil, false }
    return ctrl.service.WithDB(tx), true
}

func schoolID(c *gin.Context) (uuid.UUID,bool) {
    id,ok:=middleware.SchoolIDFromContext(c)
    if !ok { httpx.Error(c,http.StatusForbidden,"school_context_required","school context required"); return uuid.Nil,false }
    return id,true
}

func (ctrl *UserController) CreateUser(c *gin.Context) {
    school,ok:=schoolID(c); if !ok{return}
    svc,ready:=ctrl.requestService(c);if !ready{return}
    var req CreateUserRequest
    if err:=c.ShouldBindJSON(&req);err!=nil{httpx.Validation(c,httpx.ValidationErrors(err));return}
    user,err:=svc.CreateUser(school,req.Name,req.Email,req.Password,req.Role)
    if err!=nil{if errors.Is(err,service.ErrInvalidSchoolUserRole){httpx.Error(c,403,"invalid_school_user_role","school administrators cannot create platform or school administrator accounts");return};if errors.Is(err,gorm.ErrDuplicatedKey){httpx.Error(c,409,"user_email_exists","email already exists");return};httpx.Error(c,500,"user_create_failed","failed to create user");return}
    actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(svc.DB(),c,&actor,"user.create","user",&user.ID,nil)
    c.JSON(http.StatusCreated,user)
}
func (ctrl *UserController) GetUsers(c *gin.Context) {
    school,ok:=schoolID(c);if !ok{return};svc,ready:=ctrl.requestService(c);if !ready{return};users,err:=svc.GetUsers(school)
    if err!=nil{httpx.Error(c,500,"user_list_failed","failed to load users");return};c.JSON(200,users)
}
func (ctrl *UserController) GetUser(c *gin.Context) {
    school,ok:=schoolID(c);if !ok{return};svc,ready:=ctrl.requestService(c);if !ready{return};id,err:=uuid.Parse(c.Param("id"))
    if err!=nil{httpx.Error(c,400,"invalid_user_id","invalid user id");return}
    user,err:=svc.GetUser(school,id);if err!=nil{httpx.Error(c,404,"user_not_found","user not found");return}
    actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(svc.DB(),c,&actor,"user.read","user",&user.ID,nil);c.JSON(200,user)
}
func (ctrl *UserController) DeleteUser(c *gin.Context) {
    school,ok:=schoolID(c);if !ok{return};svc,ready:=ctrl.requestService(c);if !ready{return};id,err:=uuid.Parse(c.Param("id"));if err!=nil{httpx.Error(c,400,"invalid_user_id","invalid user id");return}
    if err:=svc.DeleteUser(school,id);err!=nil{if errors.Is(err,gorm.ErrRecordNotFound){httpx.Error(c,404,"user_not_found","user not found")}else{httpx.Error(c,500,"user_delete_failed","failed to delete user")};return}
    actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(svc.DB(),c,&actor,"user.delete","user",&id,nil);c.JSON(200,gin.H{"message":"deleted"})
}
func (ctrl *UserController) GetMe(c *gin.Context) {
	tx, ok := middleware.IdentityDBFromContext(c)
	if !ok || tx == nil {
		httpx.Error(c, http.StatusInternalServerError, "identity_transaction_required", "identity transaction is required")
		return
	}
	id, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "invalid_user_id", "authenticated user context is invalid")
		return
	}
	var user models.User
	if err := tx.Where("id = ? AND active = true", id).First(&user).Error; err != nil {
		httpx.Error(c, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	schoolName, schoolLogoURL, schoolSlug := "", "", ""
	if user.SchoolID != nil && *user.SchoolID != uuid.Nil {
		var school models.School
		if err := tx.Select("name, slug, logo_url").First(&school, "id = ?", *user.SchoolID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(c, http.StatusInternalServerError, "school_profile_read_failed", "failed to load school profile")
			return
		} else if err == nil {
			schoolName, schoolSlug, schoolLogoURL = school.Name, school.Slug, school.LogoURL
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"id": user.ID, "name": user.Name, "email": user.Email, "role": user.Role,
		"active": user.Active, "school_id": user.SchoolID, "school_name": schoolName,
		"school_slug": schoolSlug, "school_logo_url": schoolLogoURL,
	})
}

func (ctrl *UserController) UpdateMe(c *gin.Context) {
	tx, ok := middleware.IdentityDBFromContext(c)
	if !ok || tx == nil {
		httpx.Error(c, http.StatusInternalServerError, "identity_transaction_required", "identity transaction is required")
		return
	}
	id, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "invalid_user_id", "authenticated user context is invalid")
		return
	}
	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err))
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" { updates["name"] = req.Name }
	if req.Email != "" { updates["email"] = req.Email }

	var user models.User
	if err := tx.Where("id = ? AND active = true", id).First(&user).Error; err != nil {
		httpx.Error(c, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	if len(updates) > 0 {
		if err := tx.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "user_update_failed", "failed to update user")
			return
		}
		if err := tx.First(&user, "id = ?", id).Error; err != nil {
			httpx.Error(c, http.StatusInternalServerError, "user_read_failed", "failed to load updated profile")
			return
		}
	}
	if err := audit.Record(tx, c, &id, "user.update", "user", &id, nil); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "user_audit_failed", "failed to record profile update")
		return
	}
	c.JSON(http.StatusOK, user)
}

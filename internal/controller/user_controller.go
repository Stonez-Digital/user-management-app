package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/service"
	"github.com/onoja217/users-management-app/internal/audit"
)

type UserController struct {
	service *service.UserService
}
type UpdateMeRequest struct {
	Name  string `json:"name" binding:"omitempty,min=2,max=100"`
	Email string `json:"email" binding:"omitempty,email,max=255"`
}

func NewUserController(s *service.UserService) *UserController {
	return &UserController{service: s}
}

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required,min=2,max=100"`
	Email string `json:"email" binding:"required,email,max=255"`
}

// CREATE
func (ctrl *UserController) CreateUser(c *gin.Context) {
	var req CreateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err)); return
	}

	user, err := ctrl.service.CreateUser(req.Name, req.Email)
	if err != nil { httpx.Error(c, http.StatusInternalServerError, "user_create_failed", "failed to create user"); return }
	actorID, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actorID, "user.create", "user", &user.ID, nil)
	c.JSON(http.StatusCreated, user)
}

// GET ALL
func (ctrl *UserController) GetUsers(c *gin.Context) {
	users, err := ctrl.service.GetUsers()
	if err != nil { httpx.Error(c, http.StatusInternalServerError, "user_list_failed", "failed to load users"); return }
	c.JSON(http.StatusOK, users)
}

// GET ONE
func (ctrl *UserController) GetUser(c *gin.Context) {
	id := c.Param("id")

	uid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	user, err := ctrl.service.GetUser(uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	actorID, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actorID, "user.read", "user", &user.ID, nil)
	c.JSON(http.StatusOK, user)
}

// DELETE
func (ctrl *UserController) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	uid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	if err := ctrl.service.DeleteUser(uid); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	actorID, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actorID, "user.delete", "user", &uid, nil)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ME
func (ctrl *UserController) GetMe(c *gin.Context) {
	userID := c.GetString("user_id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := ctrl.service.GetUser(uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}
func (ctrl *UserController) UpdateMe(c *gin.Context) {

	userID := c.GetString("user_id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid user id",
		})
		return
	}

	var req UpdateMeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err)); return
	}

	user, err := ctrl.service.GetUser(uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "user not found",
		})
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}

	if req.Email != "" {
		user.Email = req.Email
	}

	if err := ctrl.service.UpdateUser(user); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "user_update_failed", "failed to update user"); return
	}

	_ = audit.Record(ctrl.service.DB(), c, &uid, "user.update", "user", &uid, nil)
	c.JSON(http.StatusOK, user)
}
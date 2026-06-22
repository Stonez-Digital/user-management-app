package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/service"
)

type UserController struct {
	service *service.UserService
}
type UpdateMeRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func NewUserController(s *service.UserService) *UserController {
	return &UserController{service: s}
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CREATE
func (ctrl *UserController) CreateUser(c *gin.Context) {
	var req CreateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := ctrl.service.CreateUser(req.Name, req.Email)
	c.JSON(http.StatusCreated, user)
}

// GET ALL
func (ctrl *UserController) GetUsers(c *gin.Context) {
	users := ctrl.service.GetUsers()
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update user",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}
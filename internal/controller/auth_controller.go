package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type AuthController struct {
	DB *gorm.DB
}

func (ac *AuthController) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User

	if err := ac.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "if the account exists, a reset token has been generated",
		})
		return
	}

	token := uuid.New().String()

	reset := models.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
		Used:      false,
	}

	if err := ac.DB.Create(&reset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create reset token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "reset token generated",
		"reset_token": token,
	})
}
func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var reset models.PasswordResetToken

	if err := ac.DB.
		Where("token = ? AND used = false", req.Token).
		First(&reset).Error; err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid reset token",
		})
		return
	}

	if time.Now().After(reset.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "reset token expired",
		})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to hash password",
		})
		return
	}

	if err := ac.DB.Model(&models.User{}).
		Where("id = ?", reset.UserID).
		Update("password_hash", hash).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update password",
		})
		return
	}

	reset.Used = true

	if err := ac.DB.Save(&reset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to invalidate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "password reset successful",
	})
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{DB: db}
}

// ---------------- REQUEST TYPES ----------------

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ---------------- REGISTER ----------------

func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         "user",
	}

	if err := ac.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user created"})
}

// ---------------- LOGIN ----------------

func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User

	if err := ac.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// access token
	accessToken, err := auth.GenerateToken(user.ID.String(), user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	// refresh token
	refreshToken, err := auth.GenerateRefreshToken(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	refresh := models.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}

	if err := ac.DB.Create(&refresh).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save refresh token"})
		return
	}
	session := models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		Revoked:      false,
		UserAgent:    c.Request.UserAgent(),
		IP:           c.ClientIP(),
	}

	if err := ac.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// ---------------- REFRESH (ROTATION) ----------------

func (ac *AuthController) Refresh(c *gin.Context) {
	var req RefreshRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var stored models.RefreshToken

	// 1. FIND VALID TOKEN ONLY
	if err := ac.DB.
		Where("token = ? AND revoked = false", req.RefreshToken).
		First(&stored).Error; err != nil {

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid or already used refresh token",
		})
		return
	}

	// 2. CHECK EXPIRY
	if time.Now().After(stored.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "refresh token expired",
		})
		return
	}

	// 3. GET USER
	var user models.User
	if err := ac.DB.First(&user, "id = ?", stored.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not found",
		})
		return
	}

	// 4. REVOKE OLD TOKEN FIRST (IMPORTANT FIX)
	if err := ac.DB.
		Model(&models.RefreshToken{}).
		Where("id = ?", stored.ID).
		Update("revoked", true).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to revoke old refresh token",
		})
		return
	}

	// 5. GENERATE NEW TOKENS
	accessToken, err := auth.GenerateToken(user.ID.String(), user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to generate access token",
		})
		return
	}

	newRefreshToken, err := auth.GenerateRefreshToken(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to generate refresh token",
		})
		return
	}

	// 6. STORE NEW REFRESH TOKEN
	newStored := models.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     newRefreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}

	if err := ac.DB.Create(&newStored).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to store refresh token",
		})
		return
	}

	// 7. RETURN RESPONSE
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}
func (ac *AuthController) GetSessions(c *gin.Context) {
	userID := c.GetString("user_id")

	var sessions []models.Session

	if err := ac.DB.
		Where("user_id = ? AND revoked = false", userID).
		Find(&sessions).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load sessions",
		})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

func (ac *AuthController) RevokeSession(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.Param("id")

	var session models.Session

	if err := ac.DB.
		Where("id = ? AND user_id = ?", sessionID, userID).
		First(&session).Error; err != nil {

		c.JSON(http.StatusNotFound, gin.H{
			"error": "session not found",
		})
		return
	}

	// revoke session
	session.Revoked = true
	ac.DB.Save(&session)

	// revoke matching refresh token
	ac.DB.Model(&models.RefreshToken{}).
		Where("token = ?", session.RefreshToken).
		Update("revoked", true)

	c.JSON(http.StatusOK, gin.H{
		"message": "session revoked",
	})
}

// ---------------- LOGOUT ----------------

func (ac *AuthController) Logout(c *gin.Context) {
	var req LogoutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var token models.RefreshToken

	err := ac.DB.Where("token = ? AND revoked = false", req.RefreshToken).
		First(&token).Error

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	token.Revoked = true

	if err := ac.DB.Save(&token).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
func (ac *AuthController) LogoutAll(c *gin.Context) {
	userID := c.GetString("user_id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// revoke ALL refresh tokens for this user
	if err := ac.DB.
		Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = false", userID).
		Update("revoked", true).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to logout all devices",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "logged out from all devices",
	})
}

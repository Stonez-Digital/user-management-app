package controller

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

const minPasswordLength = 8

var errResetExpired = errors.New("reset token expired")
var errRefreshReused = errors.New("refresh token already rotated")

type ForgotPasswordRequest struct { Email string `json:"email"` }
type ResetPasswordRequest struct { Token string `json:"token"`; NewPassword string `json:"new_password"` }
type ChangePasswordRequest struct { CurrentPassword string `json:"current_password"`; NewPassword string `json:"new_password"` }
type RefreshRequest struct { RefreshToken string `json:"refresh_token"` }
type LogoutRequest struct { RefreshToken string `json:"refresh_token"` }
type RegisterRequest struct { Name string `json:"name"`; Email string `json:"email"`; Password string `json:"password"` }
type LoginRequest struct { Email string `json:"email"`; Password string `json:"password"` }
type AssignRoleRequest struct { Role string `json:"role"` }


type AuthController struct { DB *gorm.DB }

func NewAuthController(db *gorm.DB) *AuthController { return &AuthController{DB: db} }

func validPassword(password string) bool { return len(password) >= minPasswordLength }

func newResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", err }
	return hex.EncodeToString(b), nil
}

func (ac *AuthController) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var user models.User
	if err := ac.DB.Where("email = ? AND active = true", strings.TrimSpace(req.Email)).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the account exists, a reset link will be sent"})
		return
	}

	token, err := newResetToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reset token"})
		return
	}
	reset := models.PasswordResetToken{ID: uuid.New(), UserID: user.ID, Token: token, ExpiresAt: time.Now().Add(15 * time.Minute)}
	if err := ac.DB.Create(&reset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reset token"})
		return
	}

	response := gin.H{"message": "if the account exists, a reset link will be sent"}
	if strings.EqualFold(os.Getenv("APP_ENV"), "development") {
		response["reset_token"] = token
	}
	c.JSON(http.StatusOK, response)
}

func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPassword(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password reset request"})
		return
	}

	err := ac.DB.Transaction(func(tx *gorm.DB) error {
		var reset models.PasswordResetToken
		if err := tx.Where("token = ? AND used = false", req.Token).First(&reset).Error; err != nil {
			return gorm.ErrRecordNotFound
		}
		if time.Now().After(reset.ExpiresAt) { return errResetExpired }

		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil { return err }
		if err := tx.Model(&models.User{}).Where("id = ? AND active = true", reset.UserID).Update("password_hash", hash).Error; err != nil {
			return err
		}
		if err := tx.Model(&reset).Update("used", true).Error; err != nil { return err }
		if err := tx.Model(&models.RefreshToken{}).Where("user_id = ?", reset.UserID).Update("revoked", true).Error; err != nil { return err }
		return tx.Model(&models.Session{}).Where("user_id = ?", reset.UserID).Update("revoked", true).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, errResetExpired) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired reset token"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password reset successful"})
}

func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Email) == "" || !validPassword(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, email and a password of at least 8 characters are required"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	user := models.User{Name: strings.TrimSpace(req.Name), Email: strings.ToLower(strings.TrimSpace(req.Email)), PasswordHash: hash, Role: authz.RoleStudent, Active: true}
	if err := ac.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to create account"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "user created"})
}

func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var user models.User
	if err := ac.DB.Where("email = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil || !user.Active || !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	accessToken, err := auth.GenerateToken(user.ID.String(), user.Role)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"}); return }
	refreshToken, err := auth.GenerateRefreshToken(user.ID.String())
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"}); return }

	err = ac.DB.Transaction(func(tx *gorm.DB) error {
		expiry := time.Now().Add(auth.RefreshTokenTTL)
		if err := tx.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, Token: refreshToken, ExpiresAt: expiry}).Error; err != nil { return err }
		return tx.Create(&models.Session{ID: uuid.New(), UserID: user.ID, RefreshToken: refreshToken, ExpiresAt: expiry, UserAgent: c.Request.UserAgent(), IP: c.ClientIP()}).Error
	})
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"}); return }
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken, "refresh_token": refreshToken})
}

func (ac *AuthController) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid refresh request"})
		return
	}

	var stored models.RefreshToken
	if err := ac.DB.Where("token = ? AND revoked = false", req.RefreshToken).First(&stored).Error; err != nil || time.Now().After(stored.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}
	var user models.User
	if err := ac.DB.First(&user, "id = ?", stored.UserID).Error; err != nil || !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	accessToken, err := auth.GenerateToken(user.ID.String(), user.Role)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"}); return }
	newRefreshToken, err := auth.GenerateRefreshToken(user.ID.String())
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"}); return }

	err = ac.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.RefreshToken{}).Where("id = ? AND revoked = false", stored.ID).Update("revoked", true)
		if result.Error != nil { return result.Error }
		if result.RowsAffected != 1 { return errRefreshReused }
		expiry := time.Now().Add(auth.RefreshTokenTTL)
		if err := tx.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, Token: newRefreshToken, ExpiresAt: expiry}).Error; err != nil { return err }
		if err := tx.Model(&models.Session{}).Where("refresh_token = ? AND revoked = false", req.RefreshToken).Updates(map[string]interface{}{"refresh_token": newRefreshToken, "expires_at": expiry}).Error; err != nil { return err }
		return nil
	})
	if errors.Is(err, errRefreshReused) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or already used refresh token"})
		return
	}
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate refresh token"}); return }
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken, "refresh_token": newRefreshToken})
}

func (ac *AuthController) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPassword(req.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must be at least 8 characters"})
		return
	}
	var user models.User
	if err := ac.DB.First(&user, "id = ? AND active = true", userID).Error; err != nil || !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"}); return }
	err = ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("password_hash", hash).Error; err != nil { return err }
		if err := tx.Model(&models.RefreshToken{}).Where("user_id = ?", user.ID).Update("revoked", true).Error; err != nil { return err }
		return tx.Model(&models.Session{}).Where("user_id = ?", user.ID).Update("revoked", true).Error
	})
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"}); return }
	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully; please sign in again"})
}

func (ac *AuthController) GetSessions(c *gin.Context) {
	var sessions []models.Session
	if err := ac.DB.Where("user_id = ? AND revoked = false", c.GetString("user_id")).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load sessions"})
		return
	}
	c.JSON(http.StatusOK, sessions)
}

func (ac *AuthController) RevokeSession(c *gin.Context) {
	userID := c.GetString("user_id")
	var session models.Session
	if err := ac.DB.Where("id = ? AND user_id = ? AND revoked = false", c.Param("id"), userID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	if err := ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&session).Update("revoked", true).Error; err != nil { return err }
		return tx.Model(&models.RefreshToken{}).Where("token = ? AND user_id = ?", session.RefreshToken, userID).Update("revoked", true).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "session revoked"})
}

func (ac *AuthController) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var token models.RefreshToken
	if err := ac.DB.Where("token = ? AND revoked = false", req.RefreshToken).First(&token).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	if err := ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&token).Update("revoked", true).Error; err != nil { return err }
		return tx.Model(&models.Session{}).Where("refresh_token = ?", req.RefreshToken).Update("revoked", true).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

func (ac *AuthController) LogoutAll(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.RefreshToken{}).Where("user_id = ? AND revoked = false", userID).Update("revoked", true).Error; err != nil { return err }
		return tx.Model(&models.Session{}).Where("user_id = ? AND revoked = false", userID).Update("revoked", true).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout all devices"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out from all devices"})
}

func (ac *AuthController) setUserActive(c *gin.Context, active bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var user models.User
	if err := ac.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err := ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("active", active).Error; err != nil { return err }
		if !active {
			if err := tx.Model(&models.RefreshToken{}).Where("user_id = ?", user.ID).Update("revoked", true).Error; err != nil { return err }
			return tx.Model(&models.Session{}).Where("user_id = ?", user.ID).Update("revoked", true).Error
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update account status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": map[bool]string{true: "account activated", false: "account deactivated"}[active]})
}

func (ac *AuthController) ActivateUser(c *gin.Context) { ac.setUserActive(c, true) }
func (ac *AuthController) DeactivateUser(c *gin.Context) { ac.setUserActive(c, false) }

func (ac *AuthController) AssignRole(c *gin.Context) {
    targetID, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }

    var req AssignRoleRequest
    if err := c.ShouldBindJSON(&req); err != nil || !authz.IsValidRole(req.Role) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
        return
    }

    actorID, err := uuid.Parse(c.GetString("user_id"))
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    var target models.User
    if err := ac.DB.First(&target, "id = ?", targetID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    if target.ID == actorID {
        c.JSON(http.StatusBadRequest, gin.H{"error": "users cannot change their own role"})
        return
    }
    if target.Role == req.Role {
        c.JSON(http.StatusOK, gin.H{"message": "role unchanged", "role": target.Role})
        return
    }

    previousRole := target.Role
    err = ac.DB.Transaction(func(tx *gorm.DB) error {
        if err := tx.Model(&target).Update("role", req.Role).Error; err != nil {
            return err
        }
        audit := models.RoleChangeAudit{
            ID: uuid.New(),
            ActorID: actorID,
            TargetID: target.ID,
            FromRole: previousRole,
            ToRole: req.Role,
            IP: c.ClientIP(),
        }
        return tx.Create(&audit).Error
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change role"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "role updated",
        "user_id": target.ID,
        "role": req.Role,
    })
}

func (ac *AuthController) ListRoles(c *gin.Context) {
    roles := []gin.H{}
    for _, role := range []string{
        authz.RoleSuperAdmin, authz.RoleSchoolAdmin, authz.RoleTeacher,
        authz.RoleStudent, authz.RoleParent, authz.RoleAccountant, authz.RoleStaff,
    } {
        roles = append(roles, gin.H{
            "role": role,
            "permissions": authz.PermissionsForRole(role),
        })
    }
    c.JSON(http.StatusOK, gin.H{"roles": roles})
}

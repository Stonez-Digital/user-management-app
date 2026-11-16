package controller

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"log"
	"mime/multipart"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/audit"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/gorm"
)

const minPasswordLength = 8

var errResetExpired = errors.New("reset token expired")
var errRefreshReused = errors.New("refresh token already rotated")
var errSessionRefreshTokenMissing = errors.New("session refresh token record missing")

type ForgotPasswordRequest struct { Email string `json:"email" binding:"required,email,max=255"` }
type ResetPasswordRequest struct { Token string `json:"token" binding:"required"`; NewPassword string `json:"new_password" binding:"required,min=8,max=128"` }
type ChangePasswordRequest struct { CurrentPassword string `json:"current_password" binding:"required"`; NewPassword string `json:"new_password" binding:"required,min=8,max=128"` }
type RefreshRequest struct { RefreshToken string `json:"refresh_token" binding:"required"` }
type LogoutRequest struct { RefreshToken string `json:"refresh_token" binding:"required"` }
type RegisterRequest struct { Name string `json:"name" binding:"required,min=2,max=100"`; Email string `json:"email" binding:"required,email,max=255"`; Password string `json:"password" binding:"required,min=8,max=128"` }
type SchoolSignupRequest struct {
 SchoolName string `json:"school_name" form:"school_name" binding:"required,min=2,max=160"`
 SchoolSlug string `json:"school_slug" form:"school_slug" binding:"required,min=3,max=63"`
 SchoolEmail string `json:"school_email" form:"school_email" binding:"required,email,max=255"`
 SchoolType string `json:"school_type" form:"school_type" binding:"required,oneof=primary secondary both"`
 RequestedPlan string `json:"requested_plan" form:"requested_plan" binding:"required,oneof=trial basic professional premium enterprise"`
 Address string `json:"address" form:"address" binding:"omitempty,max=300"`
 City string `json:"city" form:"city" binding:"omitempty,max=100"`
 State string `json:"state" form:"state" binding:"omitempty,max=100"`
 ContactPhone string `json:"phone" form:"phone" binding:"omitempty,max=80"`
 AdminName string `json:"admin_name" form:"admin_name" binding:"required,min=2,max=100"`
 AdminEmail string `json:"admin_email" form:"admin_email" binding:"required,email,max=255"`
 AdminPassword string `json:"admin_password" form:"admin_password" binding:"required,min=8,max=128"`
}
type LoginRequest struct { Email string `json:"email" binding:"required,email,max=255"`; Password string `json:"password" binding:"required"`; SchoolCode string `json:"school_code" binding:"omitempty,max=50"`; SchoolSlug string `json:"school_slug" binding:"omitempty,max=180"` }
type AssignRoleRequest struct { Role string `json:"role" binding:"required"` }


type PasswordResetMailer interface {
	SendPasswordReset(name, email, token string) error
}

func (ac *AuthController) requestTenantDB(c *gin.Context) (*gorm.DB, bool) {
    tx, ok := middleware.TenantDBFromContext(c)
    if !ok {
        httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation")
        return nil, false
    }
    return tx, true
}

func (ac *AuthController) requestIdentityDB(c *gin.Context) (*gorm.DB, bool) {
	tx, ok := middleware.IdentityDBFromContext(c)
	if !ok || tx == nil {
		httpx.Error(c, http.StatusInternalServerError, "identity_transaction_required", "identity transaction is required for this operation")
		return nil, false
	}
	return tx, true
}

type AuthController struct {
	DB *gorm.DB
	Mailer PasswordResetMailer
	LogoStorage *service.SchoolLogoStorage
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{DB: db, Mailer: service.NewCredentialMailerFromEnv(), LogoStorage: service.NewSchoolLogoStorageFromEnv()}
}

func validPassword(password string) bool { return len(password) >= minPasswordLength }

// validateSchoolAccess fails closed for every non-platform account. A missing
// or dangling school association must never bypass school approval checks.
func validateSchoolAccess(db *gorm.DB, user models.User, schoolCode, schoolSlug string) error {
	if user.Role == authz.RoleSuperAdmin {
		if strings.TrimSpace(schoolSlug) != "" {
			return errors.New("platform administrators must use the platform login")
		}
		return nil
	}
	if user.SchoolID == nil || *user.SchoolID == uuid.Nil {
		return errors.New("school association required")
	}
	var school models.School
	if err := db.First(&school, "id = ?", *user.SchoolID).Error; err != nil {
		return errors.New("school association is invalid")
	}
	if school.Status != models.SchoolStatusActive {
		return errors.New("school onboarding is pending platform approval")
	}
	if code := strings.TrimSpace(schoolCode); code != "" && !strings.EqualFold(code, school.Code) {
		return errors.New("school code does not match your account")
	}
	if slug := strings.TrimSpace(schoolSlug); slug != "" && !strings.EqualFold(slug, school.Slug) {
		return errors.New("school login does not match your account")
	}
	return nil
}

func newResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", err }
	return hex.EncodeToString(b), nil
}

func (ac *AuthController) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err)); return
	}

	var user models.User
	if err := ac.DB.Where("email = ? AND active = true", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "if the account exists, a reset link will be sent"})
		return
	}

	token, err := newResetToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reset token"})
		return
	}
	reset := models.PasswordResetToken{ID: uuid.New(), UserID: user.ID, TokenHash: auth.HashResetToken(token), ExpiresAt: time.Now().Add(15 * time.Minute)}
	if err := ac.DB.Create(&reset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reset token"})
		return
	}

	// Keep the response identical for existing and unknown accounts. Never return
	// the raw token from the API; it is delivered only through the configured mailer.
	if ac.Mailer == nil {
		_ = ac.DB.Model(&reset).Update("used", true).Error
		log.Printf("password reset email unavailable: mailer is not configured")
	} else if err := ac.Mailer.SendPasswordReset(user.Name, user.Email, token); err != nil {
		_ = ac.DB.Model(&reset).Update("used", true).Error
		log.Printf("password reset email delivery failed: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "if the account exists, a reset link will be sent"})
}

func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err)); return
	}

	targetUserID := uuid.Nil
	err := ac.DB.Transaction(func(tx *gorm.DB) error {
		var reset models.PasswordResetToken
		if err := tx.Where("token_hash = ? AND used = false", auth.HashResetToken(req.Token)).First(&reset).Error; err != nil {
			return gorm.ErrRecordNotFound
		}
		if time.Now().After(reset.ExpiresAt) { return errResetExpired }

		// Claim the token atomically. A concurrent request may have passed the
		// initial read, but only one transaction can change used=false to true.
		claim := tx.Model(&models.PasswordResetToken{}).
			Where("id = ? AND used = false AND expires_at > ?", reset.ID, time.Now()).
			Update("used", true)
		if claim.Error != nil { return claim.Error }
		if claim.RowsAffected != 1 { return errResetExpired }

		targetUserID = reset.UserID
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil { return err }
		if err := tx.Model(&models.User{}).Where("id = ? AND active = true", reset.UserID).Update("password_hash", hash).Error; err != nil {
			return err
		}
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
	_ = audit.Record(ac.DB, c, nil, "security.password_reset", "user", &targetUserID, nil)
	c.JSON(http.StatusOK, gin.H{"message": "password reset successful"})
}

func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err)); return
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
	_ = audit.Record(ac.DB, c, &user.ID, "user.create", "user", &user.ID, map[string]interface{}{"email": user.Email, "role": user.Role})
	c.JSON(http.StatusCreated, gin.H{"message": "user created"})
}

func (ac *AuthController) SchoolSignup(c *gin.Context) {
 var req SchoolSignupRequest
 c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6*1024*1024)
 if err := c.ShouldBind(&req); err != nil {
  httpx.Validation(c, httpx.ValidationErrors(err))
  return
 }

 var logoHeader *multipart.FileHeader
 if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
  var logoErr error
  logoHeader, logoErr = c.FormFile("logo")
  if logoErr != nil && !errors.Is(logoErr, http.ErrMissingFile) {
   httpx.Error(c, http.StatusBadRequest, "school_logo_invalid", "unable to read school logo")
   return
  }
 }
 if logoHeader != nil && ac.LogoStorage == nil {
  httpx.Error(c, http.StatusServiceUnavailable, "school_logo_storage_unavailable", "school logo storage is not configured")
  return
 }

 name := strings.TrimSpace(req.SchoolName)
 slug := strings.ToLower(strings.TrimSpace(req.SchoolSlug))
 code := strings.ToUpper(strings.ReplaceAll(slug, "-", ""))
 if len(code) > 40 { code = code[:40] }
 code += "-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
 email := strings.ToLower(strings.TrimSpace(req.AdminEmail))
 schoolEmail := strings.ToLower(strings.TrimSpace(req.SchoolEmail))
 if err := validateRequestedSchoolSlug(slug); err != nil {
  httpx.Error(c, http.StatusBadRequest, "school_subdomain_invalid", err.Error())
  return
 }
 hash, err := auth.HashPassword(req.AdminPassword)
 if err != nil {
  httpx.Error(c, http.StatusInternalServerError, "password_hash_failed", "failed to secure administrator password")
  return
 }

 school := models.School{
  ID: uuid.New(), Name: name, Code: code, Slug: slug,
  SchoolType: req.SchoolType, RequestedPlan: req.RequestedPlan, Address: strings.TrimSpace(req.Address),
  City: strings.TrimSpace(req.City), State: strings.TrimSpace(req.State),
  ContactPhone: strings.TrimSpace(req.ContactPhone), ContactEmail: schoolEmail,
  Status: models.SchoolStatusPending,
 }
 admin := models.User{ID: uuid.New(), Name: strings.TrimSpace(req.AdminName), Email: email, PasswordHash: hash, Role: authz.RoleSchoolAdmin, Active: true}
 var logoUploadErr error

 err = ac.DB.Transaction(func(tx *gorm.DB) error {
  var existing models.School
  if e := tx.Where("lower(code) = ?", strings.ToLower(code)).First(&existing).Error; e == nil {
   return gorm.ErrDuplicatedKey
  } else if !errors.Is(e, gorm.ErrRecordNotFound) {
   return e
  }
  var existingUser models.User
  if e := tx.Where("lower(email) = ?", email).First(&existingUser).Error; e == nil {
   return gorm.ErrDuplicatedKey
  } else if !errors.Is(e, gorm.ErrRecordNotFound) {
   return e
  }
  available, e := requestedSchoolSlugAvailable(tx, slug)
  if e != nil { return e }
  if !available { return gorm.ErrDuplicatedKey }
  if e := tx.Create(&school).Error; e != nil { return e }

  admin.SchoolID = &school.ID
  if e := tx.Create(&admin).Error; e != nil { return e }

  if logoHeader != nil {
   logoFile, e := logoHeader.Open()
   if e != nil { logoUploadErr = e; return e }
   defer logoFile.Close()
   contentType := logoHeader.Header.Get("Content-Type")
   logoURL, e := ac.LogoStorage.Upload(school.ID, logoHeader.Filename, contentType, logoFile, logoHeader.Size)
   if e != nil {
    logoUploadErr = e
    return e
   }
   if e := tx.Model(&models.School{}).Where("id = ?", school.ID).Update("logo_url", logoURL).Error; e != nil {
    logoUploadErr = e
    return e
   }
   school.LogoURL = logoURL
  }
  return nil
 })

 if err != nil {
  if errors.Is(err, gorm.ErrDuplicatedKey) {
   httpx.Error(c, http.StatusConflict, "school_or_email_exists", "school code, subdomain, or administrator email already exists")
  } else if logoUploadErr != nil {
   httpx.Error(c, http.StatusBadRequest, "school_logo_upload_failed", "school logo could not be uploaded; check that it is a PNG, JPEG, or WebP image no larger than 5 MB")
  } else {
   httpx.Error(c, http.StatusInternalServerError, "school_signup_failed", "failed to submit school onboarding")
  }
  return
 }

 _ = audit.Record(ac.DB, c, nil, "school.signup", "school", &school.ID, map[string]interface{}{"code": school.Code, "slug": school.Slug, "requested_plan": school.RequestedPlan, "admin_user_id": admin.ID, "logo_uploaded": school.LogoURL != ""})
 domain := school.Slug + ".stonezdigital.com.ng"
 c.JSON(http.StatusCreated, gin.H{
  "message": "school onboarding submitted",
  "status": school.Status,
  "school": gin.H{"id": school.ID, "name": school.Name, "code": school.Code, "slug": school.Slug, "domain": domain, "domain_status": "pending_approval", "logo_url": school.LogoURL, "school_type": school.SchoolType, "address": school.Address, "city": school.City, "state": school.State, "phone": school.ContactPhone, "email": school.ContactEmail, "requested_plan": school.RequestedPlan},
  "administrator": gin.H{"name": admin.Name, "email": admin.Email},
 })
}

func (ac *AuthController) PublicSchool(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	var school models.School
	if slug == "" || ac.DB.Where("lower(slug) = ?", strings.ToLower(slug)).First(&school).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "school not found"}); return
	}
	if school.Status != models.SchoolStatusActive {
		c.JSON(http.StatusGone, gin.H{"error": "school account is currently inactive"}); return
	}
	logo := strings.TrimSpace(school.LogoURL)
	c.JSON(http.StatusOK, gin.H{"id": school.ID, "name": school.Name, "code": school.Code, "slug": school.Slug, "logo_url": logo})
}

func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err))
		return
	}
	sessionID := uuid.New()
	var user models.User
	var access, rawRefresh string
	err := ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("email = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
			return err
		}
		if !user.Active || !auth.CheckPassword(user.PasswordHash, req.Password) {
			return gorm.ErrRecordNotFound
		}
		if tx.Dialector.Name() == "postgres" {
			schoolValue := ""
			if user.SchoolID != nil && *user.SchoolID != uuid.Nil {
				schoolValue = user.SchoolID.String()
			}
			if err := tx.Exec("SELECT set_config('app.current_user_id', ?, true), set_config('app.current_school_id', ?, true)", user.ID.String(), schoolValue).Error; err != nil {
				return err
			}
		}
		if err := validateSchoolAccess(tx, user, req.SchoolCode, req.SchoolSlug); err != nil {
			return gorm.ErrRecordNotFound
		}
		var err error
		access, err = auth.GenerateTokenForSession(user.ID.String(), user.Role, sessionID.String())
		if err != nil { return err }
		rawRefresh, err = auth.GenerateRefreshToken(user.ID.String())
		if err != nil { return err }
		familyID := uuid.New()
		expiry := time.Now().Add(auth.RefreshTokenTTL)
		hash := auth.HashRefreshToken(rawRefresh)
		if err := tx.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: familyID, TokenHash: hash, ExpiresAt: expiry}).Error; err != nil { return err }
		if err := tx.Create(&models.Session{ID: sessionID, UserID: user.ID, FamilyID: familyID, RefreshTokenHash: hash, ExpiresAt: expiry, UserAgent: c.Request.UserAgent(), IP: c.ClientIP()}).Error; err != nil { return err }
		return audit.Record(tx, c, &user.ID, "auth.login", "user", &user.ID, nil)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": access, "refresh_token": rawRefresh})
}
func (ac *AuthController) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err))
		return
	}
	tokenHash := auth.HashRefreshToken(req.RefreshToken)
	var user models.User
	var accessToken, newRefreshToken string
	reused := false
	err := ac.DB.Transaction(func(tx *gorm.DB) error {
		var stored models.RefreshToken
		if err := tx.Where("token_hash = ?", tokenHash).First(&stored).Error; err != nil { return err }
		// The hashed refresh token is the bootstrap credential. After resolving
		// its owner, all remaining operations carry the user identity context.
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT set_config('app.current_user_id', ?, true)", stored.UserID.String()).Error; err != nil { return err }
		}
		revokeFamily := func() error {
			if err := tx.Model(&models.RefreshToken{}).Where("family_id = ? AND user_id = ?", stored.FamilyID, stored.UserID).Update("revoked", true).Error; err != nil { return err }
			if err := tx.Model(&models.Session{}).Where("family_id = ? AND user_id = ?", stored.FamilyID, stored.UserID).Update("revoked", true).Error; err != nil { return err }
			return audit.Record(tx, c, &stored.UserID, "security.refresh_token_reuse", "user", &stored.UserID, map[string]interface{}{"family_id": stored.FamilyID.String()})
		}
		if stored.Revoked {
			if err := revokeFamily(); err != nil { return err }
			reused = true
			return nil // Commit revocation and audit together.
		}
		now := time.Now()
		if !now.Before(stored.ExpiresAt) { return gorm.ErrRecordNotFound }
		if err := tx.Where("id = ? AND active = true", stored.UserID).First(&user).Error; err != nil { return gorm.ErrRecordNotFound }
		if err := validateSchoolAccess(tx, user, "", ""); err != nil { return gorm.ErrRecordNotFound }
		var session models.Session
		if err := tx.Where("refresh_token_hash = ? AND user_id = ? AND family_id = ? AND revoked = false AND expires_at > ?", tokenHash, user.ID, stored.FamilyID, now).First(&session).Error; err != nil {
			return gorm.ErrRecordNotFound
		}
		var err error
		accessToken, err = auth.GenerateTokenForSession(user.ID.String(), user.Role, session.ID.String())
		if err != nil { return err }
		newRefreshToken, err = auth.GenerateRefreshToken(user.ID.String())
		if err != nil { return err }
		newHash := auth.HashRefreshToken(newRefreshToken)
		expiry := now.Add(auth.RefreshTokenTTL)
		claim := tx.Model(&models.RefreshToken{}).Where("id = ? AND user_id = ? AND family_id = ? AND revoked = false AND expires_at > ?", stored.ID, user.ID, stored.FamilyID, now).Update("revoked", true)
		if claim.Error != nil { return claim.Error }
		if claim.RowsAffected != 1 {
			if err := revokeFamily(); err != nil { return err }
			reused = true
			return nil
		}
		if err := tx.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: stored.FamilyID, TokenHash: newHash, ExpiresAt: expiry}).Error; err != nil { return err }
		rotated := tx.Model(&models.Session{}).Where("id = ? AND user_id = ? AND family_id = ? AND refresh_token_hash = ? AND revoked = false", session.ID, user.ID, stored.FamilyID, tokenHash).Updates(map[string]interface{}{"refresh_token_hash": newHash, "expires_at": expiry})
		if rotated.Error != nil { return rotated.Error }
		if rotated.RowsAffected != 1 {
			if err := revokeFamily(); err != nil { return err }
			reused = true
			return nil
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate refresh token"})
		return
	}
	if reused {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or already used refresh token; session revoked"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken, "refresh_token": newRefreshToken})
}
func (ac *AuthController) ChangePassword(c *gin.Context) {
	tx, ok := ac.requestIdentityDB(c)
	if !ok { return }
	userID, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user_id", "authenticated user context is invalid"); return }
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	var user models.User
	if err := tx.First(&user, "id = ? AND active = true", userID).Error; err != nil || !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		httpx.Error(c, http.StatusUnauthorized, "current_password_invalid", "current password is incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil { httpx.Error(c, http.StatusInternalServerError, "password_hash_failed", "failed to secure new password"); return }
	err = tx.Transaction(func(scoped *gorm.DB) error {
		if err := scoped.Model(&models.User{}).Where("id = ? AND active = true", userID).Update("password_hash", hash).Error; err != nil { return err }
		if err := scoped.Model(&models.RefreshToken{}).Where("user_id = ?", userID).Update("revoked", true).Error; err != nil { return err }
		if err := scoped.Model(&models.Session{}).Where("user_id = ?", userID).Update("revoked", true).Error; err != nil { return err }
		return audit.Record(scoped, c, &userID, "security.password_change", "user", &userID, nil)
	})
	if err != nil { httpx.Error(c, http.StatusInternalServerError, "password_change_failed", "failed to change password"); return }
	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully; please sign in again"})
}

func (ac *AuthController) GetSessions(c *gin.Context) {
	tx, ok := ac.requestIdentityDB(c)
	if !ok { return }
	userID, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user_id", "authenticated user context is invalid"); return }
	var sessions []models.Session
	if err := tx.Where("user_id = ? AND revoked = false", userID).Order("created_at DESC").Find(&sessions).Error; err != nil {
		httpx.Error(c, http.StatusInternalServerError, "sessions_read_failed", "failed to load sessions")
		return
	}
	c.JSON(http.StatusOK, sessions)
}

func (ac *AuthController) RevokeSession(c *gin.Context) {
	tx, ok := ac.requestIdentityDB(c)
	if !ok { return }
	userID, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user_id", "authenticated user context is invalid"); return }
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_session_id", "invalid session id"); return }

	err = tx.Transaction(func(scoped *gorm.DB) error {
		var session models.Session
		if err := scoped.Where("id = ? AND user_id = ? AND revoked = false", sessionID, userID).First(&session).Error; err != nil {
			return err
		}
		revoked := scoped.Model(&models.Session{}).Where("id = ? AND user_id = ? AND revoked = false", sessionID, userID).Update("revoked", true)
		if revoked.Error != nil { return revoked.Error }
		if revoked.RowsAffected != 1 { return gorm.ErrRecordNotFound }
		tokenRevoked := scoped.Model(&models.RefreshToken{}).Where("token_hash = ? AND user_id = ?", session.RefreshTokenHash, userID).Update("revoked", true)
		if tokenRevoked.Error != nil { return tokenRevoked.Error }
		if tokenRevoked.RowsAffected != 1 { return errSessionRefreshTokenMissing }
		return audit.Record(scoped, c, &userID, "auth.session_revoke", "session", &sessionID, nil)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Error(c, http.StatusNotFound, "session_not_found", "session not found")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "session_revoke_failed", "failed to revoke session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "session revoked"})
}

func (ac *AuthController) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err)); return
	}
	var token models.RefreshToken
	if err := ac.DB.Where("token_hash = ? AND revoked = false", auth.HashRefreshToken(req.RefreshToken)).First(&token).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	if err := ac.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&token).Update("revoked", true).Error; err != nil { return err }
		return tx.Model(&models.Session{}).Where("refresh_token_hash = ?", auth.HashRefreshToken(req.RefreshToken)).Update("revoked", true).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout"})
		return
	}
	_ = audit.Record(ac.DB, c, &token.UserID, "auth.logout", "user", &token.UserID, nil)
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

func (ac *AuthController) LogoutAll(c *gin.Context) {
	tx, ok := ac.requestIdentityDB(c)
	if !ok { return }
	userID, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user_id", "authenticated user context is invalid"); return }
	err = tx.Transaction(func(scoped *gorm.DB) error {
		if err := scoped.Model(&models.RefreshToken{}).Where("user_id = ? AND revoked = false", userID).Update("revoked", true).Error; err != nil { return err }
		if err := scoped.Model(&models.Session{}).Where("user_id = ? AND revoked = false", userID).Update("revoked", true).Error; err != nil { return err }
		return audit.Record(scoped, c, &userID, "auth.logout_all", "user", &userID, nil)
	})
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "logout_all_failed", "failed to revoke all sessions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out from all devices"})
}

func (ac *AuthController) setUserActive(c *gin.Context, active bool) {
    tx, ok := ac.requestTenantDB(c)
    if !ok { return }
    id, err := uuid.Parse(c.Param("id"))
    if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_user_id", "invalid user id"); return }
    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }
    actorID, err := uuid.Parse(c.GetString("user_id"))
    if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user", "invalid authenticated user"); return }

    var user models.User
    if err := tx.Where("school_id = ? AND id = ?", schoolID, id).First(&user).Error; err != nil {
        httpx.Error(c, http.StatusNotFound, "user_not_found", "user not found"); return
    }
    err = tx.Transaction(func(scoped *gorm.DB) error {
        if err := scoped.Model(&models.User{}).Where("school_id = ? AND id = ?", schoolID, user.ID).Update("active", active).Error; err != nil { return err }
        if !active {
            if err := scoped.Model(&models.RefreshToken{}).Where("user_id = ?", user.ID).Update("revoked", true).Error; err != nil { return err }
            if err := scoped.Model(&models.Session{}).Where("user_id = ?", user.ID).Update("revoked", true).Error; err != nil { return err }
        }
        action := "user.deactivate"
        if active { action = "user.activate" }
        return audit.Record(scoped, c, &actorID, action, "user", &user.ID, nil)
    })
    if err != nil { httpx.Error(c, http.StatusInternalServerError, "account_status_update_failed", "failed to update account status"); return }
    c.JSON(http.StatusOK, gin.H{"message": map[bool]string{true:"account activated",false:"account deactivated"}[active]})
}

func (ac *AuthController) ActivateUser(c *gin.Context) { ac.setUserActive(c, true) }
func (ac *AuthController) DeactivateUser(c *gin.Context) { ac.setUserActive(c, false) }

func (ac *AuthController) AssignRole(c *gin.Context) {
    tx, scoped := ac.requestTenantDB(c)
    if !scoped { return }
    targetID, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }

    var req AssignRoleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        httpx.Validation(c, httpx.ValidationErrors(err)); return
    }
    if !authz.IsValidRole(req.Role) || req.Role == authz.RoleSuperAdmin {
        httpx.Error(c, http.StatusBadRequest, "invalid_role", "invalid role"); return
    }

    actorID, err := uuid.Parse(c.GetString("user_id"))
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok {
        c.JSON(http.StatusForbidden, gin.H{"error": "school context required"})
        return
    }

    var target models.User
    if err := tx.Where("school_id = ? AND id = ?", schoolID, targetID).First(&target).Error; err != nil {
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
    err = tx.Transaction(func(scoped *gorm.DB) error {
        var teacherProfile models.TeacherProfile
        teacherProfileErr := scoped.Where("school_id = ? AND user_id = ?", schoolID, target.ID).First(&teacherProfile).Error
        if teacherProfileErr != nil && !errors.Is(teacherProfileErr, gorm.ErrRecordNotFound) {
            return teacherProfileErr
        }
        hasTeacherProfile := teacherProfileErr == nil

        var parentProfile models.ParentProfile
        parentProfileErr := scoped.Where("school_id = ? AND user_id = ?", schoolID, target.ID).First(&parentProfile).Error
        if parentProfileErr != nil && !errors.Is(parentProfileErr, gorm.ErrRecordNotFound) {
            return parentProfileErr
        }
        hasParentProfile := parentProfileErr == nil

        if hasTeacherProfile && req.Role != authz.RoleTeacher {
            return service.ErrRoleProfileMismatch
        }
        if hasParentProfile && req.Role != authz.RoleParent {
            return service.ErrRoleProfileMismatch
        }
        if req.Role == authz.RoleTeacher && !hasTeacherProfile {
            return service.ErrRoleProfileMismatch
        }
        if req.Role == authz.RoleParent && !hasParentProfile {
            return service.ErrRoleProfileMismatch
        }

        if err := scoped.Model(&models.User{}).Where("school_id = ? AND id = ?", schoolID, target.ID).Update("role", req.Role).Error; err != nil {
            return err
        }
        audit := models.RoleChangeAudit{
            ID: uuid.New(),
            SchoolID: &schoolID,
            ActorID: actorID,
            TargetID: target.ID,
            FromRole: previousRole,
            ToRole: req.Role,
            IP: c.ClientIP(),
        }
        return scoped.Create(&audit).Error
    })
    if errors.Is(err, service.ErrRoleProfileMismatch) {
        httpx.Error(c, http.StatusConflict, "role_profile_mismatch", "role change would leave the user role inconsistent with an existing role-specific profile; update the profile first")
        return
    }
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

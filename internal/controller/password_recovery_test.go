package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/models"
)

type passwordResetMailerStub struct {
	name, email, token string
	err error
	calls int
}

func (m *passwordResetMailerStub) SendPasswordReset(name, email, token string) error {
	m.calls++
	m.name, m.email, m.token = name, email, token
	return m.err
}

func TestForgotPasswordSendsTokenOnlyThroughMailer(t *testing.T) {
	db := authControllerTestDB(t)
	user := models.User{Name: "Recovery User", Email: "recovery@example.com", PasswordHash: "unused", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	mailer := &passwordResetMailerStub{}
	ac := NewAuthController(db)
	ac.Mailer = mailer

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/forgot-password", ac.ForgotPassword)
	req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"RECOVERY@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String()) }
	if mailer.calls != 1 || mailer.email != user.Email || mailer.token == "" { t.Fatalf("reset email was not sent: %#v", mailer) }
	if strings.Contains(rec.Body.String(), mailer.token) || strings.Contains(rec.Body.String(), "reset_token") { t.Fatal("raw reset token must not be returned by the API") }
	var stored models.PasswordResetToken
	if err := db.Where("user_id = ?", user.ID).First(&stored).Error; err != nil { t.Fatal(err) }
	if stored.TokenHash != auth.HashResetToken(mailer.token) { t.Fatal("database must store only the token hash") }
}

func TestForgotPasswordInvalidatesTokenWhenMailDeliveryFails(t *testing.T) {
	db := authControllerTestDB(t)
	user := models.User{Name: "Recovery User", Email: "delivery-failure@example.com", PasswordHash: "unused", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	mailer := &passwordResetMailerStub{err: errMailDelivery}
	ac := NewAuthController(db)
	ac.Mailer = mailer

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/forgot-password", ac.ForgotPassword)
	req := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", strings.NewReader(`{"email":"delivery-failure@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK { t.Fatalf("response should not disclose account existence: %d", rec.Code) }
	var stored models.PasswordResetToken
	if err := db.Where("user_id = ?", user.ID).First(&stored).Error; err != nil { t.Fatal(err) }
	if !stored.Used { t.Fatal("undelivered reset token must be invalidated") }
}

var errMailDelivery = &mailDeliveryError{}

type mailDeliveryError struct{}
func (*mailDeliveryError) Error() string { return "simulated mail delivery failure" }

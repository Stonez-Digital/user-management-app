package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/controller"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
)

func main() {
	if err := auth.ConfigureSecret(os.Getenv("JWT_SECRET")); err != nil {
		log.Fatal(err)
	}

	db := database.Connect()

	var adminUser models.User
	if err := db.Where("email = ?", "admin@example.com").First(&adminUser).Error; err == nil {
		if !adminUser.Active || adminUser.Role != "admin" {
			adminUser.Role = "admin"
			if err := db.Save(&adminUser).Error; err != nil {
				log.Fatal("failed to configure admin user: ", err)
			}
		}
	}

	r := gin.Default()
	r.SetTrustedProxies(nil)
	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)
	ctrl := controller.NewUserController(svc)
	authController := controller.NewAuthController(db)

	r.POST("/auth/refresh", authController.Refresh)
	r.POST("/auth/register", authController.Register)
	r.POST("/auth/login", authController.Login)
	r.POST("/users", ctrl.CreateUser)
	r.POST("/auth/logout", authController.Logout)
	r.POST("/auth/forgot-password", authController.ForgotPassword)
	r.POST("/auth/reset-password", authController.ResetPassword)

	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(db))
	protected.GET("/users", ctrl.GetUsers)
	protected.GET("/users/:id", ctrl.GetUser)
	protected.GET("/me", ctrl.GetMe)
	protected.PUT("/me", ctrl.UpdateMe)
	protected.POST("/auth/change-password", authController.ChangePassword)
	protected.POST("/auth/logout-all", authController.LogoutAll)
	protected.GET("/sessions", authController.GetSessions)
	protected.DELETE("/sessions/:id", authController.RevokeSession)

	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware(db), middleware.AdminOnly())
	admin.GET("/users", ctrl.GetUsers)
	admin.DELETE("/users/:id", ctrl.DeleteUser)
	admin.POST("/users/:id/activate", authController.ActivateUser)
	admin.POST("/users/:id/deactivate", authController.DeactivateUser)

	port := os.Getenv("APP_PORT")
	if port == "" { port = "8080" }
	log.Println("Server running on :" + port)
	if err := r.Run(":" + port); err != nil { log.Fatal(err) }
}

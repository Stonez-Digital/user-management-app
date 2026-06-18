package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/onoja217/users-management-app/internal/controller"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
)

func main() {
	db := database.Connect()

	// Promote admin@example.com to admin
	var adminUser models.User
	if err := db.Where("email = ?", "admin@example.com").First(&adminUser).Error; err == nil {
		adminUser.Role = "admin"
		db.Save(&adminUser)
	}

	r := gin.Default()
	r.SetTrustedProxies(nil)

	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)
	ctrl := controller.NewUserController(svc)

	authController := controller.NewAuthController(db)

	// Public routes
	r.POST("/auth/register", authController.Register)
	r.POST("/auth/login", authController.Login)

	// Admin routes
	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminOnly())

	admin.GET("/users", ctrl.GetUsers)
	admin.DELETE("/users/:id", ctrl.DeleteUser)

	// Public user creation
	r.POST("/users", ctrl.CreateUser)

	// Protected routes
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())

	protected.GET("/users", ctrl.GetUsers)
	protected.GET("/users/:id", ctrl.GetUser)
	protected.DELETE("/users/:id", ctrl.DeleteUser)

	log.Println("Server running on :8080")

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/onoja217/users-management-app/internal/controller"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
)

func main() {

	db := database.Connect()

	r := gin.Default()
	r.SetTrustedProxies(nil)

	// repositories & services (existing user system)
	repo := repository.NewUserRepository(db)
	svc := service.NewUserService(repo)
	ctrl := controller.NewUserController(svc)

	// auth controller
	authController := controller.NewAuthController(db)

	// AUTH ROUTES
	r.POST("/auth/register", authController.Register)
	r.POST("/auth/login", authController.Login)

	// USER ROUTES
	r.POST("/users", ctrl.CreateUser)
	r.GET("/users", ctrl.GetUsers)
	r.GET("/users/:id", ctrl.GetUser)
	r.DELETE("/users/:id", ctrl.DeleteUser)

	log.Println("Server running on :8080")
	r.Run(":8080")
}
package main

import (
    "log"
    "os"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/auth"
    "github.com/onoja217/users-management-app/internal/authz"
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

    db, err := database.Connect()
    if err != nil {
        log.Fatal(err)
    }
    if err := database.Migrate(db); err != nil {
        log.Fatal(err)
    }

    // Normalize roles created before the school RBAC model existed.
    db.Model(&models.User{}).Where("role = ?", "admin").Update("role", authz.RoleSuperAdmin)
    db.Model(&models.User{}).Where("role = ?", "user").Update("role", authz.RoleStudent)

    var adminUser models.User
    if err := db.Where("email = ?", "admin@example.com").First(&adminUser).Error; err == nil {
        if !adminUser.Active || adminUser.Role != authz.RoleSuperAdmin {
            adminUser.Role = authz.RoleSuperAdmin
            adminUser.Active = true
            if err := db.Save(&adminUser).Error; err != nil {
                log.Fatal("failed to configure admin user: ", err)
            }
        }
    }

    r := gin.Default()
    r.SetTrustedProxies(nil)
    repo := repository.NewUserRepository(db)
    svc := service.NewUserService(repo, db)
    ctrl := controller.NewUserController(svc)
    authController := controller.NewAuthController(db)
    auditController := controller.NewAuditController(db)
    studentRepo := repository.NewStudentRepository(db)
    studentService := service.NewStudentService(studentRepo, db)
    studentController := controller.NewStudentController(studentService)

    r.POST("/auth/refresh", authController.Refresh)
    r.POST("/auth/register", authController.Register)
    r.POST("/auth/login", authController.Login)
    r.POST("/auth/logout", authController.Logout)
    r.POST("/auth/forgot-password", authController.ForgotPassword)
    r.POST("/auth/reset-password", authController.ResetPassword)

    protected := r.Group("/")
    protected.Use(middleware.AuthMiddleware(db))
    protected.GET("/me", ctrl.GetMe)
    protected.PUT("/me", ctrl.UpdateMe)
    protected.POST("/auth/change-password", authController.ChangePassword)
    protected.POST("/auth/logout-all", authController.LogoutAll)
    protected.GET("/sessions", authController.GetSessions)
    protected.DELETE("/sessions/:id", authController.RevokeSession)

    admin := r.Group("/admin")
    admin.Use(middleware.AuthMiddleware(db))
    admin.GET("/roles", middleware.RequirePermission(authz.PermissionRolesAssign), authController.ListRoles)
    admin.GET("/audit-logs", middleware.RequirePermission(authz.PermissionAuditRead), auditController.List)
    admin.POST("/students", middleware.RequirePermission(authz.PermissionStudentsCreate), studentController.Create)
    admin.GET("/students", middleware.RequirePermission(authz.PermissionStudentsRead), studentController.List)
    admin.GET("/students/:id", middleware.RequirePermission(authz.PermissionStudentsRead), studentController.Get)
    admin.PUT("/students/:id", middleware.RequirePermission(authz.PermissionStudentsUpdate), studentController.Update)
    admin.DELETE("/students/:id", middleware.RequirePermission(authz.PermissionStudentsDelete), studentController.Delete)
    admin.POST("/users", middleware.RequirePermission(authz.PermissionUsersCreate), ctrl.CreateUser)
    admin.GET("/users", middleware.RequirePermission(authz.PermissionUsersRead), ctrl.GetUsers)
    admin.GET("/users/:id", middleware.RequirePermission(authz.PermissionUsersRead), ctrl.GetUser)
    admin.PUT("/users/:id/role", middleware.RequirePermission(authz.PermissionRolesAssign), authController.AssignRole)
    admin.DELETE("/users/:id", middleware.RequirePermission(authz.PermissionUsersDelete), ctrl.DeleteUser)
    admin.POST("/users/:id/activate", middleware.RequirePermission(authz.PermissionUsersActivate), authController.ActivateUser)
    admin.POST("/users/:id/deactivate", middleware.RequirePermission(authz.PermissionUsersDeactivate), authController.DeactivateUser)

    port := os.Getenv("APP_PORT")
    if port == "" { port = "8080" }
    log.Println("Server running on :" + port)
    if err := r.Run(":" + port); err != nil { log.Fatal(err) }
}

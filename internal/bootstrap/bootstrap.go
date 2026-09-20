package bootstrap

import (
    "fmt"
    "os"
    "strings"

    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

// EnsureInitialAdmin creates the first superadmin only when the database is empty
// and the explicit bootstrap environment variables are configured.
func EnsureInitialAdmin(db *gorm.DB) error {
    passwordHash := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_PASSWORD_HASH"))
    if passwordHash == "" {
        return nil
    }

    email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_EMAIL")))
    name := strings.TrimSpace(os.Getenv("ADMIN_BOOTSTRAP_NAME"))
    if email == "" || name == "" {
        return fmt.Errorf("ADMIN_BOOTSTRAP_EMAIL and ADMIN_BOOTSTRAP_NAME are required when ADMIN_BOOTSTRAP_PASSWORD_HASH is set")
    }

    var userCount int64
    if err := db.Model(&models.User{}).Count(&userCount).Error; err != nil {
        return fmt.Errorf("check administrator bootstrap state: %w", err)
    }
    if userCount != 0 {
        return nil
    }

    var school models.School
    if err := db.Where("code = ?", "DEFAULT").First(&school).Error; err != nil {
        return fmt.Errorf("load default school for administrator bootstrap: %w", err)
    }

    user := models.User{
        SchoolID:     &school.ID,
        Name:         name,
        Email:        email,
        PasswordHash: passwordHash,
        Role:         authz.RoleSuperAdmin,
        Active:       true,
    }
    if err := db.Create(&user).Error; err != nil {
        return fmt.Errorf("create administrator bootstrap user: %w", err)
    }
    return nil
}

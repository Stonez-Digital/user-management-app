package bootstrap

import (
    "testing"

    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func bootstrapTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.User{}); err != nil { t.Fatal(err) }
    school := models.School{Name: "Default School", Code: "DEFAULT", Status: models.SchoolStatusActive}
    if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
    return db
}

func TestEnsureInitialAdminDoesNotPromoteLegacyAdminUser(t *testing.T) {
    db := bootstrapTestDB(t)
    legacy := models.User{Name: "Legacy", Email: "legacy@example.com", PasswordHash: "hash", Role: "admin", Active: true}
    if err := db.Create(&legacy).Error; err != nil { t.Fatal(err) }

    t.Setenv("ADMIN_BOOTSTRAP_PASSWORD_HASH", "bcrypt-hash")
    t.Setenv("ADMIN_BOOTSTRAP_EMAIL", "bootstrap@example.com")
    t.Setenv("ADMIN_BOOTSTRAP_NAME", "Bootstrap")

    if err := EnsureInitialAdmin(db); err != nil { t.Fatal(err) }

    var users []models.User
    if err := db.Find(&users).Error; err != nil { t.Fatal(err) }
    if len(users) != 1 { t.Fatalf("expected no bootstrap user when database is non-empty, got %d users", len(users)) }
    if users[0].Role != "admin" { t.Fatalf("expected legacy role to remain unchanged here, got %q", users[0].Role) }
}

func TestEnsureInitialAdminRequiresCompleteConfiguration(t *testing.T) {
    db := bootstrapTestDB(t)
    t.Setenv("ADMIN_BOOTSTRAP_PASSWORD_HASH", "bcrypt-hash")
    t.Setenv("ADMIN_BOOTSTRAP_EMAIL", "")
    t.Setenv("ADMIN_BOOTSTRAP_NAME", "")

    if err := EnsureInitialAdmin(db); err == nil { t.Fatal("expected incomplete bootstrap configuration to fail") }
}

func TestEnsureInitialAdminCreatesOnlyFirstUser(t *testing.T) {
    db := bootstrapTestDB(t)
    t.Setenv("ADMIN_BOOTSTRAP_PASSWORD_HASH", "bcrypt-hash")
    t.Setenv("ADMIN_BOOTSTRAP_EMAIL", "Admin@Example.com")
    t.Setenv("ADMIN_BOOTSTRAP_NAME", "First Admin")

    if err := EnsureInitialAdmin(db); err != nil { t.Fatal(err) }
    if err := EnsureInitialAdmin(db); err != nil { t.Fatal(err) }

    var user models.User
    if err := db.First(&user).Error; err != nil { t.Fatal(err) }
    if user.Email != "admin@example.com" { t.Fatalf("expected normalized bootstrap email, got %q", user.Email) }
    if user.Role != authz.RoleSuperAdmin { t.Fatalf("expected superadmin role, got %q", user.Role) }
    if user.SchoolID == nil { t.Fatal("expected bootstrap administrator to have a school context") }
    var school models.School
    if err := db.First(&school, "code = ?", "DEFAULT").Error; err != nil { t.Fatal(err) }
    if *user.SchoolID != school.ID { t.Fatal("expected bootstrap administrator to use the default school") }
}

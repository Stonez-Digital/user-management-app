package database
import("github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
func migrateBulkImport(db *gorm.DB) error { return db.AutoMigrate(&models.BulkImportJob{}) }

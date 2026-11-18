package audit

import (
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestRecordDerivesSchoolFromActorWhenContextIsAbsent(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:audit_record_test?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.AuditLog{}); err != nil { t.Fatal(err) }

    school := models.School{ID: uuid.New(), Name: "Audit School", Code: "AUDIT-REC"}
    if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
    user := models.User{ID: uuid.New(), SchoolID: &school.ID, Name: "Actor", Email: "actor@example.com", Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }

    gin.SetMode(gin.TestMode)
    c, _ := gin.CreateTestContext(httptest.NewRecorder())
    c.Request = httptest.NewRequest("POST", "/test", nil)

    if err := Record(db, c, &user.ID, "test.action", "user", &user.ID, nil); err != nil { t.Fatal(err) }

    var log models.AuditLog
    if err := db.First(&log).Error; err != nil { t.Fatal(err) }
    if log.SchoolID == nil || *log.SchoolID != school.ID { t.Fatalf("expected audit log school %s, got %v", school.ID, log.SchoolID) }
}

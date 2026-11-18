package audit

import (
    "testing"
    "net/http/httptest"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestRecordPersistsAuditLog(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:audit-test?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.AuditLog{}); err != nil { t.Fatal(err) }

    gin.SetMode(gin.TestMode)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest("GET", "/", nil)
    actor := uuid.New()
    resource := uuid.New()

    if err := Record(db, c, &actor, "user.update", "user", &resource, map[string]interface{}{"field": "name"}); err != nil {
        t.Fatal(err)
    }

    var log models.AuditLog
    if err := db.First(&log).Error; err != nil { t.Fatal(err) }
    if log.Action != "user.update" || log.Resource != "user" || log.ActorID == nil || *log.ActorID != actor {
        t.Fatalf("unexpected audit record: %+v", log)
    }
}

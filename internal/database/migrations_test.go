package database

import (
    "testing"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestMigrateIsVersionedAndIdempotent(t *testing.T) {
    db,err:=gorm.Open(sqlite.Open("file::memory:?cache=shared"),&gorm.Config{}); if err!=nil{t.Fatal(err)}
    if err:=Migrate(db);err!=nil{t.Fatal(err)}
    if err:=Migrate(db);err!=nil{t.Fatal(err)}
    var count int64
    if err:=db.Model(&Migration{}).Count(&count).Error;err!=nil{t.Fatal(err)}
    if count!=16{t.Fatalf("expected sixteen applied migrations, got %d",count)}
    if !db.Migrator().HasTable(&models.User{}){t.Fatal("expected users table after migration")}
    if !db.Migrator().HasTable(&models.School{}){t.Fatal("expected schools table after migration")}
    var schools int64
    if err:=db.Model(&models.School{}).Count(&schools).Error;err!=nil{t.Fatal(err)}
    if schools!=1{t.Fatalf("expected one default school, got %d",schools)}
    var usersWithoutSchool int64
    if err:=db.Model(&models.User{}).Where("school_id IS NULL").Count(&usersWithoutSchool).Error;err!=nil{t.Fatal(err)}
    if usersWithoutSchool!=0{t.Fatalf("expected all existing users to be assigned to a school, got %d",usersWithoutSchool)}
    if !db.Migrator().HasTable(&models.Student{}){t.Fatal("expected students table after migration")}
    if !db.Migrator().HasTable(&models.StudentEnrollment{}){t.Fatal("expected student enrollments table after migration")}
    if !db.Migrator().HasTable(&models.AttendanceRecord{}){t.Fatal("expected attendance records table after migration")}
    if !db.Migrator().HasTable(&models.Assessment{}){t.Fatal("expected assessments table after migration")}
    if !db.Migrator().HasTable(&models.AssessmentResult{}){t.Fatal("expected assessment results table after migration")}
    if !db.Migrator().HasTable(&models.FeeItem{}){t.Fatal("expected fee items table after migration")}
    if !db.Migrator().HasTable(&models.Invoice{}){t.Fatal("expected invoices table after migration")}
    if !db.Migrator().HasTable(&models.InvoiceLine{}){t.Fatal("expected invoice lines table after migration")}
    if !db.Migrator().HasTable(&models.Payment{}){t.Fatal("expected payments table after migration")}
    if !db.Migrator().HasTable(&models.TimetableEntry{}){t.Fatal("expected timetable table after migration")}
    if !db.Migrator().HasTable(&models.GuardianRelationship{}){t.Fatal("expected guardian relationships table after migration")}
    if !db.Migrator().HasTable(&models.Notification{}){t.Fatal("expected notifications table after migration")}
}

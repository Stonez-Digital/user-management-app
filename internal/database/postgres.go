package database

import (
    "fmt"
    "os"

    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/postgres"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
    driver := os.Getenv("DB_DRIVER")
    if driver == "" { driver = "sqlite" }
    switch driver {
    case "postgres":
        dsn := os.Getenv("DATABASE_URL")
        if dsn == "" {
            dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
                envOrDefault("DB_HOST","localhost"), envOrDefault("DB_USER","postgres"), os.Getenv("DB_PASSWORD"),
                envOrDefault("DB_NAME","usersdb"), envOrDefault("DB_PORT","5432"), envOrDefault("DB_SSLMODE","require"))
        }
        db,err:=gorm.Open(postgres.Open(dsn),&gorm.Config{}); if err!=nil{return nil,fmt.Errorf("connect postgres database: %w",err)}; return db,nil
    case "sqlite":
        db,err:=gorm.Open(sqlite.Open(envOrDefault("DB_PATH","users.db")),&gorm.Config{}); if err!=nil{return nil,fmt.Errorf("connect sqlite database: %w",err)}; return db,nil
    default: return nil,fmt.Errorf("unsupported DB_DRIVER %q",driver)
    }
}
func envOrDefault(key,fallback string) string { if value:=os.Getenv(key); value!="" { return value }; return fallback }

func Migrate(db *gorm.DB) error {
    if !db.Migrator().HasTable(&Migration{}) {
        if err:=db.AutoMigrate(&Migration{});err!=nil{return fmt.Errorf("create migration table: %w",err)}
    }
    migrations:=[]MigrationStep{
        {Version:1,Name:"initial_school_management_schema",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.User{},&models.RefreshToken{},&models.Session{},&models.PasswordResetToken{},&models.RoleChangeAudit{},&models.AuditLog{})}},
        {Version:2,Name:"student_management",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.Student{})}},
        {Version:3,Name:"academic_foundation",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.AcademicSession{},&models.Term{})}},
        {Version:4,Name:"classes_and_sections",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.SchoolClass{},&models.Section{})}},
        {Version:5,Name:"student_enrollment",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.StudentEnrollment{})}},
        {Version:6,Name:"attendance_records",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.AttendanceRecord{})}},
        {Version:7,Name:"assessments",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.Assessment{})}},
        {Version:8,Name:"assessment_results",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.AssessmentResult{})}},
        {Version:9,Name:"finance",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.FeeItem{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{})}},
        {Version:10,Name:"timetable",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.TimetableEntry{})}},
        {Version:11,Name:"guardian_relationships",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.GuardianRelationship{})}},
        {Version:12,Name:"notifications",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.Announcement{},&models.Notification{})}},
    }
    for _,migration:=range migrations{
        var applied Migration
        result:=db.Where("version = ?",migration.Version).First(&applied)
        if result.Error==nil{continue}
        if result.Error!=gorm.ErrRecordNotFound{return fmt.Errorf("check migration %d: %w",migration.Version,result.Error)}
        if err:=db.Transaction(func(tx *gorm.DB) error{if err:=migration.Up(tx);err!=nil{return err};return tx.Create(&Migration{Version:migration.Version,Name:migration.Name}).Error});err!=nil{return fmt.Errorf("apply migration %d (%s): %w",migration.Version,migration.Name,err)}
    }
    return nil
}
type Migration struct{Version int `gorm:"primaryKey"`;Name string `gorm:"not null;uniqueIndex"`}
type MigrationStep struct{Version int;Name string;Up func(*gorm.DB) error}

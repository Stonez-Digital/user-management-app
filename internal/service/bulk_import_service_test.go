package service

import(
 "strings"
 "testing"
 "github.com/google/uuid"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
 "github.com/onoja217/users-management-app/internal/models"
)

func bulkTestDB(t *testing.T)*gorm.DB{t.Helper();db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.StudentEnrollment{},&models.GuardianRelationship{},&models.AcademicSession{},&models.SchoolClass{},&models.Section{},&models.BulkImportJob{},&models.TeacherProfile{},&models.ParentProfile{});e!=nil{t.Fatal(e)};return db}

func TestParseCSVAndValidateBulkStudents(t *testing.T){rows,e:=parseCSV(strings.NewReader("admission_number,first_name,last_name,email\nA-1,Jane,Doe,jane@example.com\nA-1,John,Doe,john@example.com\n"));if e!=nil||len(rows)!=2{t.Fatalf("parse: %v rows=%d",e,len(rows))};svc:=NewBulkImportService(bulkTestDB(t));errs,valid:=svc.Validate(uuid.New(),uuid.New(),"students",rows);if len(errs)!=1||valid!=1{t.Fatalf("expected one duplicate error and one valid row, got errors=%d valid=%d",len(errs),valid)}}

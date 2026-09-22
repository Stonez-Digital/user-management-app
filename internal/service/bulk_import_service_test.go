package service

import(
 "strings"
 "testing"
 "github.com/onoja217/users-management-app/internal/models"
 "github.com/google/uuid"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func bulkTestDB(t *testing.T)*gorm.DB{t.Helper();db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.StudentEnrollment{},&models.GuardianRelationship{},&models.AcademicSession{},&models.SchoolClass{},&models.Section{},&models.BulkImportJob{});e!=nil{t.Fatal(e)};return db}

func TestParseCSVAndValidateBulkStudents(t *testing.T){rows,e:=parseCSV(strings.NewReader("admission_number,first_name,last_name,email\nA-1,Jane,Doe,jane@example.com\nA-1,John,Doe,john@example.com\n"));if e!=nil||len(rows)!=2{t.Fatalf("parse: %v rows=%d",e,len(rows))};svc:=NewBulkImportService(bulkTestDB(t));errs,valid:=svc.Validate(uuid.New(),uuid.New(),"students",rows);if len(errs)!=1||valid!=1{t.Fatalf("expected one duplicate error and one valid row, got errors=%d valid=%d",len(errs),valid)}}

func TestBulkStudentImportIsSchoolScoped(t *testing.T){db:=bulkTestDB(t);a:=models.School{Name:"A",Code:"A",Status:models.SchoolStatusActive};b:=models.School{Name:"B",Code:"B",Status:models.SchoolStatusActive};db.Create(&a);db.Create(&b);svc:=NewBulkImportService(db);p,e:=svc.Create(a.ID,uuid.New(),"students","students.csv",[]BulkRow{{"admission_number":"A-1","first_name":"Jane","last_name":"Doe","email":"jane@example.com"}});if e!=nil{t.Fatal(e)};if e=svc.processRow(p.Job,BulkRow{"admission_number":"A-1","first_name":"Jane","last_name":"Doe","email":"jane@example.com"});e!=nil{t.Fatal(e)};var st models.Student;if e=db.Where("school_id=? AND admission_number=?",a.ID,"A-1").First(&st).Error;e!=nil{t.Fatalf("student not imported: %v",e)};var other int64;db.Model(&models.Student{}).Where("school_id=?",b.ID).Count(&other);if other!=0{t.Fatal("import crossed tenant boundary")}}

package service

import(
 "encoding/json"
 "strings"
 "testing"
 "github.com/google/uuid"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
 "github.com/onoja217/users-management-app/internal/models"
)

func bulkTestDB(t *testing.T)*gorm.DB{t.Helper();db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.StudentEnrollment{},&models.GuardianRelationship{},&models.AcademicSession{},&models.SchoolClass{},&models.Section{},&models.BulkImportJob{},&models.TeacherProfile{},&models.ParentProfile{});e!=nil{t.Fatal(e)};return db}

func TestParseCSVAndValidateBulkStudents(t *testing.T){rows,e:=parseCSV(strings.NewReader("admission_number,first_name,last_name,email\nA-1,Jane,Doe,jane@example.com\nA-1,John,Doe,john@example.com\n"));if e!=nil||len(rows)!=2{t.Fatalf("parse: %v rows=%d",e,len(rows))};svc:=NewBulkImportService(bulkTestDB(t),nil);errs,valid:=svc.Validate(uuid.New(),uuid.New(),"students",rows);if len(errs)!=1||valid!=1{t.Fatalf("expected one duplicate error and one valid row, got errors=%d valid=%d",len(errs),valid)}}


func TestRunPersistsSkippedCountForExistingStudent(t *testing.T) {
 db:=bulkTestDB(t)
 schoolID:=uuid.New()
 actor:=uuid.New()
 admission:="STU-001"
 user:=models.User{ID:uuid.New(),Name:"Existing Student",Email:"existing@example.com",Role:"student",Active:true,SchoolID:&schoolID}
 if e:=db.Create(&user).Error;e!=nil{t.Fatal(e)}
 if e:=db.Create(&models.Student{ID:uuid.New(),SchoolID:schoolID,UserID:user.ID,AdmissionNumber:admission}).Error;e!=nil{t.Fatal(e)}
 rows:=[]BulkRow{{"name":"Existing Student","email":"existing@example.com","admission_number":admission}}
 rb,_:=json.Marshal(rows)
 job:=models.BulkImportJob{ID:uuid.New(),SchoolID:schoolID,InitiatedBy:actor,Kind:"students",Status:models.BulkImportPending,Total:1,RowsJSON:string(rb)}
 if e:=db.Create(&job).Error;e!=nil{t.Fatal(e)}
 svc:=NewBulkImportService(db,nil)
 svc.run(job)
 var got models.BulkImportJob
 if e:=db.First(&got,job.ID).Error;e!=nil{t.Fatal(e)}
 if got.Skipped!=1||got.Created!=0||got.Failed!=0{t.Fatalf("expected skipped=1 created=0 failed=0, got skipped=%d created=%d failed=%d",got.Skipped,got.Created,got.Failed)}
}


func TestRunPersistsSkippedCountForExistingTeacher(t *testing.T) {
 db:=bulkTestDB(t)
 schoolID:=uuid.New()
 actor:=uuid.New()
 user:=models.User{ID:uuid.New(),Name:"Existing Teacher",Email:"teacher@example.com",Role:"teacher",Active:true,SchoolID:&schoolID}
 if e:=db.Create(&user).Error;e!=nil{t.Fatal(e)}
 if e:=db.Create(&models.TeacherProfile{ID:uuid.New(),SchoolID:schoolID,UserID:user.ID,StaffID:"T-001"}).Error;e!=nil{t.Fatal(e)}
 rb,_:=json.Marshal([]BulkRow{{"name":"Existing Teacher","email":"teacher@example.com","staff_id":"T-001"}})
 job:=models.BulkImportJob{ID:uuid.New(),SchoolID:schoolID,InitiatedBy:actor,Kind:"teachers",Status:models.BulkImportPending,Total:1,RowsJSON:string(rb)}
 if e:=db.Create(&job).Error;e!=nil{t.Fatal(e)}
 NewBulkImportService(db,nil).run(job)
 var got models.BulkImportJob
 if e:=db.First(&got,job.ID).Error;e!=nil{t.Fatal(e)}
 if got.Skipped!=1||got.Created!=0||got.Failed!=0{t.Fatalf("expected skipped=1 created=0 failed=0, got skipped=%d created=%d failed=%d",got.Skipped,got.Created,got.Failed)}
}

func TestRunPersistsSkippedCountForExistingParent(t *testing.T) {
 db:=bulkTestDB(t)
 schoolID:=uuid.New()
 actor:=uuid.New()
 user:=models.User{ID:uuid.New(),Name:"Existing Parent",Email:"parent@example.com",Role:"parent",Active:true,SchoolID:&schoolID}
 if e:=db.Create(&user).Error;e!=nil{t.Fatal(e)}
 if e:=db.Create(&models.ParentProfile{ID:uuid.New(),SchoolID:schoolID,UserID:user.ID,ParentIdentifier:"P-001"}).Error;e!=nil{t.Fatal(e)}
 rb,_:=json.Marshal([]BulkRow{{"name":"Existing Parent","email":"parent@example.com","parent_identifier":"P-001"}})
 job:=models.BulkImportJob{ID:uuid.New(),SchoolID:schoolID,InitiatedBy:actor,Kind:"parents",Status:models.BulkImportPending,Total:1,RowsJSON:string(rb)}
 if e:=db.Create(&job).Error;e!=nil{t.Fatal(e)}
 NewBulkImportService(db,nil).run(job)
 var got models.BulkImportJob
 if e:=db.First(&got,job.ID).Error;e!=nil{t.Fatal(e)}
 if got.Skipped!=1||got.Created!=0||got.Failed!=0{t.Fatalf("expected skipped=1 created=0 failed=0, got skipped=%d created=%d failed=%d",got.Skipped,got.Created,got.Failed)}
}

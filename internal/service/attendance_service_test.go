package service
import("fmt";"testing";"time";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/driver/sqlite";"gorm.io/gorm")
func attendanceTestDB(t *testing.T)*gorm.DB{t.Helper();db,e:=gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared",t.Name())),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.User{},&models.Student{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.AttendanceRecord{});e!=nil{t.Fatal(e)};return db}
func TestAttendanceValidatesEnrollmentTermAndDuplicate(t *testing.T){db:=attendanceTestDB(t);schoolID:=uuid.MustParse("00000000-0000-0000-0000-000000000001");u:=models.User{SchoolID:&schoolID,Name:"Attendance Student",Email:"attendance@example.com",Role:"student",Active:true};if e:=db.Create(&u).Error;e!=nil{t.Fatal(e)};st:=models.Student{SchoolID:schoolID,UserID:u.ID,AdmissionNumber:"ATT-001"};if e:=db.Create(&st).Error;e!=nil{t.Fatal(e)};ss:=models.AcademicSession{SchoolID:schoolID,Name:"2026/2027"};if e:=db.Create(&ss).Error;e!=nil{t.Fatal(e)};term:=models.Term{SchoolID:schoolID,AcademicSessionID:ss.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)};if e:=db.Create(&term).Error;e!=nil{t.Fatal(e)};cl:=models.SchoolClass{SchoolID:schoolID,Name:"JSS 1",Level:1};if e:=db.Create(&cl).Error;e!=nil{t.Fatal(e)};sec:=models.Section{SchoolID:schoolID,ClassID:cl.ID,Name:"A"};if e:=db.Create(&sec).Error;e!=nil{t.Fatal(e)};en:=models.StudentEnrollment{SchoolID:schoolID,StudentID:st.ID,AcademicSessionID:ss.ID,ClassID:cl.ID,SectionID:sec.ID,Status:models.EnrollmentStatusActive};if e:=db.Create(&en).Error;e!=nil{t.Fatal(e)};svc:=NewAttendanceService(repository.NewAttendanceRepository(db),db);date:=time.Date(2026,9,19,10,30,0,0,time.Local);rec,e:=svc.Create(schoolID,models.AttendanceRecord{EnrollmentID:en.ID,TermID:term.ID,Date:date,Status:models.AttendancePresent});if e!=nil{t.Fatal(e)};if rec.Status!=models.AttendancePresent{t.Fatalf("expected present, got %s",rec.Status)};if _,e=svc.Create(schoolID,models.AttendanceRecord{EnrollmentID:en.ID,TermID:term.ID,Date:date,Status:models.AttendanceAbsent});e!=ErrAttendanceDuplicate{t.Fatalf("expected duplicate error, got %v",e)};other:=uuid.MustParse("00000000-0000-0000-0000-000000000002");if _,e=svc.Get(other,rec.ID);e!=ErrAttendanceNotFound{t.Fatalf("cross-school get: %v",e)};if e=svc.Update(other,models.AttendanceRecord{ID:rec.ID,EnrollmentID:en.ID,TermID:term.ID,Date:date,Status:models.AttendanceAbsent});e!=ErrAttendanceNotFound{t.Fatalf("cross-school update: %v",e)};if e=svc.Delete(other,rec.ID);e!=ErrAttendanceNotFound{t.Fatalf("cross-school delete: %v",e)}}

func TestAttendanceListFiltersBySchoolSessionAndTermInDatabase(t *testing.T) {
 db := attendanceTestDB(t)
 schoolID := uuid.New()
 user := models.User{SchoolID: &schoolID, Name: "List Student", Email: "list-"+uuid.NewString()+"@example.com", Role: "student", Active: true}
 if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
 student := models.Student{SchoolID: schoolID, UserID: user.ID, AdmissionNumber: "LIST-"+uuid.NewString()[:8]}
 if err := db.Create(&student).Error; err != nil { t.Fatal(err) }

 sessionA := models.AcademicSession{SchoolID: schoolID, Name: "2026/2027"}
 sessionB := models.AcademicSession{SchoolID: schoolID, Name: "2027/2028"}
 if err := db.Create(&sessionA).Error; err != nil { t.Fatal(err) }
 if err := db.Create(&sessionB).Error; err != nil { t.Fatal(err) }
 termA := models.Term{SchoolID: schoolID, AcademicSessionID: sessionA.ID, Name: "First Term", StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2026,12,15,0,0,0,0,time.UTC)}
 termB := models.Term{SchoolID: schoolID, AcademicSessionID: sessionB.ID, Name: "First Term", StartDate: time.Date(2027,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2027,12,15,0,0,0,0,time.UTC)}
 if err := db.Create(&termA).Error; err != nil { t.Fatal(err) }
 if err := db.Create(&termB).Error; err != nil { t.Fatal(err) }
 class := models.SchoolClass{SchoolID: schoolID, Name: "List Class", Level: 1}
 if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
 enrollmentA := models.StudentEnrollment{SchoolID: schoolID, StudentID: student.ID, AcademicSessionID: sessionA.ID, ClassID: class.ID, Status: models.EnrollmentStatusActive}
 enrollmentB := models.StudentEnrollment{SchoolID: schoolID, StudentID: student.ID, AcademicSessionID: sessionB.ID, ClassID: class.ID, Status: models.EnrollmentStatusActive}
 if err := db.Create(&enrollmentA).Error; err != nil { t.Fatal(err) }
 if err := db.Create(&enrollmentB).Error; err != nil { t.Fatal(err) }
 dateA := time.Date(2026,9,10,0,0,0,0,time.UTC)
 dateB := time.Date(2027,9,10,0,0,0,0,time.UTC)
 rows := []models.AttendanceRecord{
  {SchoolID: schoolID, EnrollmentID: enrollmentA.ID, TermID: termA.ID, Date: dateA, Status: models.AttendancePresent},
  {SchoolID: schoolID, EnrollmentID: enrollmentB.ID, TermID: termB.ID, Date: dateB, Status: models.AttendanceAbsent},
 }
 if err := db.Create(&rows).Error; err != nil { t.Fatal(err) }
 svc := NewAttendanceService(repository.NewAttendanceRepository(db), db)
 got, err := svc.List(schoolID, sessionA.ID, termA.ID)
 if err != nil { t.Fatal(err) }
 if len(got) != 1 || got[0].TermID != termA.ID || got[0].EnrollmentID != enrollmentA.ID {
  t.Fatalf("expected only session A / term A attendance, got %#v", got)
 }
}

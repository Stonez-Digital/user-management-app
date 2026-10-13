package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func careerPathTestDB(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:careerpath-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := database.Migrate(db); err != nil { t.Fatal(err) }
	var school models.School
	if err := db.Where("code = ?", "DEFAULT").First(&school).Error; err != nil { t.Fatal(err) }
	schoolID := school.ID
	user := models.User{SchoolID:&schoolID, Name:"CareerPath Student", Email:"careerpath-student@example.test", Role:"student", Active:true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{SchoolID:schoolID, UserID:user.ID, AdmissionNumber:"CP-001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	return db, schoolID, user.ID
}

func completeCareerPathResponses() map[string][]string {
	return map[string][]string{
		"interests":{"build","investigate","create"},
		"work_style":{"independent","studio"},
		"subjects":{"maths","computing","arts"},
		"goals":{"solve","make"},
	}
}

func TestCareerPathAssessmentSubmissionIsDeterministicAndIdempotent(t *testing.T) {
	db, schoolID, userID := careerPathTestDB(t)
	svc := NewCareerPathService(db)
	attempt, err := svc.CreateOrResume(schoolID,userID)
	if err != nil { t.Fatal(err) }
	saved, err := svc.SaveResponses(schoolID,userID,attempt.ID,completeCareerPathResponses())
	if err != nil { t.Fatal(err) }
	if saved.Status != "in_progress" { t.Fatalf("expected draft status, got %q",saved.Status) }
	first, err := svc.Submit(schoolID,userID,attempt.ID)
	if err != nil { t.Fatal(err) }
	if first.Status != "submitted" || len(first.Results)==0 { t.Fatalf("expected submitted results, got %#v",first) }
	second, err := svc.Submit(schoolID,userID,attempt.ID)
	if err != nil { t.Fatal(err) }
	if len(second.Results)!=len(first.Results) { t.Fatalf("repeat submission changed result count: %d != %d",len(second.Results),len(first.Results)) }
	for i:=range first.Results { if first.Results[i].Career.ID!=second.Results[i].Career.ID || first.Results[i].SharedTags!=second.Results[i].SharedTags { t.Fatal("repeat submission changed deterministic matches") } }
	if _,err:=svc.SaveResponses(schoolID,userID,attempt.ID,completeCareerPathResponses());!errors.Is(err,ErrCareerPathSubmitted){t.Fatalf("expected submitted attempt to be immutable, got %v",err)}
}

func TestCareerPathCreateOrResumeReturnsLatestSubmittedAttempt(t *testing.T) {
	db, schoolID, userID := careerPathTestDB(t)
	svc := NewCareerPathService(db)
	attempt, err := svc.CreateOrResume(schoolID, userID)
	if err != nil { t.Fatal(err) }
	if _, err := svc.SaveResponses(schoolID, userID, attempt.ID, completeCareerPathResponses()); err != nil { t.Fatal(err) }
	submitted, err := svc.Submit(schoolID, userID, attempt.ID)
	if err != nil { t.Fatal(err) }
	returned, err := svc.CreateOrResume(schoolID, userID)
	if err != nil { t.Fatal(err) }
	if returned.ID != submitted.ID { t.Fatalf("expected submitted attempt %s to be restored, got %s", submitted.ID, returned.ID) }
	if returned.Status != "submitted" { t.Fatalf("expected submitted status after reopening, got %q", returned.Status) }
	if len(returned.Results) != len(submitted.Results) || len(returned.Results) == 0 { t.Fatalf("expected saved results after reopening, got %d results", len(returned.Results)) }
	if len(returned.Responses) != len(submitted.Responses) { t.Fatalf("expected saved answers after reopening, got %d response groups", len(returned.Responses)) }
}

func TestCareerPathRejectsIncompleteAndUnknownResponses(t *testing.T) {
	db, schoolID, userID := careerPathTestDB(t)
	svc := NewCareerPathService(db)
	attempt, err := svc.CreateOrResume(schoolID,userID)
	if err != nil { t.Fatal(err) }
	if _,err:=svc.Submit(schoolID,userID,attempt.ID);!errors.Is(err,ErrCareerPathIncomplete){t.Fatalf("expected incomplete error, got %v",err)}
	if _,err:=svc.SaveResponses(schoolID,userID,attempt.ID,map[string][]string{"not-a-question":{"x"}});!errors.Is(err,ErrCareerPathInvalidResponse){t.Fatalf("expected invalid response error, got %v",err)}
}

func TestCareerPathAttemptCannotBeReadByAnotherStudent(t *testing.T) {
	db, schoolID, firstUserID := careerPathTestDB(t)
	var school models.School
	if err:=db.Where("id = ?",schoolID).First(&school).Error;err!=nil{t.Fatal(err)}
	secondUser:=models.User{SchoolID:&schoolID,Name:"Other Student",Email:"other-careerpath@example.test",Role:"student",Active:true}
	if err:=db.Create(&secondUser).Error;err!=nil{t.Fatal(err)}
	secondStudent:=models.Student{SchoolID:schoolID,UserID:secondUser.ID,AdmissionNumber:"CP-002"}
	if err:=db.Create(&secondStudent).Error;err!=nil{t.Fatal(err)}
	svc:=NewCareerPathService(db)
	attempt,err:=svc.CreateOrResume(schoolID,firstUserID);if err!=nil{t.Fatal(err)}
	if _,err:=svc.Get(schoolID,secondUser.ID,attempt.ID);!errors.Is(err,ErrCareerPathNotFound){t.Fatalf("expected other student's attempt to be hidden, got %v",err)}
	if _,err:=svc.SaveResponses(schoolID,secondUser.ID,attempt.ID,completeCareerPathResponses());!errors.Is(err,ErrCareerPathNotFound){t.Fatalf("expected other student response write to be hidden, got %v",err)}
	if _,err:=svc.Submit(schoolID,secondUser.ID,attempt.ID);!errors.Is(err,ErrCareerPathNotFound){t.Fatalf("expected other student submission to be hidden, got %v",err)}
	otherSchoolID:=uuid.New()
	if _,err:=svc.Get(otherSchoolID,firstUserID,attempt.ID);!errors.Is(err,ErrCareerPathForbidden){t.Fatalf("expected cross-school read to be forbidden, got %v",err)}
	if _,err:=svc.SaveResponses(otherSchoolID,firstUserID,attempt.ID,completeCareerPathResponses());!errors.Is(err,ErrCareerPathForbidden){t.Fatalf("expected cross-school response write to be forbidden, got %v",err)}
	if _,err:=svc.Submit(otherSchoolID,firstUserID,attempt.ID);!errors.Is(err,ErrCareerPathForbidden){t.Fatalf("expected cross-school submission to be forbidden, got %v",err)}
}

func TestCareerPathCreateOrResumeKeepsOneDraft(t *testing.T) {
	db, schoolID, userID := careerPathTestDB(t)
	svc := NewCareerPathService(db)

	first, err := svc.CreateOrResume(schoolID, userID)
	if err != nil { t.Fatal(err) }
	second, err := svc.CreateOrResume(schoolID, userID)
	if err != nil { t.Fatal(err) }
	if second.ID != first.ID {
		t.Fatalf("expected resume to return the existing draft %s, got %s", first.ID, second.ID)
	}
	var drafts int64
	if err := db.Model(&models.CareerPathAttempt{}).
		Where("school_id = ? AND user_id = ? AND version = ? AND status = ?", schoolID, userID, CareerPathAssessmentVersion, "in_progress").
		Count(&drafts).Error; err != nil {
		t.Fatal(err)
	}
	if drafts != 1 {
		t.Fatalf("expected exactly one in-progress draft, got %d", drafts)
	}
}

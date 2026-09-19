package service

import("errors";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/authz";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrAssignmentNotFound=errors.New("teacher assignment not found");ErrAssignmentDuplicate=errors.New("teacher assignment already exists");ErrAssignmentInvalid=errors.New("invalid teacher assignment"))
type TeacherAssignmentService struct{repo repository.TeacherAssignmentRepository;db *gorm.DB}
func NewTeacherAssignmentService(r repository.TeacherAssignmentRepository,db *gorm.DB)*TeacherAssignmentService{return &TeacherAssignmentService{r,db}}
func(s *TeacherAssignmentService)DB()*gorm.DB{return s.db}
func(s *TeacherAssignmentService)validate(v models.TeacherAssignment)error{
 var u models.User;if e:=s.db.First(&u,"id = ?",v.TeacherID).Error;e!=nil||u.Role!=authz.RoleTeacher||!u.Active{return ErrAssignmentInvalid}
 var subject models.Subject;if e:=s.db.First(&subject,"id = ?",v.SubjectID).Error;e!=nil||!subject.Active{return ErrAssignmentInvalid}
 var session models.AcademicSession;if e:=s.db.First(&session,"id = ?",v.AcademicSessionID).Error;e!=nil{return ErrAssignmentInvalid}
 var term models.Term;if e:=s.db.First(&term,"id = ? AND academic_session_id = ?",v.TermID,v.AcademicSessionID).Error;e!=nil{return ErrAssignmentInvalid}
 if term.StartDate.Before(session.StartDate)||term.EndDate.After(session.EndDate){return ErrAssignmentInvalid}
 var class models.SchoolClass;if e:=s.db.First(&class,"id = ?",v.ClassID).Error;e!=nil{return ErrAssignmentInvalid}
 if v.SectionID!=nil{var section models.Section;if e:=s.db.First(&section,"id = ? AND class_id = ?",*v.SectionID,v.ClassID).Error;e!=nil{return ErrAssignmentInvalid}}
 return nil
}
func(s *TeacherAssignmentService)Create(v models.TeacherAssignment)(models.TeacherAssignment,error){if e:=s.validate(v);e!=nil{return v,e};var x models.TeacherAssignment;q:=s.db.Where("teacher_id=? AND subject_id=? AND academic_session_id=? AND term_id=? AND class_id=?",v.TeacherID,v.SubjectID,v.AcademicSessionID,v.TermID,v.ClassID);if v.SectionID==nil{q=q.Where("section_id IS NULL")}else{q=q.Where("section_id = ?",*v.SectionID)};if e:=q.First(&x).Error;e==nil{return v,ErrAssignmentDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(v)}
func(s *TeacherAssignmentService)List()([]models.TeacherAssignment,error){return s.repo.List()}
func(s *TeacherAssignmentService)Get(id uuid.UUID)(models.TeacherAssignment,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrAssignmentNotFound};return v,e}
func(s *TeacherAssignmentService)Update(v models.TeacherAssignment)error{if _,e:=s.Get(v.ID);e!=nil{return e};if e:=s.validate(v);e!=nil{return e};return s.repo.Update(v)}
func(s *TeacherAssignmentService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}

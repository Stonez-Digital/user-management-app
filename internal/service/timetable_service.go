package service

import("errors";"strings";"time";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrTimetableNotFound=errors.New("timetable entry not found");ErrTimetableConflict=errors.New("timetable conflict");ErrTimetableInvalid=errors.New("invalid timetable entry"))
type TimetableService struct{repo repository.TimetableRepository;db *gorm.DB}
func NewTimetableService(r repository.TimetableRepository,db *gorm.DB)*TimetableService{return &TimetableService{repo:r,db:db}}
func(s *TimetableService)DB()*gorm.DB{return s.db}
func validClock(v string)bool{_,e:=time.Parse("15:04",v);return e==nil}
func overlaps(start,end,otherStart,otherEnd string)bool{return start<otherEnd&&otherStart<end}
func(s *TimetableService)validate(schoolID uuid.UUID,v models.TimetableEntry)error{
 if v.AcademicSessionID==uuid.Nil||v.TermID==uuid.Nil||v.TeacherAssignmentID==uuid.Nil||v.ClassID==uuid.Nil{return ErrTimetableInvalid}
 if v.DayOfWeek<models.TimetableMonday||v.DayOfWeek>models.TimetableSaturday||!validClock(v.StartTime)||!validClock(v.EndTime)||v.StartTime>=v.EndTime{return ErrTimetableInvalid}
 var term models.Term;if e:=s.db.Where("id=? AND school_id=?",v.TermID,schoolID).First(&term).Error;e!=nil||term.AcademicSessionID!=v.AcademicSessionID{return ErrTimetableInvalid}
 var assignment models.TeacherAssignment;if e:=s.db.Where("id=? AND school_id=?",v.TeacherAssignmentID,schoolID).First(&assignment).Error;e!=nil||!assignment.Active||assignment.AcademicSessionID!=v.AcademicSessionID||assignment.TermID!=v.TermID||assignment.ClassID!=v.ClassID{return ErrTimetableInvalid}
 if assignment.SectionID!=nil&&(v.SectionID==nil||*assignment.SectionID!=*v.SectionID){return ErrTimetableInvalid}
 if v.SectionID!=nil{var section models.Section;if e:=s.db.Where("id=? AND school_id=?",*v.SectionID,schoolID).First(&section).Error;e!=nil||section.ClassID!=v.ClassID{return ErrTimetableInvalid}}
 var entries []models.TimetableEntry;q:=s.db.Where("school_id=? AND academic_session_id=? AND term_id=? AND day_of_week=? AND active=true",schoolID,v.AcademicSessionID,v.TermID,v.DayOfWeek);if v.ID!=uuid.Nil{q=q.Where("id<>?",v.ID)};if e:=q.Find(&entries).Error;e!=nil{return e}
 for _,x:=range entries{if !overlaps(v.StartTime,v.EndTime,x.StartTime,x.EndTime){continue};var other models.TeacherAssignment;if e:=s.db.Where("id=? AND school_id=?",x.TeacherAssignmentID,schoolID).First(&other).Error;e!=nil{return e};if other.TeacherID==assignment.TeacherID{return ErrTimetableConflict};if x.ClassID==v.ClassID&&(x.SectionID==nil||v.SectionID==nil||*x.SectionID==*v.SectionID){return ErrTimetableConflict}}
 return nil
}
func(s *TimetableService)Create(schoolID uuid.UUID,v models.TimetableEntry)(models.TimetableEntry,error){v.SchoolID=schoolID;if e:=s.validate(schoolID,v);e!=nil{return v,e};return s.repo.Create(schoolID,v)}
func(s *TimetableService)List(schoolID,sessionID,termID uuid.UUID)([]models.TimetableEntry,error){rows,e:=s.repo.List(schoolID);if e!=nil{return nil,e};out:=make([]models.TimetableEntry,0,len(rows));for _,r:=range rows{if r.AcademicSessionID==sessionID&&r.TermID==termID{out=append(out,r)}};return out,nil}
func(s *TimetableService)ListForTeacher(schoolID,userID uuid.UUID)([]models.TimetableEntry,error){var v []models.TimetableEntry;e:=s.db.Joins("JOIN teacher_assignments ta ON ta.id=timetable_entries.teacher_assignment_id").Where("timetable_entries.school_id=? AND ta.school_id=? AND ta.teacher_id=? AND timetable_entries.active=?",schoolID,schoolID,userID,true).Order("day_of_week,start_time").Find(&v).Error;return v,e}
func(s *TimetableService)ListForStudent(schoolID,userID uuid.UUID)([]models.TimetableEntry,error){var student models.Student;if e:=s.db.Where("school_id=? AND user_id=?",schoolID,userID).First(&student).Error;e!=nil{return nil,e};var v []models.TimetableEntry;e:=s.db.Joins("JOIN student_enrollments se ON se.class_id=timetable_entries.class_id AND se.section_id=timetable_entries.section_id AND se.academic_session_id=timetable_entries.academic_session_id AND se.school_id=timetable_entries.school_id").Where("timetable_entries.school_id=? AND se.school_id=? AND se.student_id=? AND se.status=? AND timetable_entries.active=?",schoolID,schoolID,student.ID,models.EnrollmentStatusActive,true).Order("day_of_week,start_time").Find(&v).Error;return v,e}
func(s *TimetableService)Get(schoolID,id uuid.UUID)(models.TimetableEntry,error){v,e:=s.repo.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrTimetableNotFound};return v,e}
func(s *TimetableService)Update(schoolID uuid.UUID,v models.TimetableEntry)error{if _,e:=s.Get(schoolID,v.ID);e!=nil{return e};v.SchoolID=schoolID;if e:=s.validate(schoolID,v);e!=nil{return e};return s.repo.Update(schoolID,v)}
func(s *TimetableService)Delete(schoolID,id uuid.UUID)error{if _,e:=s.Get(schoolID,id);e!=nil{return e};return s.repo.Delete(schoolID,id)}
func(s *TimetableService)Normalize(v models.TimetableEntry)models.TimetableEntry{v.StartTime=strings.TrimSpace(v.StartTime);v.EndTime=strings.TrimSpace(v.EndTime);v.Room=strings.TrimSpace(v.Room);return v}

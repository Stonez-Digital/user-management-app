package service

import ("errors"; "strings"; "time"; "github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "github.com/onoja217/users-management-app/internal/repository"; "gorm.io/gorm")

var(ErrTimetableNotFound=errors.New("timetable entry not found");ErrTimetableConflict=errors.New("timetable conflict");ErrTimetableInvalid=errors.New("invalid timetable entry"))

type TimetableService struct{repo repository.TimetableRepository;db *gorm.DB}
func NewTimetableService(r repository.TimetableRepository,db *gorm.DB)*TimetableService{return &TimetableService{repo:r,db:db}}
func(s *TimetableService)DB()*gorm.DB{return s.db}
func validClock(v string)bool{_,e:=time.Parse("15:04",v);return e==nil}
func overlaps(start,end,otherStart,otherEnd string)bool{return start<otherEnd&&otherStart<end}
func(s *TimetableService)validate(v models.TimetableEntry)error{
 if v.AcademicSessionID==uuid.Nil||v.TermID==uuid.Nil||v.TeacherAssignmentID==uuid.Nil||v.ClassID==uuid.Nil{return ErrTimetableInvalid}
 if v.DayOfWeek<models.TimetableMonday||v.DayOfWeek>models.TimetableSaturday||!validClock(v.StartTime)||!validClock(v.EndTime)||v.StartTime>=v.EndTime{return ErrTimetableInvalid}
 var term models.Term;if e:=s.db.First(&term,"id=?",v.TermID).Error;e!=nil||term.AcademicSessionID!=v.AcademicSessionID{return ErrTimetableInvalid}
 var assignment models.TeacherAssignment;if e:=s.db.First(&assignment,"id=?",v.TeacherAssignmentID).Error;e!=nil||!assignment.Active||assignment.AcademicSessionID!=v.AcademicSessionID||assignment.TermID!=v.TermID||assignment.ClassID!=v.ClassID{return ErrTimetableInvalid}
 if assignment.SectionID!=nil&&(v.SectionID==nil||*assignment.SectionID!=*v.SectionID){return ErrTimetableInvalid}
 if v.SectionID!=nil{var section models.Section;if e:=s.db.First(&section,"id=?",*v.SectionID).Error;e!=nil||section.ClassID!=v.ClassID{return ErrTimetableInvalid}}
 var entries []models.TimetableEntry;q:=s.db.Where("academic_session_id=? AND term_id=? AND day_of_week=? AND active=true",v.AcademicSessionID,v.TermID,v.DayOfWeek);if v.ID!=uuid.Nil{q=q.Where("id<>?",v.ID)};if e:=q.Find(&entries).Error;e!=nil{return e}
 for _,x:=range entries{if !overlaps(v.StartTime,v.EndTime,x.StartTime,x.EndTime){continue};if x.TeacherAssignmentID==v.TeacherAssignmentID{return ErrTimetableConflict};if x.ClassID==v.ClassID&&(x.SectionID==nil||v.SectionID==nil||*x.SectionID==*v.SectionID){return ErrTimetableConflict}}
 return nil
}
func(s *TimetableService)Create(v models.TimetableEntry)(models.TimetableEntry,error){if e:=s.validate(v);e!=nil{return v,e};return s.repo.Create(v)}
func(s *TimetableService)List()([]models.TimetableEntry,error){return s.repo.List()}
func(s *TimetableService)Get(id uuid.UUID)(models.TimetableEntry,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrTimetableNotFound};return v,e}
func(s *TimetableService)Update(v models.TimetableEntry)error{if _,e:=s.Get(v.ID);e!=nil{return e};if e:=s.validate(v);e!=nil{return e};return s.repo.Update(v)}
func(s *TimetableService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}
func(s *TimetableService)Normalize(v models.TimetableEntry)models.TimetableEntry{v.StartTime=strings.TrimSpace(v.StartTime);v.EndTime=strings.TrimSpace(v.EndTime);v.Room=strings.TrimSpace(v.Room);return v}

package service
import("errors";"strings";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrAttendanceNotFound=errors.New("attendance not found");ErrAttendanceDuplicate=errors.New("attendance already recorded for this date");ErrAttendanceInvalid=errors.New("invalid attendance"))
type AttendanceService struct{repo repository.AttendanceRepository;db *gorm.DB}
func NewAttendanceService(r repository.AttendanceRepository,db *gorm.DB)*AttendanceService{return &AttendanceService{r,db}}
func(s *AttendanceService)DB()*gorm.DB{return s.db}
func(s *AttendanceService)validate(v models.Attendance)error{var e models.StudentEnrollment;if x:=s.db.First(&e,"id = ?",v.EnrollmentID).Error;x!=nil{return ErrAttendanceInvalid};var valid=map[string]bool{models.AttendancePresent:true,models.AttendanceAbsent:true,models.AttendanceLate:true,models.AttendanceExcused:true};if !valid[strings.ToLower(v.Status)]||v.Date.IsZero(){return ErrAttendanceInvalid};return nil}
func(s *AttendanceService)Create(v models.Attendance)(models.Attendance,error){v.Status=strings.ToLower(strings.TrimSpace(v.Status));if e:=s.validate(v);e!=nil{return v,e};var x models.Attendance;if e:=s.db.Where("enrollment_id=? AND date=?",v.EnrollmentID,v.Date).First(&x).Error;e==nil{return v,ErrAttendanceDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(v)}
func(s *AttendanceService)List()([]models.Attendance,error){return s.repo.List()}
func(s *AttendanceService)Get(id uuid.UUID)(models.Attendance,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrAttendanceNotFound};return v,e}
func(s *AttendanceService)Update(v models.Attendance)error{if _,e:=s.Get(v.ID);e!=nil{return e};v.Status=strings.ToLower(strings.TrimSpace(v.Status));if e:=s.validate(v);e!=nil{return e};return s.repo.Update(v)}
func(s *AttendanceService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}
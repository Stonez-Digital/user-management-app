package repository
import("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type AttendanceRepository interface{Create(models.Attendance)(models.Attendance,error);List()([]models.Attendance,error);Get(uuid.UUID)(models.Attendance,error);Update(models.Attendance)error;Delete(uuid.UUID)error}
type attendanceRepo struct{db *gorm.DB}
func NewAttendanceRepository(db *gorm.DB)AttendanceRepository{return &attendanceRepo{db}}
func(r *attendanceRepo)Create(v models.Attendance)(models.Attendance,error){return v,r.db.Create(&v).Error}
func(r *attendanceRepo)List()([]models.Attendance,error){var v []models.Attendance;e:=r.db.Order("date DESC").Find(&v).Error;return v,e}
func(r *attendanceRepo)Get(id uuid.UUID)(models.Attendance,error){var v models.Attendance;e:=r.db.First(&v,"id = ?",id).Error;return v,e}
func(r *attendanceRepo)Update(v models.Attendance)error{return r.db.Save(&v).Error}
func(r *attendanceRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.Attendance{},"id = ?",id).Error}
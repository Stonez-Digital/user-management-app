package service

import ("errors";"strings";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")

var(ErrClassNotFound=errors.New("class not found");ErrSectionNotFound=errors.New("section not found");ErrClassDuplicate=errors.New("class already exists");ErrSectionDuplicate=errors.New("section already exists");ErrClassInvalidLevel=errors.New("class level must be zero or greater"))

type ClassService struct{classes repository.ClassRepository;sections repository.SectionRepository;db *gorm.DB}
func NewClassService(c repository.ClassRepository,s repository.SectionRepository,db *gorm.DB)*ClassService{return &ClassService{c,s,db}}
func(s *ClassService)DB()*gorm.DB{return s.db}
func(s *ClassService)CreateClass(v models.SchoolClass)(models.SchoolClass,error){v.Name=strings.TrimSpace(v.Name);if v.Name==""||v.Level<0{return v,ErrClassInvalidLevel};var x models.SchoolClass;if e:=s.db.Where("name = ?",v.Name).First(&x).Error;e==nil{return v,ErrClassDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.classes.Create(v)}
func(s *ClassService)ListClasses()([]models.SchoolClass,error){return s.classes.List()}
func(s *ClassService)GetClass(id uuid.UUID)(models.SchoolClass,error){v,e:=s.classes.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrClassNotFound};return v,e}
func(s *ClassService)UpdateClass(v models.SchoolClass)error{old,e:=s.GetClass(v.ID);if e!=nil{return e};v.Name=strings.TrimSpace(v.Name);if v.Name==""||v.Level<0{return ErrClassInvalidLevel};if v.Name!=old.Name{var x models.SchoolClass;if e=s.db.Where("name = ? AND id <> ?",v.Name,v.ID).First(&x).Error;e==nil{return ErrClassDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return e}};return s.classes.Update(v)}
func(s *ClassService)DeleteClass(id uuid.UUID)error{if _,e:=s.GetClass(id);e!=nil{return e};return s.classes.Delete(id)}
func(s *ClassService)CreateSection(v models.Section)(models.Section,error){if _,e:=s.GetClass(v.ClassID);e!=nil{return v,ErrClassNotFound};v.Name=strings.TrimSpace(v.Name);if v.Name==""{return v,ErrSectionDuplicate};var x models.Section;if e:=s.db.Where("class_id = ? AND name = ?",v.ClassID,v.Name).First(&x).Error;e==nil{return v,ErrSectionDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.sections.Create(v)}
func(s *ClassService)ListSections(id uuid.UUID)([]models.Section,error){if _,e:=s.GetClass(id);e!=nil{return nil,e};return s.sections.List(id)}
func(s *ClassService)GetSection(id uuid.UUID)(models.Section,error){v,e:=s.sections.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrSectionNotFound};return v,e}
func(s *ClassService)UpdateSection(v models.Section)error{old,e:=s.GetSection(v.ID);if e!=nil{return e};if _,e=s.GetClass(v.ClassID);e!=nil{return ErrClassNotFound};v.Name=strings.TrimSpace(v.Name);if v.Name==""{return ErrSectionDuplicate};if v.ClassID!=old.ClassID{return ErrSectionDuplicate};var x models.Section;if e=s.db.Where("class_id = ? AND name = ? AND id <> ?",v.ClassID,v.Name,v.ID).First(&x).Error;e==nil{return ErrSectionDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.sections.Update(v)}
func(s *ClassService)DeleteSection(id uuid.UUID)error{if _,e:=s.GetSection(id);e!=nil{return e};return s.sections.Delete(id)}

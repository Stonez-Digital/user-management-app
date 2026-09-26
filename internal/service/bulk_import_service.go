package service

import (
 "archive/zip"
 "bytes"
 "encoding/csv"
 "encoding/json"
 "encoding/xml"
 "errors"
 "fmt"
 "io"
 "mime/multipart"
 "net/url"
 "strconv"
 "strings"
 "sync"
 "time"
 "unicode"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/auth"
 "github.com/onoja217/users-management-app/internal/authz"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)

const maxBulkImportBytes int64 = 15 << 20

type BulkRow map[string]string
type BulkError struct { Row int `json:"row"`; Field string `json:"field,omitempty"`; Message string `json:"message"` }
type BulkImportPreview struct { Job models.BulkImportJob `json:"job"`; Preview []BulkRow `json:"preview"`; Errors []BulkError `json:"errors"` }

type BulkImportService struct { db *gorm.DB; mu sync.Mutex; mailer *CredentialMailer }
func NewBulkImportService(db *gorm.DB, mailer *CredentialMailer)*BulkImportService{return &BulkImportService{db:db,mailer:mailer}}
func(s *BulkImportService)DB()*gorm.DB{return s.db}

func parseUpload(fh *multipart.FileHeader)([]BulkRow,error){
 if fh.Size>maxBulkImportBytes{return nil,errors.New("file exceeds 15 MB limit")}
 f,e:=fh.Open();if e!=nil{return nil,e};defer f.Close()
 name:=strings.ToLower(fh.Filename)
 if strings.HasSuffix(name,".csv"){return parseCSV(f)}
 if strings.HasSuffix(name,".xlsx"){return parseXLSX(f)}
 return nil,errors.New("only CSV and XLSX files are supported")
}
func parseCSV(r io.Reader)([]BulkRow,error){
 cr:=csv.NewReader(r);cr.FieldsPerRecord=-1
 headers,e:=cr.Read();if e!=nil{return nil,errors.New("file is empty")}
 normalizeHeaders(headers)
 var out []BulkRow
 for i:=1;;i++{rec,e:=cr.Read();if errors.Is(e,io.EOF){break};if e!=nil{return nil,fmt.Errorf("row %d: %w",i,e)};row:=BulkRow{};for j,h:=range headers{if j<len(rec){row[h]=strings.TrimSpace(rec[j])}};out=append(out,row)}
 return out,nil
}
func normalizeHeaders(h []string){for i:=range h{h[i]=normalizeHeader(h[i])}}
func normalizeHeader(s string)string{s=strings.ToLower(strings.TrimSpace(s));s=strings.ReplaceAll(s," ","_");s=strings.ReplaceAll(s,"-","_");return s}

type xCell struct{Ref string `xml:"r,attr"`;Type string `xml:"t,attr"`;Value string `xml:"v"`;Inline string `xml:"is>t"`}
type xRow struct{Cells []xCell `xml:"c"`}
type xSheet struct{Rows []xRow `xml:"sheetData>row"`}
func parseXLSX(r io.Reader)([]BulkRow,error){
 b,e:=io.ReadAll(io.LimitReader(r,maxBulkImportBytes+1));if e!=nil{return nil,e};if int64(len(b))>maxBulkImportBytes{return nil,errors.New("file exceeds 15 MB limit")}
 z,e:=zip.NewReader(bytes.NewReader(b),int64(len(b)));if e!=nil{return nil,errors.New("invalid XLSX file")}
 var shared []string;var sheet []byte
 for _,f:=range z.File{if f.Name=="xl/sharedStrings.xml"{q,_:=f.Open();var ss struct{Items []struct{Text []string `xml:"t"`} `xml:"si"`};if xml.NewDecoder(q).Decode(&ss)==nil{for _,v:=range ss.Items{shared=append(shared,strings.Join(v.Text,""))}};q.Close()};if f.Name=="xl/worksheets/sheet1.xml"{q,_:=f.Open();sheet,_=io.ReadAll(q);q.Close()}}
 if len(sheet)==0{return nil,errors.New("XLSX sheet1.xml not found")}
 var xs xSheet;if e=xml.Unmarshal(sheet,&xs);e!=nil{return nil,errors.New("invalid XLSX worksheet")}
 var rows [][]string
 for _,rr:=range xs.Rows{max:=0;for _,c:=range rr.Cells{n:=colNumber(c.Ref);if n>max{max=n}};vals:=make([]string,max);for _,c:=range rr.Cells{n:=colNumber(c.Ref);v:=c.Value;if c.Type=="s"{i,_:=strconv.Atoi(v);if i>=0&&i<len(shared){v=shared[i]}};if c.Type=="inlineStr"{v=c.Inline};vals[n-1]=strings.TrimSpace(v)};rows=append(rows,vals)}
 if len(rows)==0{return nil,errors.New("worksheet is empty")};headers:=rows[0];normalizeHeaders(headers);out:=make([]BulkRow,0,len(rows)-1);for _,rec:=range rows[1:]{row:=BulkRow{};for j,h:=range headers{if j<len(rec){row[h]=rec[j]}};out=append(out,row)};return out,nil
}
func colNumber(ref string)int{n:=0;for _,r:=range ref{if r>='A'&&r<='Z'{n=n*26+int(r-'A')+1}else if r>='a'&&r<='z'{n=n*26+int(unicode.ToUpper(r)-'A')+1}else{break}};return n}

func(s *BulkImportService)Validate(schoolID,actor uuid.UUID,kind string,rows []BulkRow)([]BulkError,int){
 errs:=[]BulkError{};valid:=0;seen:=map[string]bool{}
 if kind!="students"&&kind!="teachers"&&kind!="parents"{errs=append(errs,BulkError{Message:"kind must be students, teachers or parents"});return errs,0}
 for i,row:=range rows{rn:=i+2;name:=strings.TrimSpace(row["name"]);if name==""{name=strings.TrimSpace(strings.Join([]string{row["first_name"],row["last_name"]}," "))};email:=strings.ToLower(strings.TrimSpace(row["email"]));if name==""{errs=append(errs,BulkError{rn,"name","name or first_name/last_name is required"})};if email==""||!strings.Contains(email,"@"){errs=append(errs,BulkError{rn,"email","valid email is required"})};key:=email
 if kind=="students"{ad:=strings.TrimSpace(row["admission_number"]);if ad==""{errs=append(errs,BulkError{rn,"admission_number","admission number is required"})};key=ad}
 if kind=="teachers"{sid:=strings.TrimSpace(row["staff_id"]);if sid==""{errs=append(errs,BulkError{rn,"staff_id","staff ID is required"})};key=sid}
 if kind=="parents"{if strings.TrimSpace(row["parent_identifier"])==""&&email==""&&strings.TrimSpace(row["phone"])==""{errs=append(errs,BulkError{rn,"parent_identifier","parent identifier, email or phone is required"})}}
 if seen[strings.ToLower(key)]{errs=append(errs,BulkError{rn,"","duplicate row in uploaded file"})}else{seen[strings.ToLower(key)]=true}
 if len(errs)==0||errs[len(errs)-1].Row!=rn{valid++}
 }
 return errs,valid
}

func(s *BulkImportService)Create(schoolID,actor uuid.UUID,kind,filename string,sendCredentials bool,rows []BulkRow)(BulkImportPreview,error){
 errs,valid:=s.Validate(schoolID,actor,kind,rows);if len(rows)==0{return BulkImportPreview{},errors.New("file contains no data rows")}
 rb,_:=json.Marshal(rows);eb,_:=json.Marshal(errs);j:=models.BulkImportJob{SchoolID:schoolID,InitiatedBy:actor,Kind:kind,FileName:filename,Status:models.BulkImportPending,Total:len(rows),Valid:valid,SendCredentials:sendCredentials,RowsJSON:string(rb),ErrorsJSON:string(eb)};if e:=s.db.Create(&j).Error;e!=nil{return BulkImportPreview{},e}
 preview:=rows;if len(preview)>10{preview=preview[:10]};return BulkImportPreview{Job:j,Preview:preview,Errors:errs},nil
}
func(s *BulkImportService)ResumePending(){var jobs []models.BulkImportJob;if s.db.Where("status IN ?",[]string{models.BulkImportQueued,models.BulkImportRunning}).Find(&jobs).Error!=nil{return};for _,j:=range jobs{j.Status=models.BulkImportQueued;s.db.Model(&j).Update("status",j.Status);go s.run(j)}}

func(s *BulkImportService)Start(schoolID,actor,jobID uuid.UUID)error{var j models.BulkImportJob;if e:=s.db.Where("id=? AND school_id=? AND initiated_by=?",jobID,schoolID,actor).First(&j).Error;e!=nil{return e};if j.Status!=models.BulkImportPending{return errors.New("import job is not pending")};if j.SendCredentials&&!s.mailer.Enabled(){return errors.New("credential email is not configured on the server")};j.Status=models.BulkImportQueued;if e:=s.db.Save(&j).Error;e!=nil{return e};go s.run(j);return nil}
func(s *BulkImportService)run(j models.BulkImportJob){s.mu.Lock();defer s.mu.Unlock();var rows []BulkRow;if json.Unmarshal([]byte(j.RowsJSON),&rows)!=nil{s.fail(j.ID,"invalid stored import payload");return};j.Status=models.BulkImportRunning;s.db.Model(&j).Updates(map[string]any{"status":j.Status})
 var errs []BulkError;_ = json.Unmarshal([]byte(j.ErrorsJSON),&errs)
 validSet:=map[int]bool{};for _,e:=range errs{validSet[e.Row]=true}
 for i,row:=range rows{rn:=i+2;if validSet[rn]{continue};created,skipped,email,password,role,e:=s.processRow(j,row);if skipped{j.Skipped++};if e!=nil{errs=append(errs,BulkError{rn,"",e.Error()});j.Failed++}else{j.Created++;if j.SendCredentials&&created{if e=s.mailer.SendCredentials(strings.TrimSpace(row["name"]),email,role,password);e!=nil{errs=append(errs,BulkError{rn,"email","account created but credential email failed: "+e.Error()});j.CredentialEmailsFailed++}else{j.CredentialsSent++}}};s.db.Model(&j).Updates(map[string]any{"created":j.Created,"skipped":j.Skipped,"failed":j.Failed,"credentials_sent":j.CredentialsSent,"credential_emails_failed":j.CredentialEmailsFailed,"errors_json":mustJSON(errs)})}
 j.ErrorsJSON=mustJSON(errs);if j.Failed>0||len(errs)>0{j.Status=models.BulkImportCompletedWithErrors}else{j.Status=models.BulkImportCompleted};s.db.Model(&j).Updates(map[string]any{"status":j.Status,"errors_json":j.ErrorsJSON,"skipped":j.Skipped,"failed":j.Failed,"credentials_sent":j.CredentialsSent,"credential_emails_failed":j.CredentialEmailsFailed})}
func(s *BulkImportService)fail(id uuid.UUID,msg string){s.db.Model(&models.BulkImportJob{}).Where("id=?",id).Updates(map[string]any{"status":models.BulkImportFailed,"errors_json":mustJSON([]BulkError{{Message:msg}})})}
func mustJSON(v any)string{b,_:=json.Marshal(v);return string(b)}

func(s *BulkImportService)processRow(j models.BulkImportJob,row BulkRow)(bool,bool,string,string,string,error){
 name:=strings.TrimSpace(row["name"]);if name==""{name=strings.TrimSpace(strings.Join([]string{row["first_name"],row["last_name"]}," "))}
 email:=strings.ToLower(strings.TrimSpace(row["email"]))
 if j.Kind=="students"{return s.processStudent(j,name,email,row)}
 if j.Kind=="parents"{return s.processParent(j,name,email,row)}
 return s.processTeacher(j,name,email,row)
}
func(s *BulkImportService)createUser(tx *gorm.DB,schoolID uuid.UUID,name,email,role string)(models.User,string,bool,error){
 if email==""{return models.User{},"",false,errors.New("email is required for credential delivery")}
 var u models.User
 if e:=tx.Where("LOWER(email)=?",email).First(&u).Error;e==nil {
  if u.SchoolID==nil || *u.SchoolID!=schoolID{return u,"",false,fmt.Errorf("email %q already belongs to another tenant",email)}
  return u,"",false,nil
 } else if !errors.Is(e,gorm.ErrRecordNotFound){return u,"",false,e}
 password,e:=GenerateTemporaryPassword();if e!=nil{return u,"",false,e}
 hash,e:=auth.HashPassword(password);if e!=nil{return u,"",false,e}
 u=models.User{Name:name,Email:email,PasswordHash:hash,Role:role,Active:true,SchoolID:&schoolID}
 if e=tx.Create(&u).Error;e!=nil{return u,"",false,e}
 return u,password,true,nil
}
func(s *BulkImportService)processStudent(j models.BulkImportJob,name,email string,row BulkRow)(bool,bool,string,string,string,error){var created bool;var skipped bool;var password string;userEmail:=email;var role=authz.RoleStudent;e:=s.db.Transaction(func(tx *gorm.DB)error{var st models.Student;ad:=strings.TrimSpace(row["admission_number"]);if e:=tx.Where("school_id=? AND admission_number=?",j.SchoolID,ad).First(&st).Error;e==nil{skipped=true;return nil}else if !errors.Is(e,gorm.ErrRecordNotFound){return e};u,p,wasCreated,e:=s.createUser(tx,j.SchoolID,name,email,authz.RoleStudent);password=p;created=wasCreated;userEmail=u.Email;if e!=nil{return e};st=models.Student{SchoolID:j.SchoolID,UserID:u.ID,AdmissionNumber:ad,EnrollmentStatus:models.EnrollmentActive,GuardianName:row["guardian_name"],GuardianPhone:row["guardian_phone"],GuardianEmail:row["guardian_email"]};if v:=row["gender"];v!=""{st.Gender=v};if e=tx.Create(&st).Error;e!=nil{return e};return s.maybeEnroll(tx,j.SchoolID,st.ID,row)});return created,skipped,userEmail,password,role,e}
func(s *BulkImportService)maybeEnroll(tx *gorm.DB,schoolID,studentID uuid.UUID,row BulkRow)error{sn:=strings.TrimSpace(row["session"]);cn:=strings.TrimSpace(row["class"]);sec:=strings.TrimSpace(row["section"]);if sn==""&&cn==""&&sec==""{return nil};var ses models.AcademicSession;if e:=tx.Where("school_id=? AND name=?",schoolID,sn).First(&ses).Error;e!=nil{return fmt.Errorf("academic session %q not found",sn)};if ses.Status==models.AcademicStatusClosed||ses.Status==models.AcademicStatusArchived{return fmt.Errorf("academic session %q is closed or archived",sn)};var cl models.SchoolClass;if e:=tx.Where("school_id=? AND lower(name)=lower(?)",schoolID,cn).First(&cl).Error;e!=nil{return fmt.Errorf("class %q not found",cn)};var se models.Section;if e:=tx.Where("school_id=? AND class_id=? AND name=?",schoolID,cl.ID,sec).First(&se).Error;e!=nil{return fmt.Errorf("section %q not found",sec)};var exists models.StudentEnrollment;if e:=tx.Where("school_id=? AND student_id=? AND academic_session_id=?",schoolID,studentID,ses.ID).First(&exists).Error;e==nil{return nil};var active models.StudentEnrollment;if e:=tx.Where("school_id=? AND student_id=? AND status=? AND academic_session_id<>?",schoolID,studentID,models.EnrollmentStatusActive,ses.ID).First(&active).Error;e==nil{return fmt.Errorf("student already has an active enrollment in another academic session")};return tx.Create(&models.StudentEnrollment{SchoolID:schoolID,StudentID:studentID,AcademicSessionID:ses.ID,ClassID:cl.ID,SectionID:se.ID,Status:models.EnrollmentStatusActive,EnrolledAt:time.Now().UTC()}).Error}
func(s *BulkImportService)processTeacher(j models.BulkImportJob,name,email string,row BulkRow)(bool,bool,string,string,string,error){var created bool;var password string;userEmail:=email;var role=authz.RoleTeacher;e:=s.db.Transaction(func(tx *gorm.DB)error{staff:=strings.TrimSpace(row["staff_id"]);var p models.TeacherProfile;if e:=tx.Where("school_id=? AND staff_id=?",j.SchoolID,staff).First(&p).Error;e==nil{return nil}else if !errors.Is(e,gorm.ErrRecordNotFound){return e};u,pw,wasCreated,e:=s.createUser(tx,j.SchoolID,name,email,authz.RoleTeacher);password=pw;created=wasCreated;userEmail=u.Email;if e!=nil{return e};p=models.TeacherProfile{SchoolID:j.SchoolID,UserID:u.ID,StaffID:staff,Phone:row["phone"],Gender:row["gender"],Department:row["department"],Designation:row["designation"],Subjects:row["subjects"],Classes:row["classes"],Status:row["status"]};if e=tx.Create(&p).Error;e!=nil{return e};return nil});return created,false,userEmail,password,role,e}
func(s *BulkImportService)processParent(j models.BulkImportJob,name,email string,row BulkRow)(bool,bool,string,string,string,error){var created bool;var password string;userEmail:=email;var role=authz.RoleParent;e:=s.db.Transaction(func(tx *gorm.DB)error{identifier:=strings.ToLower(strings.TrimSpace(row["parent_identifier"]));var u models.User;if identifier!=""{var pp models.ParentProfile;if e:=tx.Where("school_id=? AND parent_identifier=?",j.SchoolID,identifier).First(&pp).Error;e==nil{_ = tx.Where("id=?",pp.UserID).First(&u).Error}};if u.ID==uuid.Nil&&email!=""{_ = tx.Where("school_id=? AND LOWER(email)=?",j.SchoolID,email).First(&u).Error};if u.ID==uuid.Nil{var e error;u,password,created,e=s.createUser(tx,j.SchoolID,name,email,authz.RoleParent);userEmail=u.Email;if e!=nil{return e};if e=tx.Create(&models.ParentProfile{SchoolID:j.SchoolID,UserID:u.ID,ParentIdentifier:identifier,Phone:row["phone"],Address:row["address"],Occupation:row["occupation"]}).Error;e!=nil{return e}};ad:=strings.TrimSpace(row["student_admission_number"]);if ad==""{return nil};var st models.Student;if e:=tx.Where("school_id=? AND admission_number=?",j.SchoolID,ad).First(&st).Error;e!=nil{return fmt.Errorf("student %q not found",ad)};var link models.GuardianRelationship;if e:=tx.Where("school_id=? AND guardian_user_id=? AND student_id=?",j.SchoolID,u.ID,st.ID).First(&link).Error;e==nil{return nil};return tx.Create(&models.GuardianRelationship{SchoolID:j.SchoolID,GuardianUserID:u.ID,StudentID:st.ID,Relationship:strings.TrimSpace(row["relationship"]),Primary:strings.EqualFold(row["primary"],"true")||strings.EqualFold(row["primary"],"yes"),Active:true}).Error});return created,false,userEmail,password,role,e}
func(s *BulkImportService)List(schoolID uuid.UUID)([]models.BulkImportJob,error){var out []models.BulkImportJob;e:=s.db.Where("school_id=?",schoolID).Order("created_at DESC").Limit(100).Find(&out).Error;return out,e}
func(s *BulkImportService)Get(schoolID,jobID uuid.UUID)(models.BulkImportJob,error){var j models.BulkImportJob;e:=s.db.Where("id=? AND school_id=?",jobID,schoolID).First(&j).Error;return j,e}
func parseURLFilename(name string)string{if u,e:=url.QueryUnescape(name);e==nil{name=u};return strings.TrimSpace(name)}

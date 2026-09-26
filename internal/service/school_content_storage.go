package service

import (
 "bytes"
 "fmt"
 "io"
 "mime"
 "net/http"
 "os"
 "path"
 "strings"
 "time"
 "github.com/google/uuid"
)

const schoolMediaBucket="school-media"

type SchoolMediaStorage struct { baseURL,secretKey string; client *http.Client }

func NewSchoolMediaStorageFromEnv()*SchoolMediaStorage{
 return &SchoolMediaStorage{baseURL:strings.TrimRight(os.Getenv("SUPABASE_URL"),"/"),secretKey:strings.TrimSpace(os.Getenv("SUPABASE_SECRET_KEY")),client:&http.Client{Timeout:30*time.Second}}
}

func(s *SchoolMediaStorage) Upload(schoolID uuid.UUID, area, filename, contentType string, body io.Reader, size int64)(string,error){
 if s.baseURL==""||s.secretKey=="" {return "",fmt.Errorf("school media storage is not configured")}
 if size<=0||size>10*1024*1024{return "",fmt.Errorf("image must be between 1 byte and 10 MB")}
 ext:=strings.ToLower(path.Ext(filename)); if ext=="" {ext=extensionForContentMime(contentType)}
 allowed:=map[string]string{".png":"image/png",".jpg":"image/jpeg",".jpeg":"image/jpeg",".webp":"image/webp"}
 expected,ok:=allowed[ext];if !ok||expected!=contentType{return "",fmt.Errorf("image must be PNG, JPEG, or WebP")}
 data,err:=io.ReadAll(io.LimitReader(body,10*1024*1024+1));if err!=nil{return "",fmt.Errorf("failed to read image: %w",err)}
 if int64(len(data))>10*1024*1024{return "",fmt.Errorf("image must not exceed 10 MB")}
 if http.DetectContentType(data)!=contentType{return "",fmt.Errorf("uploaded file content does not match its declared image type")}
 if err=s.ensureBucket();err!=nil{return "",err}
 objectPath:=schoolID.String()+"/"+strings.Trim(area,"/")+"/"+uuid.NewString()+ext
 req,err:=http.NewRequest(http.MethodPost,s.baseURL+"/storage/v1/object/"+schoolMediaBucket+"/"+objectPath,bytes.NewReader(data));if err!=nil{return "",err}
 req.Header.Set("Authorization","Bearer "+s.secretKey);req.Header.Set("apikey",s.secretKey);req.Header.Set("Content-Type",contentType);req.Header.Set("x-upsert","false")
 resp,err:=s.client.Do(req);if err!=nil{return "",fmt.Errorf("image upload failed: %w",err)};defer resp.Body.Close()
 if resp.StatusCode<200||resp.StatusCode>=300{detail,_:=io.ReadAll(io.LimitReader(resp.Body,4096));return "",fmt.Errorf("image upload failed: %s",strings.TrimSpace(string(detail)))}
 return s.baseURL+"/storage/v1/object/public/"+schoolMediaBucket+"/"+objectPath,nil
}
func(s *SchoolMediaStorage) ensureBucket()error{
 payload:=[]byte(`{"id":"school-media","name":"school-media","public":true,"file_size_limit":10485760,"allowed_mime_types":["image/png","image/jpeg","image/webp"]}`)
 req,err:=http.NewRequest(http.MethodPost,s.baseURL+"/storage/v1/bucket",bytes.NewReader(payload));if err!=nil{return err}
 req.Header.Set("Authorization","Bearer "+s.secretKey);req.Header.Set("apikey",s.secretKey);req.Header.Set("Content-Type","application/json")
 resp,err:=s.client.Do(req);if err!=nil{return fmt.Errorf("school media storage setup failed: %w",err)};defer resp.Body.Close()
 if resp.StatusCode==http.StatusConflict{return nil};if resp.StatusCode<200||resp.StatusCode>=300{detail,_:=io.ReadAll(io.LimitReader(resp.Body,4096));return fmt.Errorf("school media storage setup failed: %s",strings.TrimSpace(string(detail)))}
 return nil
}
func extensionForContentMime(contentType string)string{exts,_:=mime.ExtensionsByType(contentType);if len(exts)>0{return exts[0]};return ""}

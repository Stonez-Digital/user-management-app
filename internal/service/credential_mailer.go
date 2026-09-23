package service

import (
 "crypto/rand"
 "crypto/tls"
 "fmt"
 "net"
 "net/smtp"
 "os"
 "strconv"
 "strings"
)

type CredentialMailer struct {
 host string
 port int
 username string
 password string
 from string
 fromName string
 loginURL string
}

func NewCredentialMailerFromEnv() *CredentialMailer {
 port:=587
 if v:=os.Getenv("SMTP_PORT");v!="" { if n,e:=strconv.Atoi(v);e==nil {port=n} }
 from:=strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
 if from=="" {from=strings.TrimSpace(os.Getenv("SMTP_USERNAME"))}
 return &CredentialMailer{
  host:strings.TrimSpace(os.Getenv("SMTP_HOST")),port:port,
  username:strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
  password:os.Getenv("SMTP_PASSWORD"),
  from:from,fromName:strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")),
  loginURL:strings.TrimRight(strings.TrimSpace(os.Getenv("APP_LOGIN_URL")),"/"),
 }
}
func(m *CredentialMailer) Enabled() bool { return m!=nil && m.host!="" && m.from!="" && m.loginURL!="" }
func(m *CredentialMailer) SendCredentials(name,email,role,password string) error {
 if !m.Enabled(){return fmt.Errorf("credential email is not configured")}
 if strings.TrimSpace(email)=="" {return fmt.Errorf("recipient email is missing")}
 subject:="Your Stonez Digital school portal login"
 body:=fmt.Sprintf("Hello %s,\n\nYour school portal account has been created.\n\nRole: %s\nLogin: %s\nTemporary password: %s\n\nSign in: %s\n\nFor security, change your password after signing in and do not share these credentials.\n\nStonez Digital",name,role,email,password,m.loginURL)
 msg:=[]byte("From: "+m.fromName+" <"+m.from+">\r\nTo: "+email+"\r\nSubject: "+subject+"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"+body+"\r\n")
 addr:=net.JoinHostPort(m.host,strconv.Itoa(m.port))
 if m.port==465 {
  conn,e:=tls.Dial("tcp",addr,&tls.Config{ServerName:m.host,MinVersion:tls.VersionTLS12});if e!=nil{return e}
  client,e:=smtp.NewClient(conn,m.host);if e!=nil{return e};defer client.Close()
  if m.username!="" {if e=client.Auth(smtp.PlainAuth("",m.username,m.password,m.host));e!=nil{return e}}
  if e=client.Mail(m.from);e!=nil{return e};if e=client.Rcpt(email);e!=nil{return e}
  w,e:=client.Data();if e!=nil{return e};if _,e=w.Write(msg);e!=nil{w.Close();return e};return w.Close()
 }
 auth:=smtp.PlainAuth("",m.username,m.password,m.host)
 return smtp.SendMail(addr,auth,m.from,[]string{email},msg)
}

func GenerateTemporaryPassword() (string,error) {
 const alphabet="ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%*"
 b:=make([]byte,18);if _,e:=rand.Read(b);e!=nil{return "",e}
 out:=make([]byte,len(b));for i,v:=range b {out[i]=alphabet[int(v)%len(alphabet)]}
 return string(out),nil
}

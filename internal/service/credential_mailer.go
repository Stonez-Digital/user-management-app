package service

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"net/url"
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
	passwordResetURL string
}

func NewCredentialMailerFromEnv() *CredentialMailer {
	port := 587
	if v := os.Getenv("SMTP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil { port = n }
	}
	from := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	if from == "" { from = strings.TrimSpace(os.Getenv("SMTP_USERNAME")) }
	return &CredentialMailer{
		host: strings.TrimSpace(os.Getenv("SMTP_HOST")), port: port,
		username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")), password: os.Getenv("SMTP_PASSWORD"),
		from: from, fromName: strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")),
		loginURL: strings.TrimRight(strings.TrimSpace(os.Getenv("APP_LOGIN_URL")), "/"),
		passwordResetURL: strings.TrimSpace(os.Getenv("APP_PASSWORD_RESET_URL")),
	}
}

func (m *CredentialMailer) Enabled() bool {
	return m != nil && m.host != "" && m.from != "" && m.loginURL != ""
}

func (m *CredentialMailer) SendCredentials(name, email, role, password string) error {
	if !m.Enabled() { return fmt.Errorf("credential email is not configured") }
	if strings.TrimSpace(email) == "" { return fmt.Errorf("recipient email is missing") }
	subject := "Your Stonez Digital school portal login"
	body := fmt.Sprintf("Hello %s,\n\nYour school portal account has been created.\n\nRole: %s\nLogin: %s\nTemporary password: %s\n\nSign in: %s\n\nFor security, change your password after signing in and do not share these credentials.\n\nStonez Digital", name, role, email, password, m.loginURL)
	return m.send(email, subject, body)
}

// BuildPasswordResetURL adds the one-time token without discarding existing URL parameters.
func BuildPasswordResetURL(baseURL, token string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return "", fmt.Errorf("password reset URL must be an absolute HTTP(S) URL")
	}
	query := parsed.Query()
	query.Set("token", token)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (m *CredentialMailer) SendPasswordReset(name, email, token string) error {
	if m == nil || m.host == "" || m.from == "" || strings.TrimSpace(m.passwordResetURL) == "" {
		return fmt.Errorf("password reset email is not configured")
	}
	if strings.TrimSpace(email) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("password reset recipient and token are required")
	}
	resetURL, err := BuildPasswordResetURL(m.passwordResetURL, token)
	if err != nil { return err }
	greeting := strings.TrimSpace(name)
	if greeting == "" { greeting = "there" }
	body := fmt.Sprintf("Hello %s,\n\nWe received a request to reset your Stonez Digital school portal password.\n\nUse the secure link below to choose a new password. This link expires in 15 minutes and can only be used once.\n\n%s\n\nIf you did not request this change, you can ignore this email. Your password will remain unchanged.\n\nStonez Digital", greeting, resetURL)
	return m.send(email, "Reset your Stonez Digital school portal password", body)
}

func (m *CredentialMailer) send(email, subject, body string) error {
	if strings.ContainsAny(email, "\r\n") { return fmt.Errorf("invalid recipient email") }
	fromName := strings.ReplaceAll(strings.ReplaceAll(m.fromName, "\r", ""), "\n", "")
	from := m.from
	if fromName != "" { from = fromName + " <" + m.from + ">" }
	msg := []byte("From: " + from + "\r\nTo: " + email + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n")
	addr := net.JoinHostPort(m.host, strconv.Itoa(m.port))
	if m.port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12})
		if err != nil { return err }
		client, err := smtp.NewClient(conn, m.host)
		if err != nil { return err }
		defer client.Close()
		if m.username != "" {
			if err = client.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil { return err }
		}
		if err = client.Mail(m.from); err != nil { return err }
		if err = client.Rcpt(email); err != nil { return err }
		writer, err := client.Data()
		if err != nil { return err }
		if _, err = writer.Write(msg); err != nil { _ = writer.Close(); return err }
		return writer.Close()
	}
	var auth smtp.Auth
	if m.username != "" { auth = smtp.PlainAuth("", m.username, m.password, m.host) }
	return smtp.SendMail(addr, auth, m.from, []string{email}, msg)
}

func GenerateTemporaryPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%*"
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil { return "", err }
	out := make([]byte, len(b))
	for i, value := range b { out[i] = alphabet[int(value)%len(alphabet)] }
	return string(out), nil
}

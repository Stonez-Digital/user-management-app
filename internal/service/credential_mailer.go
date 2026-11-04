package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const resendEmailsEndpoint = "https://api.resend.com/emails"

type CredentialMailer struct {
	host string
	port int
	username string
	password string
	resendAPIKey string
	resendAPIURL string
	httpClient *http.Client
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
	resendKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	// Existing Resend SMTP configuration stores the Resend API key in SMTP_PASSWORD.
	// Reuse it for HTTPS delivery when the configured SMTP host is Resend.
	if resendKey == "" && strings.EqualFold(strings.TrimSpace(os.Getenv("SMTP_HOST")), "smtp.resend.com") &&
		strings.EqualFold(strings.TrimSpace(os.Getenv("SMTP_USERNAME")), "resend") {
		resendKey = strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	}
	return &CredentialMailer{
		host: strings.TrimSpace(os.Getenv("SMTP_HOST")), port: port,
		username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")), password: os.Getenv("SMTP_PASSWORD"),
		resendAPIKey: resendKey, resendAPIURL: resendEmailsEndpoint, httpClient: &http.Client{Timeout: 12 * time.Second},
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
	parsed.Fragment = url.Values{"token": []string{token}}.Encode()
	return parsed.String(), nil
}

func (m *CredentialMailer) SendPasswordReset(name, email, token string) error {
	if m == nil || m.from == "" || strings.TrimSpace(m.passwordResetURL) == "" {
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
	subject := "Reset your Stonez Digital school portal password"

	if m.resendAPIKey != "" {
		return m.sendViaResend(email, subject, body)
	}
	if strings.EqualFold(m.host, "smtp.resend.com") {
		return fmt.Errorf("Resend API key is missing; set RESEND_API_KEY or the existing SMTP_PASSWORD")
	}
	return m.send(email, subject, body)
}

func (m *CredentialMailer) sendViaResend(email, subject, body string) error {
	if strings.ContainsAny(email, "\r\n") { return fmt.Errorf("invalid recipient email") }
	payload := struct {
		From string `json:"from"`
		To []string `json:"to"`
		Subject string `json:"subject"`
		Text string `json:"text"`
	}{From: m.from, To: []string{email}, Subject: subject, Text: body}
	encoded, err := json.Marshal(payload)
	if err != nil { return fmt.Errorf("encode Resend email: %w", err) }
	endpoint := m.resendAPIURL
	if endpoint == "" { endpoint = resendEmailsEndpoint }
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil { return fmt.Errorf("create Resend request: %w", err) }
	req.Header.Set("Authorization", "Bearer "+m.resendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	client := m.httpClient
	if client == nil { client = &http.Client{Timeout: 12*time.Second} }
	resp, err := client.Do(req)
	if err != nil { return fmt.Errorf("Resend API request failed: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Resend API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	return nil
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

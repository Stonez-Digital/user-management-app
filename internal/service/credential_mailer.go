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
	"errors"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const resendEmailsEndpoint = "https://api.resend.com/emails"

const smtpOperationTimeout = 35 * time.Second

type providerHTTPError struct { statusCode int }

func (e *providerHTTPError) Error() string { return fmt.Sprintf("email provider returned HTTP %d", e.statusCode) }

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
	return m != nil && m.from != "" && m.loginURL != "" && (m.host != "" || m.resendAPIKey != "")
}

// SendAccountSetupNotice never includes a password or reset token. The
// recipient starts the existing password-reset flow from the portal login.
func (m *CredentialMailer) SendAccountSetupNotice(name, email, role, eventID string) error {
	if !m.Enabled() { return fmt.Errorf("account-setup email is not configured") }
	if strings.TrimSpace(email) == "" || strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("account-setup recipient and event ID are required")
	}
	greeting := strings.TrimSpace(name)
	if greeting == "" { greeting = "there" }
	subject := "Your Stonez Digital school portal account is ready"
	body := fmt.Sprintf(
		"Hello %s,\n\nYour school portal account has been created.\n\nRole: %s\nEmail: %s\n\nTo choose your password, visit %s and select Forgot password. We will send a separate, time-limited password-reset link to this email address.\n\nFor your security, never share password-reset links or verification codes.\n\nStonez Digital",
		greeting, role, email, m.loginURL,
	)
	if m.resendAPIKey != "" {
		return m.sendViaResendWithIdempotency(email, subject, body, "stonez-account-setup-"+eventID)
	}
	if strings.EqualFold(m.host, "smtp.resend.com") {
		return fmt.Errorf("Resend API key is missing; set RESEND_API_KEY or the existing SMTP_PASSWORD")
	}
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
	return m.sendViaResendWithIdempotency(email, subject, body, "")
}

func (m *CredentialMailer) sendViaResendWithIdempotency(email, subject, body, idempotencyKey string) error {
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
	if idempotencyKey != "" { req.Header.Set("Idempotency-Key", idempotencyKey) }
	client := m.httpClient
	if client == nil { client = &http.Client{Timeout: 12*time.Second} }
	resp, err := client.Do(req)
	if err != nil { return fmt.Errorf("Resend API request failed: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return &providerHTTPError{statusCode: resp.StatusCode}
	}
	return nil
}

func (m *CredentialMailer) send(email, subject, body string) error {
	if strings.ContainsAny(email, "\r\n") { return fmt.Errorf("invalid recipient email") }
	fromName := strings.ReplaceAll(strings.ReplaceAll(m.fromName, "\r", ""), "\n", "")
	from := m.from
	if fromName != "" { from = fromName + " <" + m.from + ">" }
	msg := []byte("From: " + from + "\r\nTo: " + email + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n")
	return sendSMTPWithTimeout(m.host, m.port, m.username, m.password, m.from, email, msg, smtpOperationTimeout)
}

// sendSMTPWithTimeout bounds the entire SMTP exchange (including greeting,
// TLS handshake, authentication, message transfer and final response). The
// single absolute deadline prevents a peer from extending the lease by
// periodically sending partial data.
func sendSMTPWithTimeout(host string, port int, username, password, from, recipient string, message []byte, timeout time.Duration) error {
	if timeout <= 0 { return fmt.Errorf("SMTP timeout must be positive") }
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: timeout}
	var conn net.Conn
	var err error
	if port == 465 {
		raw, dialErr := dialer.Dial("tcp", addr)
		if dialErr != nil { return dialErr }
		_ = raw.SetDeadline(deadline)
		tlsConn := tls.Client(raw, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
			_ = raw.Close()
			return handshakeErr
		}
		conn = tlsConn
	} else {
		conn, err = dialer.Dial("tcp", addr)
		if err != nil { return err }
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil { return err }

	client, err := smtp.NewClient(conn, host)
	if err != nil { return err }
	defer client.Close()
	if err := client.Hello("localhost"); err != nil { return err }
	if port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil { return err }
		}
	}
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, password, host)); err != nil { return err }
	}
	if err := client.Mail(from); err != nil { return err }
	if err := client.Rcpt(recipient); err != nil { return err }
	writer, err := client.Data()
	if err != nil { return err }
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil { return err }
	if err := client.Quit(); err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() { return err }
	}
	return nil
}

func GenerateTemporaryPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%*"
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil { return "", err }
	out := make([]byte, len(b))
	for i, value := range b { out[i] = alphabet[int(value)%len(alphabet)] }
	return string(out), nil
}

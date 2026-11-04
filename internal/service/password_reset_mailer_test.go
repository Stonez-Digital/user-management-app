package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBuildPasswordResetURLPreservesExistingQuery(t *testing.T) {
	got, err := BuildPasswordResetURL("https://portal.example/reset-password?source=email", "a+b/c")
	if err != nil { t.Fatal(err) }
	parsed, err := url.Parse(got)
	if err != nil { t.Fatal(err) }
	if parsed.Query().Get("source") != "email" { t.Fatal("existing query parameter was lost") }
	if parsed.Query().Get("token") != "" { t.Fatal("reset token must not be placed in the query string") }
	if parsed.Fragment == "" { t.Fatal("expected token in URL fragment") }
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil { t.Fatal(err) }
	if fragment.Get("token") != "a+b/c" { t.Fatalf("token was not safely encoded: %q", fragment.Get("token")) }
}

func TestBuildPasswordResetURLRejectsNonHTTPAndRelativeURLs(t *testing.T) {
	for _, input := range []string{"/reset-password", "javascript:alert(1)", "ftp://portal.example/reset"} {
		if _, err := BuildPasswordResetURL(input, "token"); err == nil { t.Fatalf("expected %q to be rejected", input) }
	}
}

func TestSendPasswordResetUsesResendHTTPSAPI(t *testing.T) {
	var gotAuthorization string
	var gotPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.Method != http.MethodPost { t.Errorf("expected POST, got %s", r.Method) }
		if r.URL.Path != "/emails" { t.Errorf("unexpected path: %s", r.URL.Path) }
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil { t.Errorf("decode payload: %v", err) }
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"email_test_123"}`))
	}))
	defer server.Close()

	mailer := &CredentialMailer{
		from: "noreply@stonezdigital.com.ng", resendAPIKey: "re_test_key",
		resendAPIURL: server.URL + "/emails", httpClient: server.Client(),
		passwordResetURL: "https://portal.example/reset-password",
	}
	if err := mailer.SendPasswordReset("Test User", "user@example.com", "one-time-token"); err != nil { t.Fatal(err) }
	if gotAuthorization != "Bearer re_test_key" { t.Fatalf("unexpected authorization header: %q", gotAuthorization) }
	if gotPayload["from"] != "noreply@stonezdigital.com.ng" { t.Fatalf("unexpected sender: %#v", gotPayload["from"]) }
	if gotPayload["subject"] != "Reset your Stonez Digital school portal password" { t.Fatalf("unexpected subject: %#v", gotPayload["subject"]) }
	to, ok := gotPayload["to"].([]interface{})
	if !ok || len(to) != 1 || to[0] != "user@example.com" { t.Fatalf("unexpected recipient: %#v", gotPayload["to"]) }
	body, _ := gotPayload["text"].(string)
	if !strings.Contains(body, "one-time-token") { t.Fatal("reset URL missing from email body") }
}

func TestSendPasswordResetDoesNotUseBlockedResendSMTPPorts(t *testing.T) {
	mailer := &CredentialMailer{
		host: "smtp.resend.com", from: "noreply@stonezdigital.com.ng",
		passwordResetURL: "https://portal.example/reset-password",
	}
	err := mailer.SendPasswordReset("Test User", "user@example.com", "token")
	if err == nil || !strings.Contains(err.Error(), "Resend API key is missing") {
		t.Fatalf("expected actionable missing API key error, got %v", err)
	}
}

package service

import (
	"net/url"
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

package service

import (
    "io"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/google/uuid"
)

func TestSchoolLogoUploadUpdatesExistingBucketWithPut(t *testing.T) {
    var sawBucketPut, sawObjectUpload bool
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch {
        case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/bucket":
            w.WriteHeader(http.StatusConflict)
        case r.Method == http.MethodPut && r.URL.Path == "/storage/v1/bucket/school-logos":
            sawBucketPut = true
            w.WriteHeader(http.StatusOK)
        case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/storage/v1/object/school-logos/"):
            sawObjectUpload = true
            if r.Header.Get("Content-Type") != "image/png" { t.Errorf("expected image/png, got %q", r.Header.Get("Content-Type")) }
            _, _ = io.Copy(io.Discard, r.Body)
            w.WriteHeader(http.StatusOK)
        default:
            t.Errorf("unexpected storage request: %s %s", r.Method, r.URL.Path)
            http.NotFound(w, r)
        }
    }))
    defer server.Close()

    storage := &SchoolLogoStorage{baseURL: server.URL, secretKey: "test-secret", client: server.Client()}
    logoURL, err := storage.Upload(uuid.New(), "school.png", "image/png", strings.NewReader(string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})), 8)
    if err != nil { t.Fatalf("expected upload to succeed: %v", err) }
    if !sawBucketPut { t.Fatal("expected existing bucket configuration to be updated with PUT") }
    if !sawObjectUpload { t.Fatal("expected logo object to be uploaded") }
    if !strings.Contains(logoURL, "/storage/v1/object/public/school-logos/") { t.Fatalf("unexpected public logo URL: %s", logoURL) }
}

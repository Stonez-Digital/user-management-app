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

const schoolLogoBucket = "school-logos"

type SchoolLogoStorage struct {
    baseURL string
    secretKey string
    client *http.Client
}

func NewSchoolLogoStorageFromEnv() *SchoolLogoStorage {
    return &SchoolLogoStorage{
        baseURL: strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
        secretKey: strings.TrimSpace(os.Getenv("SUPABASE_SECRET_KEY")),
        client: &http.Client{Timeout: 30 * time.Second},
    }
}

func (s *SchoolLogoStorage) Upload(schoolID uuid.UUID, filename, contentType string, body io.Reader, size int64) (string, error) {
    if s.baseURL == "" || s.secretKey == "" {
        return "", fmt.Errorf("school logo storage is not configured")
    }
    if size <= 0 || size > 2*1024*1024 {
        return "", fmt.Errorf("school logo must be between 1 byte and 2 MB")
    }

    ext := strings.ToLower(path.Ext(filename))
    if ext == "" {
        ext = extensionForMime(contentType)
    }
    allowed := map[string]string{
        ".png": "image/png",
        ".jpg": "image/jpeg",
        ".jpeg": "image/jpeg",
        ".webp": "image/webp",
    }
    expectedType, ok := allowed[ext]
    if !ok || expectedType != contentType {
        return "", fmt.Errorf("school logo must be PNG, JPEG, or WebP")
    }

    data, err := io.ReadAll(io.LimitReader(body, 2*1024*1024+1))
    if err != nil {
        return "", fmt.Errorf("failed to read school logo: %w", err)
    }
    if int64(len(data)) > 2*1024*1024 {
        return "", fmt.Errorf("school logo must not exceed 2 MB")
    }
    if http.DetectContentType(data) != contentType {
        return "", fmt.Errorf("uploaded file content does not match its declared image type")
    }

    if err := s.ensureBucket(); err != nil {
        return "", err
    }

    objectPath := schoolID.String() + "/" + uuid.NewString() + ext
    uploadURL := s.baseURL + "/storage/v1/object/" + schoolLogoBucket + "/" + objectPath
    req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(data))
    if err != nil {
        return "", fmt.Errorf("failed to create school logo upload request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+s.secretKey)
    req.Header.Set("apikey", s.secretKey)
    req.Header.Set("Content-Type", contentType)
    req.Header.Set("x-upsert", "false")

    resp, err := s.client.Do(req)
    if err != nil {
        return "", fmt.Errorf("school logo upload failed: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return "", fmt.Errorf("school logo upload failed: %s", strings.TrimSpace(string(detail)))
    }

    return s.baseURL + "/storage/v1/object/public/" + schoolLogoBucket + "/" + objectPath, nil
}

func (s *SchoolLogoStorage) ensureBucket() error {
    bucketURL := s.baseURL + "/storage/v1/bucket"
    payload := []byte(`{"id":"school-logos","name":"school-logos","public":true,"file_size_limit":2097152,"allowed_mime_types":["image/png","image/jpeg","image/webp"]}`)
    req, err := http.NewRequest(http.MethodPost, bucketURL, bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("failed to create school logo bucket request: %w", err)
    }
    req.Header.Set("Authorization", "Bearer "+s.secretKey)
    req.Header.Set("apikey", s.secretKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return fmt.Errorf("school logo storage setup failed: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusConflict {
        return nil
    }
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return fmt.Errorf("school logo storage setup failed: %s", strings.TrimSpace(string(detail)))
    }
    return nil
}

func extensionForMime(contentType string) string {
    exts, _ := mime.ExtensionsByType(contentType)
    if len(exts) > 0 {
        return exts[0]
    }
    return ""
}

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type CloudflareSchoolDomainProvisioner struct {
	token      string
	accountID  string
	workerName string
	baseDomain string
	apiBaseURL string
	client     *http.Client
}

func NewCloudflareSchoolDomainProvisionerFromEnv() *CloudflareSchoolDomainProvisioner {
	token := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	accountID := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	workerName := strings.TrimSpace(os.Getenv("CLOUDFLARE_WORKER_NAME"))
	baseDomain := strings.Trim(strings.ToLower(strings.TrimSpace(os.Getenv("SCHOOL_BASE_DOMAIN"))), ".")
	if baseDomain == "" {
		baseDomain = "stonezdigital.com.ng"
	}
	if token == "" || accountID == "" || workerName == "" {
		return nil
	}
	return &CloudflareSchoolDomainProvisioner{
		token: token, accountID: accountID, workerName: workerName, baseDomain: baseDomain,
		apiBaseURL: "https://api.cloudflare.com/client/v4", client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *CloudflareSchoolDomainProvisioner) Attach(ctx context.Context, hostname string) error {
	if p == nil || p.token == "" || p.accountID == "" || p.workerName == "" {
		return fmt.Errorf("Cloudflare school-domain provisioning is not configured")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	suffix := "." + p.baseDomain
	if !strings.HasSuffix(host, suffix) {
		return fmt.Errorf("school hostname is outside the configured base domain")
	}
	label := strings.TrimSuffix(host, suffix)
	if label == "" || strings.Contains(label, ".") || len(label) > 63 ||
		!validCloudflareLabel(label) || label == "app" || label == "www" || label == "api" || label == "mail" {
		return fmt.Errorf("invalid or reserved school hostname")
	}

	body, err := json.Marshal(map[string]string{"hostname": host, "service": p.workerName})
	if err != nil {
		return err
	}
	url := strings.TrimRight(p.apiBaseURL, "/") + "/accounts/" + p.accountID + "/workers/domains"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("Cloudflare domain request failed: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Cloudflare returned an unreadable response (HTTP %d)", resp.StatusCode)
	}
	success, _ := result["success"].(bool)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !success {
		return fmt.Errorf("Cloudflare rejected the domain (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func validCloudflareLabel(label string) bool {
	if len(label) == 0 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

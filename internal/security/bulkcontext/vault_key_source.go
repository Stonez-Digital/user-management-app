package bulkcontext

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// VaultKVKeySource reads versioned HMAC keys from a Vault KV v2 secret.
// The token is read from a Vault Agent token file and is never logged.
type VaultKVKeySource struct {
	address   string
	tokenFile string
	mount     string
	path      string
	client    *http.Client
	cacheTTL  time.Duration

	mu        sync.RWMutex
	activeID  string
	keys      map[string][]byte
	expiresAt time.Time
}

func NewVaultKVKeySource(address, tokenFile, mount, secretPath string, client *http.Client, cacheTTL time.Duration) (*VaultKVKeySource, error) {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("Vault address must be an HTTPS URL without user info, query, or fragment")
	}
	if strings.TrimSpace(tokenFile) == "" || !validVaultPath(mount) || !validVaultPath(secretPath) {
		return nil, errors.New("Vault token file, KV mount, and secret path are required")
	}
	if cacheTTL <= 0 || cacheTTL > 5*time.Minute {
		return nil, errors.New("Vault key cache TTL must be greater than zero and no more than five minutes")
	}
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &VaultKVKeySource{
		address: strings.TrimRight(parsed.String(), "/"),
		tokenFile: tokenFile,
		mount: strings.Trim(mount, "/"),
		path: strings.Trim(secretPath, "/"),
		client: client,
		cacheTTL: cacheTTL,
		keys: make(map[string][]byte),
	}, nil
}

func validVaultPath(value string) bool {
	value = strings.Trim(value, "/")
	if value == "" || strings.Contains(value, "\\") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, r := range segment {
			if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
				!(r >= '0' && r <= '9') && r != '-' && r != '_' {
				return false
			}
		}
	}
	return true
}

func (v *VaultKVKeySource) ActiveSigningKey(ctx context.Context) (Key, error) {
	if err := v.refresh(ctx); err != nil {
		return Key{}, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	secret, ok := v.keys[v.activeID]
	if !ok {
		return Key{}, ErrInvalidKey
	}
	return Key{ID: v.activeID, Secret: append([]byte(nil), secret...)}, nil
}

func (v *VaultKVKeySource) VerificationKey(ctx context.Context, keyID string) (Key, error) {
	if err := v.refresh(ctx); err != nil {
		return Key{}, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	secret, ok := v.keys[keyID]
	if !ok {
		return Key{}, fmt.Errorf("Vault does not contain bulk import verification key %q", keyID)
	}
	return Key{ID: keyID, Secret: append([]byte(nil), secret...)}, nil
}

func (v *VaultKVKeySource) refresh(ctx context.Context) error {
	v.mu.RLock()
	fresh := time.Now().Before(v.expiresAt) && v.activeID != "" && len(v.keys) > 0
	v.mu.RUnlock()
	if fresh {
		return nil
	}

	tokenBytes, err := os.ReadFile(v.tokenFile)
	if err != nil {
		return fmt.Errorf("read Vault Agent token file: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return errors.New("Vault Agent token file is empty")
	}

	endpoint := v.address + "/v1/" + escapeVaultPath(v.mount) + "/data/" + escapeVaultPath(v.path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Vault key request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("request bulk import signing keys from Vault: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Vault key request returned HTTP %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return fmt.Errorf("decode Vault key response: %w", err)
	}
	outerData, ok := payload["data"].(map[string]any)
	if !ok {
		return ErrInvalidKey
	}
	secretData, ok := outerData["data"].(map[string]any)
	if !ok {
		return ErrInvalidKey
	}
	activeID, ok := secretData["active_key_id"].(string)
	if !ok || !keyIDPattern.MatchString(activeID) {
		return ErrInvalidKey
	}
	rawKeys, ok := secretData["keys"].(map[string]any)
	if !ok || len(rawKeys) == 0 {
		return ErrInvalidKey
	}

	next := make(map[string][]byte, len(rawKeys))
	for id, value := range rawKeys {
		encoded, ok := value.(string)
		if !ok || !keyIDPattern.MatchString(id) {
			zeroKeys(next)
			return ErrInvalidKey
		}
		secret, err := base64.RawStdEncoding.DecodeString(encoded)
		if err != nil || len(secret) < minimumKeyBytes {
			zeroKeys(next)
			return ErrInvalidKey
		}
		next[id] = secret
	}
	if _, ok := next[activeID]; !ok {
		zeroKeys(next)
		return ErrInvalidKey
	}

	v.mu.Lock()
	old := v.keys
	v.keys = next
	v.activeID = activeID
	v.expiresAt = time.Now().Add(v.cacheTTL)
	v.mu.Unlock()
	zeroKeys(old)
	return nil
}

func escapeVaultPath(value string) string {
	segments := strings.Split(strings.Trim(value, "/"), "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return strings.Join(segments, "/")
}

func zeroKeys(keys map[string][]byte) {
	for _, secret := range keys {
		for i := range secret {
			secret[i] = 0
		}
	}
}

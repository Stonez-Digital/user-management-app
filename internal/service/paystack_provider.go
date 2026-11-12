package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const paystackAPIBaseURL = "https://api.paystack.co"

var (
	ErrPaystackNotConfigured = errors.New("paystack is not configured")
	ErrPaystackRequest       = errors.New("paystack request failed")
	ErrPaystackVerification  = errors.New("paystack transaction verification failed")
)

type PaystackProvider struct {
	secretKey string
	client    *http.Client
	baseURL   string
}

type PaystackInitializeInput struct {
	Email        string
	AmountMinor  int64
	Reference    string
	CallbackURL  string
	Metadata     map[string]interface{}
}

type PaystackInitializeResult struct {
	AuthorizationURL string `json:"authorization_url"`
	AccessCode       string `json:"access_code"`
	Reference        string `json:"reference"`
}

type paystackEnvelope[T any] struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type paystackTransaction struct {
	Status    string                 `json:"status"`
	Reference string                 `json:"reference"`
	Amount    int64                  `json:"amount"`
	Currency  string                 `json:"currency"`
	PaidAt    string                 `json:"paid_at"`
	Metadata  map[string]interface{} `json:"metadata"`
}

func NewPaystackProvider(secretKey string, client *http.Client) (*PaystackProvider, error) {
	secretKey = strings.TrimSpace(secretKey)
	if secretKey == "" {
		return nil, ErrPaystackNotConfigured
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &PaystackProvider{secretKey: secretKey, client: client, baseURL: paystackAPIBaseURL}, nil
}

func NewPaystackProviderFromEnv() (*PaystackProvider, error) {
	return NewPaystackProvider(os.Getenv("PAYSTACK_SECRET_KEY"), nil)
}

func (p *PaystackProvider) Name() string { return "paystack" }

// Initialize creates a checkout session on Paystack. AmountMinor is an integer
// number of kobo; the caller must derive it from the server-side billing plan.
func (p *PaystackProvider) Initialize(ctx context.Context, in PaystackInitializeInput) (PaystackInitializeResult, error) {
	var result PaystackInitializeResult
	if p == nil || p.secretKey == "" {
		return result, ErrPaystackNotConfigured
	}
	if strings.TrimSpace(in.Email) == "" || in.AmountMinor <= 0 || !validPaystackReference(in.Reference) {
		return result, ErrPaystackRequest
	}
	if in.CallbackURL != "" {
		u, err := url.ParseRequestURI(in.CallbackURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return result, ErrPaystackRequest
		}
	}
	payload := map[string]interface{}{
		"email": in.Email, "amount": strconv.FormatInt(in.AmountMinor, 10),
		"currency": "NGN", "reference": in.Reference,
	}
	if in.CallbackURL != "" { payload["callback_url"] = in.CallbackURL }
	if in.Metadata != nil { payload["metadata"] = in.Metadata }
	var envelope paystackEnvelope[PaystackInitializeResult]
	if err := p.doJSON(ctx, http.MethodPost, "/transaction/initialize", payload, &envelope); err != nil {
		return result, err
	}
	if !envelope.Status || envelope.Data.AuthorizationURL == "" || envelope.Data.Reference != in.Reference {
		return result, ErrPaystackRequest
	}
	return envelope.Data, nil
}

// Verify fetches the transaction from Paystack and accepts only a successful
// NGN transaction with a matching reference and positive amount.
func (p *PaystackProvider) Verify(reference string) (PaymentVerification, error) {
	var result PaymentVerification
	if p == nil || p.secretKey == "" {
		return result, ErrPaystackNotConfigured
	}
	reference = strings.TrimSpace(reference)
	if !validPaystackReference(reference) { return result, ErrPaystackVerification }

	var envelope paystackEnvelope[paystackTransaction]
	path := "/transaction/verify/" + url.PathEscape(reference)
	if err := p.doJSON(context.Background(), http.MethodGet, path, nil, &envelope); err != nil {
		return result, ErrPaystackVerification
	}
	tx := envelope.Data
	if !envelope.Status || tx.Status != "success" || tx.Reference != reference ||
		tx.Amount <= 0 || !strings.EqualFold(tx.Currency, "NGN") {
		return result, ErrPaystackVerification
	}
	paidAt, err := time.Parse(time.RFC3339, tx.PaidAt)
	if err != nil { return result, ErrPaystackVerification }
	return PaymentVerification{
		Provider: "paystack", Reference: tx.Reference,
		Amount: float64(tx.Amount) / 100, AmountMinor: tx.Amount,
		Currency: "NGN", Status: "succeeded", PaidAt: paidAt.UTC(),
		Metadata: tx.Metadata,
	}, nil
}

func (p *PaystackProvider) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil { return ErrPaystackRequest }
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil { return ErrPaystackRequest }
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil { req.Header.Set("Content-Type", "application/json") }
	resp, err := p.client.Do(req)
	if err != nil { return ErrPaystackRequest }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return ErrPaystackRequest }
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(out); err != nil { return ErrPaystackRequest }
	return nil
}

func validPaystackReference(reference string) bool {
	if len(reference) == 0 || len(reference) > 100 { return false }
	for _, r := range reference {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' && r != '.' && r != '=' {
			return false
		}
	}
	return true
}

var _ PaymentProvider = (*PaystackProvider)(nil)

func (in PaystackInitializeInput) String() string {
	return fmt.Sprintf("PaystackInitializeInput{amount_minor:%d, reference:%q}", in.AmountMinor, in.Reference)
}

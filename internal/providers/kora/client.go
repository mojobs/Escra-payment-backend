package kora

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
	"time"

	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

const defaultBaseURL = "https://api.korapay.com"

type Client struct {
	baseURL    string
	base       *url.URL
	secretKey  string
	httpClient *http.Client
}

func NewClient(baseURL, secretKey string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	trimmed := strings.TrimRight(baseURL, "/")
	var parsed *url.URL
	if u, err := url.Parse(trimmed); err == nil {
		parsed = u
	}
	return &Client{
		baseURL:   trimmed,
		base:      parsed,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type PayoutRequest struct {
	Reference   string
	Amount      money.Amount
	Currency    string
	Narration   string
	BankCode    string
	AccountName string
	AccountNo   string
	Email       string
}

type CheckoutRequest struct {
	Reference         string
	Amount            money.Amount
	Currency          string
	RedirectURL       string
	NotificationURL   string
	DefaultChannel    string
	Channels          []string
	MerchantBearsCost bool
	CustomerName      string
	CustomerEmail     string
	CustomerPhone     string
	Description       string
	Metadata          map[string]interface{}
}

type CheckoutResponse struct {
	Reference   string          `json:"reference"`
	CheckoutURL string          `json:"checkout_url"`
	Status      string          `json:"status"`
	Raw         json.RawMessage `json:"raw"`
}

type payoutPayload struct {
	Reference   string                 `json:"reference"`
	Destination payoutDestination      `json:"destination"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type payoutDestination struct {
	Type        string      `json:"type"`
	Amount      json.Number `json:"amount"`
	Currency    string      `json:"currency"`
	Narration   string      `json:"narration,omitempty"`
	BankAccount bankAccount `json:"bank_account"`
	Customer    customer    `json:"customer"`
}

type bankAccount struct {
	Bank    string `json:"bank"`
	Account string `json:"account"`
}

type customer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"`
}

type PayoutResponse struct {
	Reference         string          `json:"reference"`
	Status            string          `json:"status"`
	ExternalReference string          `json:"external_reference,omitempty"`
	Raw               json.RawMessage `json:"raw"`
}

type Bank struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Code    string `json:"code"`
	Country string `json:"country"`
}

type ResolveBankAccountRequest struct {
	BankCode      string `json:"bank"`
	AccountNumber string `json:"account"`
	Currency      string `json:"currency"`
}

type ResolvedBankAccount struct {
	BankName      string `json:"bank_name"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

func (c *Client) RequestPayout(ctx context.Context, req PayoutRequest) (*PayoutResponse, error) {
	payload := payoutPayload{
		Reference: req.Reference,
		Destination: payoutDestination{
			Type:      "bank_account",
			Amount:    json.Number(req.Amount.String()),
			Currency:  req.Currency,
			Narration: req.Narration,
			BankAccount: bankAccount{
				Bank:    req.BankCode,
				Account: req.AccountNo,
			},
			Customer: customer{
				Name:  req.AccountName,
				Email: req.Email,
			},
		},
	}

	var raw json.RawMessage
	if err := c.post(ctx, "/merchant/api/v1/transactions/disburse", payload, &raw); err != nil {
		return nil, err
	}

	response := &PayoutResponse{
		Reference: req.Reference,
		Raw:       raw,
	}

	var envelope struct {
		Status interface{} `json:"status"`
		Data   struct {
			Reference string `json:"reference"`
			Status    string `json:"status"`
			TraceID   string `json:"trace_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		response.Status = firstNonEmpty(envelope.Data.Status, providerStatusString(envelope.Status))
		response.ExternalReference = envelope.Data.TraceID
		if envelope.Data.Reference != "" {
			response.Reference = envelope.Data.Reference
		}
	}

	return response, nil
}

func (c *Client) InitializeCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResponse, error) {
	type checkoutPayload struct {
		Amount            json.Number            `json:"amount"`
		Currency          string                 `json:"currency"`
		RedirectURL       string                 `json:"redirect_url,omitempty"`
		NotificationURL   string                 `json:"notification_url"`
		Reference         string                 `json:"reference"`
		DefaultChannel    string                 `json:"default_channel,omitempty"`
		Channels          []string               `json:"channels,omitempty"`
		MerchantBearsCost bool                   `json:"merchant_bears_cost"`
		Description       string                 `json:"description,omitempty"`
		Customer          customer               `json:"customer"`
		Metadata          map[string]interface{} `json:"metadata,omitempty"`
	}

	payload := checkoutPayload{
		Amount:            json.Number(req.Amount.String()),
		Currency:          req.Currency,
		RedirectURL:       req.RedirectURL,
		NotificationURL:   req.NotificationURL,
		Reference:         req.Reference,
		DefaultChannel:    req.DefaultChannel,
		Channels:          req.Channels,
		MerchantBearsCost: req.MerchantBearsCost,
		Description:       req.Description,
		Customer: customer{
			Name:  req.CustomerName,
			Email: req.CustomerEmail,
			Phone: req.CustomerPhone,
		},
		Metadata: req.Metadata,
	}

	var raw json.RawMessage
	if err := c.post(ctx, "/merchant/api/v1/charges/initialize", payload, &raw); err != nil {
		return nil, err
	}

	response := &CheckoutResponse{
		Reference: req.Reference,
		Status:    "pending",
		Raw:       raw,
	}

	var envelope struct {
		Status interface{} `json:"status"`
		Data   struct {
			Reference    string `json:"reference"`
			CheckoutURL  string `json:"checkout_url"`
			CheckoutURL2 string `json:"checkoutUrl"`
			PaymentURL   string `json:"payment_url"`
			Status       string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		response.Status = firstNonEmpty(envelope.Data.Status, providerStatusString(envelope.Status), response.Status)
		response.CheckoutURL = firstNonEmpty(envelope.Data.CheckoutURL, envelope.Data.CheckoutURL2, envelope.Data.PaymentURL)
		if envelope.Data.Reference != "" {
			response.Reference = envelope.Data.Reference
		}
	}

	return response, nil
}

func (c *Client) ListBanks(ctx context.Context, countryCode string) ([]Bank, error) {
	if countryCode == "" {
		countryCode = "NG"
	}

	path := "/merchant/api/v1/misc/banks?countryCode=" + url.QueryEscape(strings.ToUpper(countryCode))
	var envelope struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Data    []Bank `json:"data"`
	}
	if err := c.get(ctx, path, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Status {
		return nil, fmt.Errorf("kora list banks failed: %s", envelope.Message)
	}
	return envelope.Data, nil
}

func (c *Client) ResolveBankAccount(ctx context.Context, req ResolveBankAccountRequest) (*ResolvedBankAccount, error) {
	if req.Currency == "" {
		req.Currency = "NG"
	}

	var envelope struct {
		Status  bool                `json:"status"`
		Message string              `json:"message"`
		Data    ResolvedBankAccount `json:"data"`
	}
	if err := c.post(ctx, "/merchant/api/v1/misc/banks/resolve", req, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Status {
		return nil, fmt.Errorf("kora resolve bank account failed: %s", envelope.Message)
	}
	return &envelope.Data, nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) post(ctx context.Context, path string, payload interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, payload, out)
}

func (c *Client) do(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
	if c.secretKey == "" {
		return fmt.Errorf("kora secret key is not configured")
	}

	var body io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}

	// Build URL safely by joining the base URL and the request path so
	// callers can supply a base that already includes "/merchant" or not.
	var fullURL string
	if c.base != nil {
		u := *c.base
		u.Path = pathpkg.Join(u.Path, strings.TrimPrefix(path, "/"))
		fullURL = u.String()
	} else {
		fullURL = c.baseURL + path
	}
	request, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.secretKey)
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	raw := json.RawMessage{}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("kora request failed with status %d: %s", response.StatusCode, string(raw))
	}

	return json.Unmarshal(raw, out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func providerStatusString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "processing"
		}
		return "failed"
	default:
		return ""
	}
}

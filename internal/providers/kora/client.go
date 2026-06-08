package kora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	publicKey  string
	secretKey  string
	httpClient *http.Client
}

func NewClient(baseURL, publicKey, secretKey string) *Client {
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
		publicKey: publicKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("kora request failed with status %d: %s", e.StatusCode, e.Body)
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
	Narration         string
	Description       string
	Metadata          map[string]interface{}
}

type CheckoutResponse struct {
	Reference   string          `json:"reference"`
	CheckoutURL string          `json:"checkout_url"`
	Status      string          `json:"status"`
	Raw         json.RawMessage `json:"raw"`
}

type VerifyIdentityRequest struct {
	IDNumber string
	IDType   string
}

type VerifyIdentityResponse struct {
	Reference string          `json:"reference"`
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	Raw       json.RawMessage `json:"raw"`
}

type VirtualAccountRequest struct {
	AccountName      string
	AccountReference string
	BankCode         string
	Currency         string
	IDNumber         string
	IDType           string
	Permanent        bool
	CustomerName     string
	CustomerEmail    string
}

type VirtualAccountResponse struct {
	AccountReference   string          `json:"account_reference"`
	AccountName        string          `json:"account_name"`
	AccountNumber      string          `json:"account_number"`
	BankCode           string          `json:"bank_code"`
	BankName           string          `json:"bank_name"`
	Currency           string          `json:"currency"`
	Status             string          `json:"status"`
	ProviderCustomerID string          `json:"provider_customer_id,omitempty"`
	Raw                json.RawMessage `json:"raw"`
}

type Balance struct {
	Currency         string
	AvailableBalance money.Amount
	PendingBalance   money.Amount
	RawAvailable     string
	RawPending       string
}

type RefundRequest struct {
	Reference        string
	PaymentReference string
	Amount           money.Amount
	Currency         string
	Reason           string
}

type RefundResponse struct {
	Reference         string          `json:"reference"`
	Status            string          `json:"status"`
	ExternalReference string          `json:"external_reference,omitempty"`
	Raw               json.RawMessage `json:"raw"`
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
		Narration         string                 `json:"narration,omitempty"`
		DefaultChannel    string                 `json:"default_channel,omitempty"`
		Channels          []string               `json:"channels,omitempty"`
		MerchantBearsCost bool                   `json:"merchant_bears_cost"`
		Customer          customer               `json:"customer"`
		Metadata          map[string]interface{} `json:"metadata,omitempty"`
	}

	payload := checkoutPayload{
		Amount:            json.Number(req.Amount.String()),
		Currency:          req.Currency,
		RedirectURL:       req.RedirectURL,
		NotificationURL:   req.NotificationURL,
		Reference:         req.Reference,
		Narration:         sanitizeKorapayNarration(firstNonEmpty(req.Narration, req.Description)),
		DefaultChannel:    req.DefaultChannel,
		Channels:          req.Channels,
		MerchantBearsCost: req.MerchantBearsCost,
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

func sanitizeKorapayNarration(narration string) string {
	candidate := strings.TrimSpace(narration)
	if candidate == "" {
		candidate = "ESCRA Payment"
	}

	var builder strings.Builder
	builder.Grow(len(candidate))
	lastWasSpace := false
	for _, r := range candidate {
		allowed := (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			r == ' '
		if !allowed {
			r = ' '
		}
		if r == ' ' {
			if lastWasSpace {
				continue
			}
			lastWasSpace = true
		} else {
			lastWasSpace = false
		}
		builder.WriteRune(r)
	}

	sanitized := strings.TrimSpace(builder.String())
	if sanitized == "" {
		sanitized = "ESCRA Payment"
	}
	if len(sanitized) > 100 {
		sanitized = strings.TrimSpace(sanitized[:100])
	}
	return sanitized
}

func (c *Client) VerifyIdentity(ctx context.Context, req VerifyIdentityRequest) (*VerifyIdentityResponse, error) {
	idType := strings.ToLower(strings.TrimSpace(req.IDType))
	if idType == "" {
		idType = "bvn"
	}

	payload := map[string]string{idType: req.IDNumber}
	var raw json.RawMessage
	if err := c.postSecretFirst(ctx, []string{
		"/merchant/api/v1/identities/" + idType,
		"/merchant/api/v1/identities/ng/" + idType,
		"/merchant/api/v1/identity/" + idType,
	}, payload, &raw); err != nil {
		return nil, err
	}

	response := &VerifyIdentityResponse{
		Status: "verified",
		Raw:    raw,
	}
	var envelope struct {
		Status  interface{} `json:"status"`
		Message string      `json:"message"`
		Data    struct {
			Reference string `json:"reference"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		response.Message = envelope.Message
		response.Status = firstNonEmpty(envelope.Data.Status, providerStatusString(envelope.Status), response.Status)
		response.Reference = envelope.Data.Reference
	}

	return response, nil
}

func (c *Client) CreateVirtualAccount(ctx context.Context, req VirtualAccountRequest) (*VirtualAccountResponse, error) {
	idType := strings.ToLower(strings.TrimSpace(req.IDType))
	payload := map[string]interface{}{
		"account_name":      req.AccountName,
		"account_reference": req.AccountReference,
		"bank_code":         req.BankCode,
		"currency":          req.Currency,
		"permanent":         req.Permanent,
		"customer": map[string]string{
			"name":  req.CustomerName,
			"email": req.CustomerEmail,
		},
		"kyc": map[string]string{
			idType: req.IDNumber,
		},
	}

	var raw json.RawMessage
	if err := c.postSecretFirst(ctx, []string{
		"/merchant/api/v1/virtual-bank-account",
		"/merchant/api/v1/virtual-bank-accounts",
	}, payload, &raw); err != nil {
		return nil, err
	}

	response := &VirtualAccountResponse{
		AccountReference: req.AccountReference,
		AccountName:      req.AccountName,
		Currency:         req.Currency,
		Status:           "ACTIVE",
		Raw:              raw,
	}
	var envelope struct {
		Status interface{} `json:"status"`
		Data   struct {
			AccountReference string `json:"account_reference"`
			AccountName      string `json:"account_name"`
			AccountNumber    string `json:"account_number"`
			AccountNumber2   string `json:"accountNumber"`
			BankCode         string `json:"bank_code"`
			BankName         string `json:"bank_name"`
			Currency         string `json:"currency"`
			Status           string `json:"status"`
			Customer         struct {
				ID string `json:"id"`
			} `json:"customer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		response.Status = firstNonEmpty(envelope.Data.Status, providerStatusString(envelope.Status), response.Status)
		response.AccountReference = firstNonEmpty(envelope.Data.AccountReference, response.AccountReference)
		response.AccountName = firstNonEmpty(envelope.Data.AccountName, response.AccountName)
		response.AccountNumber = firstNonEmpty(envelope.Data.AccountNumber, envelope.Data.AccountNumber2)
		response.BankCode = envelope.Data.BankCode
		response.BankName = envelope.Data.BankName
		response.Currency = firstNonEmpty(envelope.Data.Currency, response.Currency)
		response.ProviderCustomerID = envelope.Data.Customer.ID
	}

	return response, nil
}

func (c *Client) GetBalances(ctx context.Context) ([]Balance, error) {
	var raw json.RawMessage
	if err := c.getSecretFirst(ctx, []string{
		"/merchant/api/v1/balances",
		"/merchant/api/v1/balance",
	}, &raw); err != nil {
		return nil, err
	}

	var envelope struct {
		Data []struct {
			Currency         string      `json:"currency"`
			AvailableBalance interface{} `json:"available_balance"`
			Balance          interface{} `json:"balance"`
			PendingBalance   interface{} `json:"pending_balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}

	balances := make([]Balance, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		availableRaw := valueToString(firstNonNil(item.AvailableBalance, item.Balance))
		pendingRaw := valueToString(item.PendingBalance)
		available, _ := money.ParseDecimal(defaultDecimal(availableRaw))
		pending, _ := money.ParseDecimal(defaultDecimal(pendingRaw))
		balances = append(balances, Balance{
			Currency:         item.Currency,
			AvailableBalance: available,
			PendingBalance:   pending,
			RawAvailable:     availableRaw,
			RawPending:       pendingRaw,
		})
	}
	return balances, nil
}

func (c *Client) RequestRefund(ctx context.Context, req RefundRequest) (*RefundResponse, error) {
	payload := map[string]interface{}{
		"reference":         req.Reference,
		"payment_reference": req.PaymentReference,
		"amount":            json.Number(req.Amount.String()),
		"currency":          req.Currency,
		"reason":            req.Reason,
	}

	var raw json.RawMessage
	if err := c.postSecretFirst(ctx, []string{
		"/merchant/api/v1/transactions/refund",
		"/merchant/api/v1/transactions/refunds",
		"/merchant/api/v1/refunds",
		"/merchant/api/v1/transactions/reverse",
	}, payload, &raw); err != nil {
		return nil, err
	}

	response := &RefundResponse{
		Reference: req.Reference,
		Status:    "processing",
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
		response.Status = firstNonEmpty(envelope.Data.Status, providerStatusString(envelope.Status), response.Status)
		response.ExternalReference = envelope.Data.TraceID
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
	if err := c.getPublic(ctx, path, &envelope); err != nil {
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
	if err := c.postPublic(ctx, "/merchant/api/v1/misc/banks/resolve", req, &envelope); err != nil {
		return nil, err
	}
	if !envelope.Status {
		return nil, fmt.Errorf("kora resolve bank account failed: %s", envelope.Message)
	}
	return &envelope.Data, nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.getSecret(ctx, path, out)
}

func (c *Client) post(ctx context.Context, path string, payload interface{}, out interface{}) error {
	return c.postSecret(ctx, path, payload, out)
}

func (c *Client) getPublic(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out, c.publicKey, "kora public key")
}

func (c *Client) postPublic(ctx context.Context, path string, payload interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, payload, out, c.publicKey, "kora public key")
}

func (c *Client) getSecret(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out, c.secretKey, "kora secret key")
}

func (c *Client) postSecret(ctx context.Context, path string, payload interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, payload, out, c.secretKey, "kora secret key")
}

func (c *Client) getSecretFirst(ctx context.Context, paths []string, out interface{}) error {
	var lastErr error
	for _, path := range paths {
		err := c.getSecret(ctx, path, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !canTryAlternateKoraPath(err) {
			return err
		}
	}
	return lastErr
}

func (c *Client) postSecretFirst(ctx context.Context, paths []string, payload interface{}, out interface{}) error {
	var lastErr error
	for _, path := range paths {
		err := c.postSecret(ctx, path, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !canTryAlternateKoraPath(err) {
			return err
		}
	}
	return lastErr
}

func (c *Client) do(ctx context.Context, method, requestPath string, payload interface{}, out interface{}, authKey, authName string) error {
	if authKey == "" {
		return fmt.Errorf("%s is not configured", authName)
	}

	var body io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}

	fullURL := c.buildURL(requestPath)
	request, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+authKey)
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
		return &HTTPError{StatusCode: response.StatusCode, Body: string(raw)}
	}

	return json.Unmarshal(raw, out)
}

func (c *Client) buildURL(requestPath string) string {
	if c.base == nil {
		return c.baseURL + requestPath
	}

	u := *c.base
	pathOnly := strings.TrimPrefix(requestPath, "/")
	rawQuery := ""
	if parsedPath, err := url.Parse(requestPath); err == nil {
		pathOnly = strings.TrimPrefix(parsedPath.Path, "/")
		rawQuery = parsedPath.RawQuery
	}
	if strings.HasSuffix(strings.Trim(u.Path, "/"), "merchant/api/v1") && strings.HasPrefix(pathOnly, "merchant/api/v1/") {
		pathOnly = strings.TrimPrefix(pathOnly, "merchant/api/v1/")
	}
	if strings.HasSuffix(strings.Trim(u.Path, "/"), "merchant") && strings.HasPrefix(pathOnly, "merchant/") {
		pathOnly = strings.TrimPrefix(pathOnly, "merchant/")
	}
	u.Path = pathpkg.Join(u.Path, pathOnly)
	u.RawQuery = rawQuery
	return u.String()
}

func canTryAlternateKoraPath(err error) bool {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.StatusCode == http.StatusNotFound || httpErr.StatusCode == http.StatusMethodNotAllowed
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

func valueToString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.2f", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	default:
		return ""
	}
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func defaultDecimal(value string) string {
	if strings.TrimSpace(value) == "" {
		return "0"
	}
	return value
}

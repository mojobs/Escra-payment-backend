package quidax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://openapi.quidax.io/exchange-open-api/v1"

type Client struct {
	baseURL    string
	secretKey  string
	httpClient *http.Client
}

func NewClient(baseURL, secretKey string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type WithdrawRequest struct {
	UserID          string `json:"-"`
	Currency        string `json:"currency"`
	Amount          string `json:"amount"`
	FundUID         string `json:"fund_uid"`
	FundUID2        string `json:"fund_uid2,omitempty"`
	TransactionNote string `json:"transaction_note,omitempty"`
	Narration       string `json:"narration,omitempty"`
	Network         string `json:"network,omitempty"`
	Reference       string `json:"reference"`
}

type WithdrawResponse struct {
	Reference         string          `json:"reference"`
	Status            string          `json:"status"`
	ExternalReference string          `json:"external_reference,omitempty"`
	Raw               json.RawMessage `json:"raw"`
}

type WithdrawalDetail struct {
	ID        string          `json:"id"`
	Reference string          `json:"reference"`
	Status    string          `json:"status"`
	Currency  string          `json:"currency"`
	Amount    string          `json:"amount"`
	Fee       string          `json:"fee"`
	Raw       json.RawMessage `json:"raw"`
}

func (c *Client) Withdraw(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, error) {
	userID := req.UserID
	if userID == "" {
		userID = "me"
	}

	var raw json.RawMessage
	path := fmt.Sprintf("/users/%s/withdraws", url.PathEscape(userID))
	if err := c.post(ctx, path, req, &raw); err != nil {
		return nil, err
	}

	response := &WithdrawResponse{
		Reference: req.Reference,
		Raw:       raw,
	}

	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			ID        string `json:"id"`
			Reference string `json:"reference"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		response.Status = firstNonEmpty(envelope.Data.Status, envelope.Status)
		response.ExternalReference = envelope.Data.ID
		if envelope.Data.Reference != "" {
			response.Reference = envelope.Data.Reference
		}
	}

	return response, nil
}

func (c *Client) FetchWithdrawal(ctx context.Context, userID, withdrawalID string) (*WithdrawalDetail, error) {
	if userID == "" {
		userID = "me"
	}

	var raw json.RawMessage
	path := fmt.Sprintf("/users/%s/withdraws/%s", url.PathEscape(userID), url.PathEscape(withdrawalID))
	if err := c.get(ctx, path, &raw); err != nil {
		return nil, err
	}

	detail := &WithdrawalDetail{Raw: raw}
	var envelope struct {
		Data struct {
			ID        string `json:"id"`
			Reference string `json:"reference"`
			Status    string `json:"status"`
			Currency  string `json:"currency"`
			Amount    string `json:"amount"`
			Fee       string `json:"fee"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		detail.ID = envelope.Data.ID
		detail.Reference = envelope.Data.Reference
		detail.Status = envelope.Data.Status
		detail.Currency = envelope.Data.Currency
		detail.Amount = envelope.Data.Amount
		detail.Fee = envelope.Data.Fee
	}
	return detail, nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) post(ctx context.Context, path string, payload interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, payload, out)
}

func (c *Client) do(ctx context.Context, method, path string, payload interface{}, out interface{}) error {
	if c.secretKey == "" {
		return fmt.Errorf("quidax secret key is not configured")
	}

	var body io.Reader
	if payload != nil {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
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
		return fmt.Errorf("quidax request failed with status %d: %s", response.StatusCode, string(raw))
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

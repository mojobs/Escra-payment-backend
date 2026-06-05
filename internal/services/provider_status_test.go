package services

import (
	"encoding/json"
	"testing"
)

func TestNormalizeProviderStatus(t *testing.T) {
	tests := map[string]string{
		"success":    "SUCCESS",
		"Done":       "SUCCESS",
		"Processing": "PROCESSING",
		"Rejected":   "FAILED",
		"custom":     "CUSTOM",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := normalizeProviderStatus(input); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestProviderStatusFromWebhook(t *testing.T) {
	payload := json.RawMessage(`{"event":"withdraw.rejected","data":{"reference":"CRYPTO-123","status":"Rejected"}}`)
	got := providerStatusFromWebhook(WebhookInput{
		Provider:  "quidax",
		EventType: "withdraw.rejected",
		Reference: "CRYPTO-123",
		Payload:   payload,
	})
	if got != "FAILED" {
		t.Fatalf("got %q, want FAILED", got)
	}

	got = providerStatusFromWebhook(WebhookInput{
		Provider:  "kora",
		EventType: "payout",
		Reference: "KORA-123",
		Payload:   json.RawMessage(`{"data":{"status":"success"}}`),
	})
	if got != "SUCCESS" {
		t.Fatalf("got %q, want SUCCESS", got)
	}
}

package kora

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mojobs/lara-payment-backend.git/pkg/money"
)

func TestRequestPayout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchant/api/v1/transactions/disburse" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header %q", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["reference"] != "LARA-123" {
			t.Fatalf("unexpected reference %v", payload["reference"])
		}
		destination := payload["destination"].(map[string]interface{})
		if destination["amount"] != 123.45 {
			t.Fatalf("unexpected amount %v", destination["amount"])
		}
		if destination["currency"] != "NGN" {
			t.Fatalf("unexpected currency %v", destination["currency"])
		}
		customer := destination["customer"].(map[string]interface{})
		if customer["email"] != "customer@example.com" {
			t.Fatalf("unexpected customer email %v", customer["email"])
		}
		bankAccount := destination["bank_account"].(map[string]interface{})
		if bankAccount["bank"] != "044" || bankAccount["account"] != "0123456789" {
			t.Fatalf("unexpected bank account %v", bankAccount)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"reference":"LARA-123","status":"success","trace_id":"trace-1"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "public", "secret")
	got, err := client.RequestPayout(context.Background(), PayoutRequest{
		Reference: "LARA-123",
		Amount:    money.FromMinorUnits(12345),
		Currency:  "NGN",
		BankCode:  "044",
		AccountNo: "0123456789",
		Email:     "customer@example.com",
	})
	if err != nil {
		t.Fatalf("request payout: %v", err)
	}
	if got.Reference != "LARA-123" {
		t.Fatalf("got reference %q", got.Reference)
	}
	if got.Status != "success" {
		t.Fatalf("got status %q", got.Status)
	}
	if got.ExternalReference != "trace-1" {
		t.Fatalf("got external reference %q", got.ExternalReference)
	}
}

func TestListBanks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchant/api/v1/misc/banks" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("countryCode"); got != "NG" {
			t.Fatalf("unexpected countryCode %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"success","data":[{"name":"Access Bank","slug":"access","code":"044","country":"NG"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret", "secret")
	banks, err := client.ListBanks(context.Background(), "ng")
	if err != nil {
		t.Fatalf("list banks: %v", err)
	}
	if len(banks) != 1 {
		t.Fatalf("got %d banks", len(banks))
	}
	if banks[0].Code != "044" {
		t.Fatalf("got bank code %q", banks[0].Code)
	}
}

func TestResolveBankAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchant/api/v1/misc/banks/resolve" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["bank"] != "044" || payload["account"] != "0123456789" || payload["currency"] != "NG" {
			t.Fatalf("unexpected payload %v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"Request Completed","data":{"bank_name":"Access Bank","bank_code":"044","account_number":"0123456789","account_name":"Ada Lovelace"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret", "secret")
	account, err := client.ResolveBankAccount(context.Background(), ResolveBankAccountRequest{
		BankCode:      "044",
		AccountNumber: "0123456789",
		Currency:      "NG",
	})
	if err != nil {
		t.Fatalf("resolve bank account: %v", err)
	}
	if account.AccountName != "Ada Lovelace" {
		t.Fatalf("got account name %q", account.AccountName)
	}
}

func TestInitializeCheckout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/merchant/api/v1/charges/initialize" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header %q", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["reference"] != "ESCROW-123" {
			t.Fatalf("unexpected reference %v", payload["reference"])
		}
		if payload["notification_url"] != "https://example.com/webhooks/kora" {
			t.Fatalf("unexpected notification url %v", payload["notification_url"])
		}
		if payload["narration"] != "Escrow checkout pay now" {
			t.Fatalf("unexpected narration %v", payload["narration"])
		}
		if _, ok := payload["description"]; ok {
			t.Fatalf("checkout payload should use narration, got description in %v", payload)
		}
		customer := payload["customer"].(map[string]interface{})
		if customer["email"] != "buyer@example.com" {
			t.Fatalf("unexpected customer email %v", customer["email"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"data":{"reference":"ESCROW-123","checkout_url":"https://checkout.kora/abc","status":"pending"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "public", "secret")
	got, err := client.InitializeCheckout(context.Background(), CheckoutRequest{
		Reference:         "ESCROW-123",
		Amount:            money.FromMinorUnits(150000),
		Currency:          "NGN",
		NotificationURL:   "https://example.com/webhooks/kora",
		RedirectURL:       "https://example.com/return",
		CustomerEmail:     "buyer@example.com",
		CustomerName:      "Buyer Demo",
		CustomerPhone:     "08100000000",
		MerchantBearsCost: true,
		Narration:         "Escrow checkout: pay now!",
	})
	if err != nil {
		t.Fatalf("initialize checkout: %v", err)
	}
	if got.Reference != "ESCROW-123" {
		t.Fatalf("got reference %q", got.Reference)
	}
	if got.CheckoutURL != "https://checkout.kora/abc" {
		t.Fatalf("got checkout url %q", got.CheckoutURL)
	}
	if got.Status != "pending" {
		t.Fatalf("got status %q", got.Status)
	}
}

func TestBuildURLDoesNotDuplicateMerchantAPIPrefix(t *testing.T) {
	client := NewClient("https://api.korapay.com/merchant/api/v1", "public", "secret")

	got := client.buildURL("/merchant/api/v1/charges/initialize")
	want := "https://api.korapay.com/merchant/api/v1/charges/initialize"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSanitizeKorapayNarration(t *testing.T) {
	got := sanitizeKorapayNarration(" Escrow checkout: pay now! ")
	if got != "Escrow checkout pay now" {
		t.Fatalf("got %q", got)
	}

	got = sanitizeKorapayNarration("")
	if got != "ESCRA Payment" {
		t.Fatalf("got default narration %q", got)
	}
}

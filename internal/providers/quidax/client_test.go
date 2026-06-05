package quidax

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithdraw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/user-1/withdraws" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header %q", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["reference"] != "CRYPTO-123" {
			t.Fatalf("unexpected reference %v", payload["reference"])
		}
		if payload["amount"] != "5.25" {
			t.Fatalf("unexpected amount %v", payload["amount"])
		}
		if _, ok := payload["UserID"]; ok {
			t.Fatalf("UserID should not be sent in request body")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":"wd-1","reference":"CRYPTO-123","status":"processing"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret")
	got, err := client.Withdraw(context.Background(), WithdrawRequest{
		UserID:    "user-1",
		Currency:  "usdt",
		Amount:    "5.25",
		FundUID:   "wallet-address",
		Network:   "trc20",
		Reference: "CRYPTO-123",
	})
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if got.Reference != "CRYPTO-123" {
		t.Fatalf("got reference %q", got.Reference)
	}
	if got.Status != "processing" {
		t.Fatalf("got status %q", got.Status)
	}
	if got.ExternalReference != "wd-1" {
		t.Fatalf("got external reference %q", got.ExternalReference)
	}
}

func TestFetchWithdrawal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/me/withdraws/wd-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"id":"wd-1","reference":"CRYPTO-123","status":"Done","currency":"usdt","amount":"5.25","fee":"1.0"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret")
	got, err := client.FetchWithdrawal(context.Background(), "me", "wd-1")
	if err != nil {
		t.Fatalf("fetch withdrawal: %v", err)
	}
	if got.ID != "wd-1" {
		t.Fatalf("got id %q", got.ID)
	}
	if got.Status != "Done" {
		t.Fatalf("got status %q", got.Status)
	}
	if got.Amount != "5.25" {
		t.Fatalf("got amount %q", got.Amount)
	}
}

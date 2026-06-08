package controllers

import (
	"testing"
)

func TestParseWebhookEnvelope(t *testing.T) {
	body := []byte(`{
		"event": "transfer.success",
		"data": {
			"reference": "TRX123",
			"status": "success"
		}
	}`)

	got := parseWebhookEnvelope(body)
	if got.EventType != "transfer.success" {
		t.Fatalf("got event type %q", got.EventType)
	}
	if got.Reference != "TRX123" {
		t.Fatalf("got reference %q", got.Reference)
	}
	if got.EventID != "TRX123" {
		t.Fatalf("got event id %q", got.EventID)
	}
}

func TestParseWebhookEnvelopeInvalidJSON(t *testing.T) {
	got := parseWebhookEnvelope([]byte(`not json`))
	if got.EventID != "" || got.EventType != "" || got.Reference != "" {
		t.Fatalf("expected empty envelope, got %+v", got)
	}
}

func TestVerifyKoraSignature(t *testing.T) {
	body := []byte(`{"event":"charge.success","data":{"reference":"TRX123","status":"success"}}`)
	signature := hmacSHA256Hex(body, "secret")

	if !verifyKoraSignature(body, signature, "secret") {
		t.Fatalf("expected raw body signature to verify")
	}
	if !verifyKoraSignature(body, "sha256="+signature, "secret") {
		t.Fatalf("expected prefixed raw body signature to verify")
	}
	if verifyKoraSignature(body, "bad", "secret") {
		t.Fatalf("expected bad signature to fail")
	}
}

func TestVerifyKoraSignatureDataFallback(t *testing.T) {
	body := []byte(`{"event":"charge.success","data":{"reference":"TRX123","status":"success"}}`)
	canonical, err := canonicalJSON([]byte(`{"reference":"TRX123","status":"success"}`))
	if err != nil {
		t.Fatalf("canonical json: %v", err)
	}
	signature := hmacSHA256Hex(canonical, "secret")

	if !verifyKoraSignature(body, signature, "secret") {
		t.Fatalf("expected canonical data signature to verify")
	}
}

func TestVerifyQuidaxSignature(t *testing.T) {
	body := []byte(`{"event":"withdraw.successful","data":{"id":"wd-1","reference":"QDX-123"}}`)
	canonical, err := canonicalJSON(body)
	if err != nil {
		t.Fatalf("canonical json: %v", err)
	}
	signature := hmacSHA256Hex([]byte("12345."+string(canonical)), "secret")

	if !verifyQuidaxSignature(body, "t=12345,v1="+signature, "secret") {
		t.Fatalf("expected signature to verify")
	}
	if verifyQuidaxSignature(body, "t=12345,v1=bad", "secret") {
		t.Fatalf("expected bad signature to fail")
	}
}

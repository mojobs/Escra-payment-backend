package services

import (
	"regexp"
	"testing"
)

func TestGenerateDeliveryCode(t *testing.T) {
	code, err := generateDeliveryCode()
	if err != nil {
		t.Fatalf("generate delivery code: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %q", code)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
		t.Fatalf("delivery code should be numeric, got %q", code)
	}
}

func TestEscrowReferenceHelpers(t *testing.T) {
	if got := escrowWalletReference("ngn"); got != "ESCROW_NGN" {
		t.Fatalf("got escrow wallet reference %q", got)
	}
	if got := escrowReference(); len(got) < len("ESC-") || got[:4] != "ESC-" {
		t.Fatalf("got escrow reference %q", got)
	}
	if got := ledgerReference("ESCROW_RELEASE"); len(got) < len("ESCROW_RELEASE-") || got[:14] != "ESCROW_RELEASE" {
		t.Fatalf("got ledger reference %q", got)
	}
}

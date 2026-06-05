package services

import "testing"

func TestRequestHash(t *testing.T) {
	first := RequestHash([]byte(`{"amount":1000}`))
	second := RequestHash([]byte(`{"amount":1000}`))
	third := RequestHash([]byte(`{"amount":1001}`))

	if first == "" {
		t.Fatalf("expected non-empty hash")
	}
	if first != second {
		t.Fatalf("same request body should produce same hash")
	}
	if first == third {
		t.Fatalf("different request body should produce different hash")
	}
}

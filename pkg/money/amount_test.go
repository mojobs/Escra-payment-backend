package money

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Amount
		wantErr bool
	}{
		{name: "whole naira", input: "1000", want: 100000},
		{name: "naira and kobo", input: "1000.50", want: 100050},
		{name: "single fraction digit", input: "10.5", want: 1050},
		{name: "too many fraction digits", input: "10.555", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDecimal(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAmountJSON(t *testing.T) {
	var amount Amount
	if err := json.Unmarshal([]byte(`"1500.75"`), &amount); err != nil {
		t.Fatalf("unexpected decimal string error: %v", err)
	}
	if amount != 150075 {
		t.Fatalf("got %d, want 150075", amount)
	}

	if err := json.Unmarshal([]byte(`150075`), &amount); err != nil {
		t.Fatalf("unexpected integer minor-unit error: %v", err)
	}
	if amount != 150075 {
		t.Fatalf("got %d, want 150075", amount)
	}

	if err := json.Unmarshal([]byte(`1500.75`), &amount); err == nil {
		t.Fatalf("expected error for decimal JSON number")
	}
}

func TestAmountScanLegacyScaleDecimalError(t *testing.T) {
	var amount Amount
	err := amount.Scan("0.0000")
	if err == nil {
		t.Fatalf("expected legacy schema error")
	}
	if !strings.Contains(err.Error(), "legacy decimal money value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

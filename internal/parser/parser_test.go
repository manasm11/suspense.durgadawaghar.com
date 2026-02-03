package parser

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	input := `Dec 26 BABA MEDICAL AND GENERAL STOR SHAMBHUA 11744.00
ICICI 192105002017 11744.00
Chq.704339 Dt. 26-12-2025 Ag. DDG024782

Dec 26 SANDHYA MEDICAL STORE LUCKNOW 5000.00
UPI/9450852076@YBL 5000.00

Dec 27 SUSPENSE A/C 1000.00
HDFC 123456789 1000.00`

	transactions := Parse(input, 2025)

	// Should have 2 transactions (SUSPENSE A/C should be skipped)
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
	}

	// Check first transaction
	if len(transactions) > 0 {
		tx := transactions[0]
		if tx.PartyName != "BABA MEDICAL AND GENERAL STOR" {
			t.Errorf("Expected party name 'BABA MEDICAL AND GENERAL STOR', got '%s'", tx.PartyName)
		}
		if tx.Location != "SHAMBHUA" {
			t.Errorf("Expected location 'SHAMBHUA', got '%s'", tx.Location)
		}
		if tx.Amount != 11744.00 {
			t.Errorf("Expected amount 11744.00, got %f", tx.Amount)
		}
		if tx.Date.Day() != 26 || tx.Date.Month() != time.December {
			t.Errorf("Expected Dec 26, got %v", tx.Date)
		}
		if tx.PaymentMode != "CHEQUE" {
			t.Errorf("Expected payment mode 'CHEQUE', got '%s'", tx.PaymentMode)
		}
	}

	// Check second transaction
	if len(transactions) > 1 {
		tx := transactions[1]
		if tx.PartyName != "SANDHYA MEDICAL STORE" {
			t.Errorf("Expected party name 'SANDHYA MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "LUCKNOW" {
			t.Errorf("Expected location 'LUCKNOW', got '%s'", tx.Location)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Expected payment mode 'UPI', got '%s'", tx.PaymentMode)
		}
	}
}

func TestParseSkipsSuspenseAC(t *testing.T) {
	input := `Dec 26 SUSPENSE A/C 1000.00
HDFC 123456789 1000.00`

	transactions := Parse(input, 2025)

	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions (SUSPENSE A/C should be skipped), got %d", len(transactions))
	}
}

func TestParseSkipsSubTotal(t *testing.T) {
	input := `Dec 26 MEDICAL STORE DELHI 5000.00
HDFC 123456789 5000.00
SUB TOTAL 5000.00`

	transactions := Parse(input, 2025)

	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}
}

func TestParseRemovesInvoiceRef(t *testing.T) {
	input := `Dec 26 MEDICAL STORE DELHI 5000.00
HDFC 123456789 5000.00
Chq.123 Dt. 26-12-2025 Ag. DDG024782`

	transactions := Parse(input, 2025)

	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}

	// Narration should not contain the invoice reference
	if len(transactions) > 0 {
		if contains(transactions[0].Narration, "DDG024782") {
			t.Error("Narration should not contain invoice reference")
		}
	}
}

func TestParsePartyNameLocation(t *testing.T) {
	tests := []struct {
		input        string
		wantName     string
		wantLocation string
	}{
		{"BABA MEDICAL STORE DELHI", "BABA MEDICAL STORE", "DELHI"},
		{"SANDHYA MEDICAL LUCKNOW", "SANDHYA MEDICAL", "LUCKNOW"},
		{"SIMPLE STORE", "SIMPLE", "STORE"},
		{"STORE MUMBAI", "STORE", "MUMBAI"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, location := parsePartyNameLocation(tt.input)
			if name != tt.wantName {
				t.Errorf("parsePartyNameLocation() name = %v, want %v", name, tt.wantName)
			}
			if location != tt.wantLocation {
				t.Errorf("parsePartyNameLocation() location = %v, want %v", location, tt.wantLocation)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDetectPaymentMode(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      string
	}{
		// UPI patterns
		{
			name:      "UPI at start",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      "UPI",
		},
		{
			name:      "UPI in middle",
			narration: "PAYMENT/UPI/REF123",
			want:      "UPI",
		},
		{
			name:      "UPI at end",
			narration: "PAYMENT FROM/UPI",
			want:      "UPI",
		},
		// IMPS patterns
		{
			name:      "IMPS with slash",
			narration: "IMPS/450912345678/PAYMENT",
			want:      "IMPS",
		},
		{
			name:      "MMT/IMPS pattern",
			narration: "MMT/IMPS/450912345678/PAYMENT",
			want:      "IMPS",
		},
		// NEFT patterns
		{
			name:      "NEFT at start",
			narration: "NEFT-CBINH25360482077-M S VISHNOI MEDICAL STORE-0000000364324",
			want:      "NEFT",
		},
		// RTGS patterns
		{
			name:      "RTGS at start",
			narration: "RTGS-PUNBR52024122700015403-SHREE GANESH TRADERS-9876543210123",
			want:      "RTGS",
		},
		// CLG patterns
		{
			name:      "CLG at start",
			narration: "CLG/SATISH KUMAR/CHQ123456",
			want:      "CLG",
		},
		// INF patterns
		{
			name:      "INF at start",
			narration: "INF/INFT/Internal Transfer/REF123",
			want:      "INF",
		},
		{
			name:      "INFT at start",
			narration: "INFT/Internal Transfer/REF123",
			want:      "INF",
		},
		{
			name:      "INFT in middle",
			narration: "TRANSFER/INFT/REF123",
			want:      "INF",
		},
		// CHEQUE patterns
		{
			name:      "Cheque with Chq.",
			narration: "ICICI 192105002017 Chq.704339 Dt. 26-12-2025",
			want:      "CHEQUE",
		},
		{
			name:      "Cheque with CHQ",
			narration: "PAYMENT VIA CHQ 123456",
			want:      "CHEQUE",
		},
		{
			name:      "Cheque full word",
			narration: "Payment by Cheque",
			want:      "CHEQUE",
		},
		// OTHER
		{
			name:      "Unknown pattern",
			narration: "CASH PAYMENT 5000",
			want:      "OTHER",
		},
		{
			name:      "Empty narration",
			narration: "",
			want:      "OTHER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectPaymentMode(tt.narration)
			if got != tt.want {
				t.Errorf("detectPaymentMode(%q) = %q, want %q", tt.narration, got, tt.want)
			}
		})
	}
}

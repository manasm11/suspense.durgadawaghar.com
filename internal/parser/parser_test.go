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

func TestParseRemovesComplexInvoiceRef(t *testing.T) {
	// Test complex invoice references like: Ag. *DDG028429,*DDG028437,*DDG028498
	input := `May 7 AKANCHA MED STORE CHIBRAMAU 200000.00
ICICI 192105002017 200000.00
Chq.206132 Dt. 07-05-2025 Ag. *DDG028429,*DDG028437,*DDG028498,*DDG028723,*DDG029117`

	transactions := Parse(input, 2025)

	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}

	// Narration should not contain any DDG references
	if len(transactions) > 0 {
		if contains(transactions[0].Narration, "DDG028429") {
			t.Error("Narration should not contain invoice reference DDG028429")
		}
		if contains(transactions[0].Narration, "DDG029117") {
			t.Error("Narration should not contain invoice reference DDG029117")
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
		{"SIMPLE STORE", "SIMPLE STORE", ""},      // STORE is not a location
		{"STORE MUMBAI", "STORE", "MUMBAI"},
		{"PAYTM BUSINESS", "PAYTM BUSINESS", ""},  // BUSINESS is not a location
		{"ICICI POS MACHINE", "ICICI POS MACHINE", ""}, // MACHINE is not a location
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

func TestParseMultiPartyTransaction(t *testing.T) {
	// Test parsing multi-party transactions where multiple parties share a single bank entry
	input := `Apr 2 NIDHI MEDICAL STORE GEHLO 5361.00
PANKAJ MEDICAL STOERE KANPUR DEHAT 3780.00
ICICI 192105002017 9141.00
UPI/545843195657/UPI/ALOK7860855471@/PUNJAB NATIONAL/ICIB5D9264C992C4AFD848F`

	transactions := Parse(input, 2025)

	// Should have 2 transactions (both parties)
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
		for i, tx := range transactions {
			t.Logf("Transaction %d: %s %s %.2f", i, tx.PartyName, tx.Location, tx.Amount)
		}
	}

	// Check first transaction
	if len(transactions) > 0 {
		tx := transactions[0]
		if tx.PartyName != "NIDHI MEDICAL STORE" {
			t.Errorf("Expected party name 'NIDHI MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "GEHLO" {
			t.Errorf("Expected location 'GEHLO', got '%s'", tx.Location)
		}
		if tx.Amount != 5361.00 {
			t.Errorf("Expected amount 5361.00, got %f", tx.Amount)
		}
	}

	// Check second transaction
	if len(transactions) > 1 {
		tx := transactions[1]
		if tx.PartyName != "PANKAJ MEDICAL STOERE KANPUR" {
			t.Errorf("Expected party name 'PANKAJ MEDICAL STOERE KANPUR', got '%s'", tx.PartyName)
		}
		if tx.Location != "DEHAT" {
			t.Errorf("Expected location 'DEHAT', got '%s'", tx.Location)
		}
		if tx.Amount != 3780.00 {
			t.Errorf("Expected amount 3780.00, got %f", tx.Amount)
		}
		// Should inherit date from first transaction
		if tx.Date.Day() != 2 || tx.Date.Month() != 4 {
			t.Errorf("Expected Apr 2, got %v", tx.Date)
		}
	}
}

func TestParseBankAccountLine(t *testing.T) {
	// Test that bank account lines are properly handled as narration
	input := `Apr 1 UPMANYU TRADERS BIRHANA ROAD 11145.00
ICICI 192105002017 11145.00
UPI/100270440630/FOR MEDICAL/8299120242@HDFC/HDFCBANK LTD/HDFD65E8311250F4F3`

	transactions := Parse(input, 2025)

	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}

	if len(transactions) > 0 {
		tx := transactions[0]
		if tx.PartyName != "UPMANYU TRADERS BIRHANA" {
			t.Errorf("Expected party name 'UPMANYU TRADERS BIRHANA', got '%s'", tx.PartyName)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Expected payment mode 'UPI', got '%s'", tx.PaymentMode)
		}
		// Bank account line should be in narration
		if !contains(tx.Narration, "ICICI 192105002017") {
			t.Errorf("Expected narration to contain bank account info, got '%s'", tx.Narration)
		}
	}
}

func TestParseReceiptBookFormat(t *testing.T) {
	// Test the exact receipt book format from DURGA DAWA GHAR
	input := `DURGA DAWA GHAR (PARTNER)
60/33,PURANI DAL MANDI KANPUR
E-Mail : durgadawaghar2022@gmail.com
D.L.No. : UP7820B001680,UP7821B001673
GSTIN : 09AATFD8891P1Z2
RECEIPT BOOK
01-04-2025 - 30-04-2025
------------------------------------------------------------------------------
DATE PARTICULARS DEBIT CREDIT
------------------------------------------------------------------------------
Apr 1 UPMANYU TRADERS BIRHANA ROAD 11145.00
ICICI 192105002017 11145.00
UPI/100270440630/FOR MEDICAL/8299120242@HDFC/HDFCBANK LTD/HDFD65E8311250F4F3
Apr 1 AMIT MED STORE MANIMAU 1440.00
ICICI 192105002017 1440.00
UPI/AK6895300@YBL/PAYMENT FROM PH/AXIS BANK/183583307455/YBLECF59B3A8B0447AF
Apr 2 CASH 384000.00
ICICI 192105002017 384000.00
BY CASH - KANPUR - BIRHANA ROAD
Apr 2 NIDHI MEDICAL STORE GEHLO 5361.00
PANKAJ MEDICAL STOERE KANPUR DEHAT 3780.00
ICICI 192105002017 9141.00
UPI/545843195657/UPI/ALOK7860855471@/PUNJAB NATIONAL/ICIB5D9264C992C4AFD848F
Apr 3 SHRI RAM MEDICAL STORE SEKHREJ 17183.00
ICICI 192105002017 17183.00
Chq.567705 Dt. 03-04-2025 Ag. DDG034269,DDG034684,DDG034750,DDG035360,DDG035774,DDG036131,DDG036237,
,DD
------------------------------------------------------------------------------
SUB TOTAL 73494.00 73494.00
------------------------------------------------------------------------------`

	transactions := Parse(input, 2025)

	// Should have 6 transactions (including PANKAJ as separate)
	expectedCount := 6
	if len(transactions) != expectedCount {
		t.Errorf("Expected %d transactions, got %d", expectedCount, len(transactions))
		for i, tx := range transactions {
			t.Logf("Transaction %d: %s %s %.2f [%s]", i+1, tx.PartyName, tx.Location, tx.Amount, tx.PaymentMode)
		}
	}

	// Verify specific transactions
	// Note: For multi-party transactions (NIDHI + PANKAJ), the bank account and
	// narration lines go to the last party (PANKAJ), so NIDHI has empty narration
	expectedTxs := []struct {
		partyName   string
		amount      float64
		paymentMode string
	}{
		{"UPMANYU TRADERS BIRHANA", 11145.00, "UPI"},
		{"AMIT MED STORE", 1440.00, "UPI"},
		{"CASH", 384000.00, "CASH"},
		{"NIDHI MEDICAL STORE", 5361.00, "OTHER"}, // Empty narration (bank lines go to PANKAJ)
		{"PANKAJ MEDICAL STOERE KANPUR", 3780.00, "UPI"},
		{"SHRI RAM MEDICAL STORE", 17183.00, "CHEQUE"},
	}

	for i, expected := range expectedTxs {
		if i < len(transactions) {
			tx := transactions[i]
			if tx.PartyName != expected.partyName {
				t.Errorf("Transaction %d: Expected party '%s', got '%s'", i+1, expected.partyName, tx.PartyName)
			}
			if tx.Amount != expected.amount {
				t.Errorf("Transaction %d: Expected amount %.2f, got %.2f", i+1, expected.amount, tx.Amount)
			}
			if tx.PaymentMode != expected.paymentMode {
				t.Errorf("Transaction %d: Expected mode '%s', got '%s'", i+1, expected.paymentMode, tx.PaymentMode)
			}
		}
	}
}

func TestIsPartyLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"PANKAJ MEDICAL STOERE KANPUR DEHAT 3780.00", true},
		{"NIDHI MEDICAL STORE GEHLO 5361.00", true},
		{"ICICI 192105002017 11145.00", false},                      // Bank account line
		{"UPI/545843195657/UPI/ALOK7860855471@/PUNJAB NATIONAL", false}, // Narration
		{"NEFT-BARBN52025040226217799-VAIBHAV LAXMI", false},         // Narration
		{"BY CASH -KANPUR - BIRHANA ROAD MANISHA", false},            // Narration
		{"5361.00", false},                                           // Just amount
		{"STORE", false},                                             // Single word
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isPartyLine(tt.line)
			if got != tt.expected {
				t.Errorf("isPartyLine(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}

func TestParseMay2025ReceiptBook(t *testing.T) {
	// Test with actual May 2025 receipt book data
	input := `DURGA DAWA GHAR (PARTNER)
60/33,PURANI DAL MANDI KANPUR
E-Mail : durgadawaghar2022@gmail.com
D.L.No. : UP7820B001680,UP7821B001673
GSTIN : 09AATFD8891P1Z2
RECEIPT BOOK
01-05-2025 - 31-05-2025
------------------------------------------------------------------------------
DATE PARTICULARS DEBIT CREDIT
------------------------------------------------------------------------------
May 1 SHRI RAM MEDICAL STORE SEKHREJ 28214.00
ICICI 192105002017 28214.00
Chq.567719 Dt. 01-05-2025 Ag. DDG000080
May 1 PAYTM BUSINESS 555.00
ICICI 192105002017 555.00
NEFT-YESBN12025050101615715-ONE 97 COMMUNICATIONSLIMITED SETTL--001425000000
May 1 ICICI POS MACHINE 80318.18
ICICI 192105002017 80318.18
FT-MESPOS SET 10XX174556 010525
May 1 CASH 226000.00
ICICI 192105002017 226000.00
BY CASH -KANPUR - BIRHANA ROAD MANISHA
May 6 PNB 0257002100103683 460000.00
ICICI 192105002017 460000.00
RTGS-PUNBR52025050611851715-DURGA DAWA GHAR-0257002100103683-PUNB0025700
May 20 AMIT MED STORE MANIMAU 6639.00
BHAWANI MEDICAL STORE MANIMAU 1856.00
ICICI 192105002017 8495.00
UPI/514030181499/UPI/SURESHRATHORE19/CANARA BANK/ICIA72FE214318743F08A5267E9
May 29 DWIVEDI MEDICAL STORE SACHENDI 1505.00
SANTOSH CHEMIST KANPUR 246.00
ICICI 192105002017 1751.00
UPI/514934551697/UPI/PRASHANTSAVITA1/PUNJAB NATIONAL/ICI021DF69120D54A17A931
------------------------------------------------------------------------------
SUB TOTAL 803333.18 803333.18
------------------------------------------------------------------------------`

	transactions := Parse(input, 2025)

	// Verify we got the expected number of transactions
	// 1. SHRI RAM MEDICAL STORE
	// 2. PAYTM BUSINESS
	// 3. ICICI POS MACHINE
	// 4. CASH
	// 5. PNB (bank transfer - treated as party)
	// 6. AMIT MED STORE
	// 7. BHAWANI MEDICAL STORE
	// 8. DWIVEDI MEDICAL STORE
	// 9. SANTOSH CHEMIST
	expectedCount := 9
	if len(transactions) != expectedCount {
		t.Errorf("Expected %d transactions, got %d", expectedCount, len(transactions))
		for i, tx := range transactions {
			t.Logf("Transaction %d: Date=%v Party='%s' Location='%s' Amount=%.2f Mode=%s",
				i+1, tx.Date.Format("Jan 2"), tx.PartyName, tx.Location, tx.Amount, tx.PaymentMode)
		}
	}

	// Verify specific transactions
	if len(transactions) >= 1 {
		tx := transactions[0]
		if tx.PartyName != "SHRI RAM MEDICAL STORE" {
			t.Errorf("Transaction 1: Expected party 'SHRI RAM MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "SEKHREJ" {
			t.Errorf("Transaction 1: Expected location 'SEKHREJ', got '%s'", tx.Location)
		}
		if tx.Amount != 28214.00 {
			t.Errorf("Transaction 1: Expected amount 28214.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "CHEQUE" {
			t.Errorf("Transaction 1: Expected mode 'CHEQUE', got '%s'", tx.PaymentMode)
		}
	}

	// Verify PAYTM BUSINESS with NEFT
	if len(transactions) >= 2 {
		tx := transactions[1]
		if tx.PartyName != "PAYTM BUSINESS" {
			t.Errorf("Transaction 2: Expected party 'PAYTM BUSINESS', got '%s'", tx.PartyName)
		}
		if tx.Amount != 555.00 {
			t.Errorf("Transaction 2: Expected amount 555.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "NEFT" {
			t.Errorf("Transaction 2: Expected mode 'NEFT', got '%s'", tx.PaymentMode)
		}
	}

	// Verify POS MACHINE
	if len(transactions) >= 3 {
		tx := transactions[2]
		if tx.PartyName != "ICICI POS MACHINE" {
			t.Errorf("Transaction 3: Expected party 'ICICI POS MACHINE', got '%s'", tx.PartyName)
		}
		if tx.Amount != 80318.18 {
			t.Errorf("Transaction 3: Expected amount 80318.18, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "POS" {
			t.Errorf("Transaction 3: Expected mode 'POS', got '%s'", tx.PaymentMode)
		}
	}

	// Verify CASH
	if len(transactions) >= 4 {
		tx := transactions[3]
		if tx.PartyName != "CASH" {
			t.Errorf("Transaction 4: Expected party 'CASH', got '%s'", tx.PartyName)
		}
		if tx.Amount != 226000.00 {
			t.Errorf("Transaction 4: Expected amount 226000.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "CASH" {
			t.Errorf("Transaction 4: Expected mode 'CASH', got '%s'", tx.PaymentMode)
		}
	}

	// Verify PNB bank transfer
	if len(transactions) >= 5 {
		tx := transactions[4]
		if tx.PartyName != "PNB 0257002100103683" {
			t.Errorf("Transaction 5: Expected party 'PNB 0257002100103683', got '%s'", tx.PartyName)
		}
		if tx.Amount != 460000.00 {
			t.Errorf("Transaction 5: Expected amount 460000.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "RTGS" {
			t.Errorf("Transaction 5: Expected mode 'RTGS', got '%s'", tx.PaymentMode)
		}
	}

	// Verify multi-party transactions: AMIT and BHAWANI
	if len(transactions) >= 7 {
		// AMIT MED STORE
		tx := transactions[5]
		if tx.PartyName != "AMIT MED STORE" {
			t.Errorf("Transaction 6: Expected party 'AMIT MED STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "MANIMAU" {
			t.Errorf("Transaction 6: Expected location 'MANIMAU', got '%s'", tx.Location)
		}
		if tx.Amount != 6639.00 {
			t.Errorf("Transaction 6: Expected amount 6639.00, got %.2f", tx.Amount)
		}

		// BHAWANI MEDICAL STORE
		tx = transactions[6]
		if tx.PartyName != "BHAWANI MEDICAL STORE" {
			t.Errorf("Transaction 7: Expected party 'BHAWANI MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "MANIMAU" {
			t.Errorf("Transaction 7: Expected location 'MANIMAU', got '%s'", tx.Location)
		}
		if tx.Amount != 1856.00 {
			t.Errorf("Transaction 7: Expected amount 1856.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Transaction 7: Expected mode 'UPI', got '%s'", tx.PaymentMode)
		}
	}

	// Verify DWIVEDI and SANTOSH multi-party
	if len(transactions) >= 9 {
		tx := transactions[7]
		if tx.PartyName != "DWIVEDI MEDICAL STORE" {
			t.Errorf("Transaction 8: Expected party 'DWIVEDI MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "SACHENDI" {
			t.Errorf("Transaction 8: Expected location 'SACHENDI', got '%s'", tx.Location)
		}

		tx = transactions[8]
		if tx.PartyName != "SANTOSH CHEMIST" {
			t.Errorf("Transaction 9: Expected party 'SANTOSH CHEMIST', got '%s'", tx.PartyName)
		}
		if tx.Location != "KANPUR" {
			t.Errorf("Transaction 9: Expected location 'KANPUR', got '%s'", tx.Location)
		}
	}
}

func TestParseJune2025ReceiptBook(t *testing.T) {
	// Test with actual June 2025 receipt book data
	input := `DURGA DAWA GHAR (PARTNER)
60/33,PURANI DAL MANDI KANPUR
E-Mail : durgadawaghar2022@gmail.com
D.L.No. : UP7820B001680,UP7821B001673
GSTIN : 09AATFD8891P1Z2
RECEIPT BOOK
01-06-2025 - 30-06-2025
------------------------------------------------------------------------------
DATE PARTICULARS DEBIT CREDIT
------------------------------------------------------------------------------
Jun 1 PAYTM BUSINESS 89311.00
ICICI 192105002017 89311.00
NEFT-YESBN12025060103629541-ONE 97 COMMUNICATIONSLIMITED SETTL--001425000000
Jun 1 AWASTHI MED AGENCY BHAGHPUR 70000.00
ICICI 192105002017 70000.00
Chq.471571 Dt. 06-01-2026 Ag. DDG026900,DDGT000180
Jun 2 CASH 181000.00
ICICI 192105002017 181000.00
BY CASH -KANPUR - BIRHANA ROAD MANISHA
Jun 6 AMIT MED STORE MANIMAU 9658.00
BHAWANI MEDICAL STORE MANIMAU 1540.00
ICICI 192105002017 11198.00
UPI/552337359414/UPI/SURESHRATHORE19/CANARA BANK/ICID751BEB851BB458FABA20697
Jun 6 MAHASHAKTI MED STORE FATEHPUR 100000.00
ICICI 192105002017 100000.00
Chq.000577 Dt. 06-06-2025 Ag. *,*DDG00454,*DDG036896,*DDG036897,DDG001697
Jun 6 AWASTHI MED AGENCY BHAGHPUR 66000.00
ICICI 192105002017 66000.00
Chq.369800 Dt. 06-06-2025 Ag. *#DDG000413,*DDG000495
------------------------------------------------------------------------------
SUB TOTAL 517169.00 517169.00
------------------------------------------------------------------------------`

	transactions := Parse(input, 2025)

	// Expected: 7 transactions
	// 1. PAYTM BUSINESS
	// 2. AWASTHI MED AGENCY
	// 3. CASH
	// 4. AMIT MED STORE
	// 5. BHAWANI MEDICAL STORE
	// 6. MAHASHAKTI MED STORE
	// 7. AWASTHI MED AGENCY (second one)
	expectedCount := 7
	if len(transactions) != expectedCount {
		t.Errorf("Expected %d transactions, got %d", expectedCount, len(transactions))
		for i, tx := range transactions {
			t.Logf("Transaction %d: Date=%v Party='%s' Location='%s' Amount=%.2f Mode=%s Narration='%s'",
				i+1, tx.Date.Format("Jan 2"), tx.PartyName, tx.Location, tx.Amount, tx.PaymentMode, tx.Narration)
		}
	}

	// Verify PAYTM BUSINESS with NEFT
	if len(transactions) >= 1 {
		tx := transactions[0]
		if tx.PartyName != "PAYTM BUSINESS" {
			t.Errorf("Transaction 1: Expected party 'PAYTM BUSINESS', got '%s'", tx.PartyName)
		}
		if tx.Amount != 89311.00 {
			t.Errorf("Transaction 1: Expected amount 89311.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "NEFT" {
			t.Errorf("Transaction 1: Expected mode 'NEFT', got '%s'", tx.PaymentMode)
		}
	}

	// Verify AWASTHI with cheque and DDGT reference (should be removed from narration)
	if len(transactions) >= 2 {
		tx := transactions[1]
		if tx.PartyName != "AWASTHI MED AGENCY" {
			t.Errorf("Transaction 2: Expected party 'AWASTHI MED AGENCY', got '%s'", tx.PartyName)
		}
		if tx.Location != "BHAGHPUR" {
			t.Errorf("Transaction 2: Expected location 'BHAGHPUR', got '%s'", tx.Location)
		}
		if tx.Amount != 70000.00 {
			t.Errorf("Transaction 2: Expected amount 70000.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "CHEQUE" {
			t.Errorf("Transaction 2: Expected mode 'CHEQUE', got '%s'", tx.PaymentMode)
		}
		// Narration should NOT contain DDGT reference
		if contains(tx.Narration, "DDGT000180") {
			t.Errorf("Transaction 2: Narration should not contain 'DDGT000180', got '%s'", tx.Narration)
		}
	}

	// Verify CASH
	if len(transactions) >= 3 {
		tx := transactions[2]
		if tx.PartyName != "CASH" {
			t.Errorf("Transaction 3: Expected party 'CASH', got '%s'", tx.PartyName)
		}
		if tx.Amount != 181000.00 {
			t.Errorf("Transaction 3: Expected amount 181000.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "CASH" {
			t.Errorf("Transaction 3: Expected mode 'CASH', got '%s'", tx.PaymentMode)
		}
	}

	// Verify multi-party: AMIT and BHAWANI
	if len(transactions) >= 5 {
		tx := transactions[3]
		if tx.PartyName != "AMIT MED STORE" {
			t.Errorf("Transaction 4: Expected party 'AMIT MED STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "MANIMAU" {
			t.Errorf("Transaction 4: Expected location 'MANIMAU', got '%s'", tx.Location)
		}
		if tx.Amount != 9658.00 {
			t.Errorf("Transaction 4: Expected amount 9658.00, got %.2f", tx.Amount)
		}

		tx = transactions[4]
		if tx.PartyName != "BHAWANI MEDICAL STORE" {
			t.Errorf("Transaction 5: Expected party 'BHAWANI MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "MANIMAU" {
			t.Errorf("Transaction 5: Expected location 'MANIMAU', got '%s'", tx.Location)
		}
		if tx.Amount != 1540.00 {
			t.Errorf("Transaction 5: Expected amount 1540.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Transaction 5: Expected mode 'UPI', got '%s'", tx.PaymentMode)
		}
	}

	// Verify MAHASHAKTI with complex invoice ref (*,*DDG)
	if len(transactions) >= 6 {
		tx := transactions[5]
		if tx.PartyName != "MAHASHAKTI MED STORE" {
			t.Errorf("Transaction 6: Expected party 'MAHASHAKTI MED STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "FATEHPUR" {
			t.Errorf("Transaction 6: Expected location 'FATEHPUR', got '%s'", tx.Location)
		}
		if tx.Amount != 100000.00 {
			t.Errorf("Transaction 6: Expected amount 100000.00, got %.2f", tx.Amount)
		}
		// Narration should NOT contain the complex DDG references
		if contains(tx.Narration, "DDG00454") {
			t.Errorf("Transaction 6: Narration should not contain DDG refs, got '%s'", tx.Narration)
		}
	}

	// Verify second AWASTHI with *#DDG pattern
	if len(transactions) >= 7 {
		tx := transactions[6]
		if tx.PartyName != "AWASTHI MED AGENCY" {
			t.Errorf("Transaction 7: Expected party 'AWASTHI MED AGENCY', got '%s'", tx.PartyName)
		}
		if tx.Amount != 66000.00 {
			t.Errorf("Transaction 7: Expected amount 66000.00, got %.2f", tx.Amount)
		}
		// Narration should NOT contain *#DDG pattern
		if contains(tx.Narration, "DDG000413") {
			t.Errorf("Transaction 7: Narration should not contain DDG refs, got '%s'", tx.Narration)
		}
	}
}

func TestParseJuly2025ReceiptBook(t *testing.T) {
	// Test with actual July 2025 receipt book data
	// This format has some transactions without the bank account line
	input := `DURGA DAWA GHAR (PARTNER)
60/33,PURANI DAL MANDI KANPUR
E-Mail : durgadawaghar2022@gmail.com
D.L.No. : UP7820B001680,UP7821B001673
GSTIN : 09AATFD8891P1Z2
RECEIPT BOOK
01-07-2025 - 31-07-2025
------------------------------------------------------------------------------
DATE PARTICULARS DEBIT CREDIT
------------------------------------------------------------------------------
Jul 1 MR ANURAG SHARMA(PROVIMINI) KANPUR 6000.00
ICICI 192105002017 6000.00
MMT/IMPS/518211116991/OK/ANURAG SHA/HDFC BANK
Jul 1 RAM JI MEDICAL STORE KENJARI 35000.00
ICICI 192105002017 35000.00
UPI/9919375846@IBL/PAYMENT FROM PH/STATE BANK OF I/378958118211/IBL54D1AC686
Jul 1 J.K MED STORE (JAFARGANJ) FATEHPUR 111918.00
ICICI 192105002017 111918.00
NEFT-BARBN52025070146956385-J K MEDICAL STORE--54220200000128-BARB0BUPGBX
Jul 7 SUSPENSE A/C 2000.00
ICICI 192105002017 2000.00
UPI/528967984881/PAYMENT FROM PH/UMASHANKAR4444Y/INDIAN BANK/AXLF8B1D253C871
Jul 7 ANSH MEDICAL STORE FATEHPUR 10000.00
ICICI 192105002017 10000.00
UPI/518810832519/UPI/SG81818282-8@OK/AXIS BANK/AXI27CCFEDAA43F405F8A1DB4FBBE
Jul 14 PALAK MEDICAL AGENCIES BANDA 28307.00
UPI/100976122989/DURGA/7355103104@HDFC/HDFC BANK LTD/HDF08F768440A4B425BB125
Jul 14 UPMANYU TRADERS BIRHANA ROAD 4774.00
ICICI 192105002017 4774.00
NEFT-YESBN12025071405685000-ONE 97 COMMUNICATIONSLIMITED SETTL--001425000000
Jul 18 YADAV MED STORE AJGAIN 6826.00
ICICI 192105002017 6826.00
Ag. DDG010296,DDG010661
Jul 21 POLICE 2000.00
ICICI 192105002017 2000.00
UPI/520284704051/UPI/BHAIASIF853@OKI/STATE BANK OF I/ICI81512593D4A24BA6A9FF
Jul 31 JAY MAHAKALI MEDICAL STORE MAUDAHA 20246.00
UPI/521230139687/UPI/SUKHBEERDANPURA/BANK OF BARODA/AXIE22B11F268CF422B8D0B6
Jul 31 MANVI MEDICAL STORE ALIYAPUR 2000.00
ICICI 192105002017 2000.00
UPI/521204558516/UPI/KULDEEPYADAV.84/BANK OF INDIA/AXIDC84EB6EAB714B0F864FBD
------------------------------------------------------------------------------
SUB TOTAL 226071.00 226071.00
------------------------------------------------------------------------------`

	transactions := Parse(input, 2025)

	// Expected transactions:
	// 1. MR ANURAG SHARMA(PROVIMINI) KANPUR
	// 2. RAM JI MEDICAL STORE KENJARI
	// 3. J.K MED STORE (JAFARGANJ) FATEHPUR
	// 4. SUSPENSE A/C - should be SKIPPED
	// 5. ANSH MEDICAL STORE FATEHPUR
	// 6. PALAK MEDICAL AGENCIES BANDA (no bank line!)
	// 7. UPMANYU TRADERS BIRHANA ROAD
	// 8. YADAV MED STORE AJGAIN
	// 9. POLICE
	// 10. JAY MAHAKALI MEDICAL STORE MAUDAHA (no bank line!)
	// 11. MANVI MEDICAL STORE ALIYAPUR
	expectedCount := 10 // SUSPENSE A/C is skipped
	if len(transactions) != expectedCount {
		t.Errorf("Expected %d transactions, got %d", expectedCount, len(transactions))
		for i, tx := range transactions {
			t.Logf("Transaction %d: Date=%v Party='%s' Location='%s' Amount=%.2f Mode=%s Narration='%s'",
				i+1, tx.Date.Format("Jan 2"), tx.PartyName, tx.Location, tx.Amount, tx.PaymentMode, tx.Narration)
		}
	}

	// Verify first transaction - MR ANURAG SHARMA with IMPS
	if len(transactions) >= 1 {
		tx := transactions[0]
		if tx.PartyName != "MR ANURAG SHARMA(PROVIMINI)" {
			t.Errorf("Transaction 1: Expected party 'MR ANURAG SHARMA(PROVIMINI)', got '%s'", tx.PartyName)
		}
		if tx.Location != "KANPUR" {
			t.Errorf("Transaction 1: Expected location 'KANPUR', got '%s'", tx.Location)
		}
		if tx.Amount != 6000.00 {
			t.Errorf("Transaction 1: Expected amount 6000.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "IMPS" {
			t.Errorf("Transaction 1: Expected mode 'IMPS', got '%s'", tx.PaymentMode)
		}
	}

	// Verify J.K MED STORE with NEFT
	if len(transactions) >= 3 {
		tx := transactions[2]
		if tx.PartyName != "J.K MED STORE (JAFARGANJ)" {
			t.Errorf("Transaction 3: Expected party 'J.K MED STORE (JAFARGANJ)', got '%s'", tx.PartyName)
		}
		if tx.Location != "FATEHPUR" {
			t.Errorf("Transaction 3: Expected location 'FATEHPUR', got '%s'", tx.Location)
		}
		if tx.PaymentMode != "NEFT" {
			t.Errorf("Transaction 3: Expected mode 'NEFT', got '%s'", tx.PaymentMode)
		}
	}

	// Verify PALAK MEDICAL AGENCIES - no bank account line, just UPI
	if len(transactions) >= 5 {
		tx := transactions[4]
		if tx.PartyName != "PALAK MEDICAL AGENCIES" {
			t.Errorf("Transaction 5: Expected party 'PALAK MEDICAL AGENCIES', got '%s'", tx.PartyName)
		}
		if tx.Location != "BANDA" {
			t.Errorf("Transaction 5: Expected location 'BANDA', got '%s'", tx.Location)
		}
		if tx.Amount != 28307.00 {
			t.Errorf("Transaction 5: Expected amount 28307.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Transaction 5: Expected mode 'UPI', got '%s'", tx.PaymentMode)
		}
	}

	// Verify YADAV MED STORE - has only Ag. reference in narration (should be stripped)
	if len(transactions) >= 7 {
		tx := transactions[6]
		if tx.PartyName != "YADAV MED STORE" {
			t.Errorf("Transaction 7: Expected party 'YADAV MED STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "AJGAIN" {
			t.Errorf("Transaction 7: Expected location 'AJGAIN', got '%s'", tx.Location)
		}
		// Ag. reference should be stripped, leaving only bank account line
		if contains(tx.Narration, "DDG010296") {
			t.Errorf("Transaction 7: Narration should not contain DDG ref, got '%s'", tx.Narration)
		}
	}

	// Verify POLICE - single word party name
	if len(transactions) >= 8 {
		tx := transactions[7]
		if tx.PartyName != "POLICE" {
			t.Errorf("Transaction 8: Expected party 'POLICE', got '%s'", tx.PartyName)
		}
		if tx.Amount != 2000.00 {
			t.Errorf("Transaction 8: Expected amount 2000.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Transaction 8: Expected mode 'UPI', got '%s'", tx.PaymentMode)
		}
	}

	// Verify JAY MAHAKALI - no bank account line
	if len(transactions) >= 9 {
		tx := transactions[8]
		if tx.PartyName != "JAY MAHAKALI MEDICAL STORE" {
			t.Errorf("Transaction 9: Expected party 'JAY MAHAKALI MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "MAUDAHA" {
			t.Errorf("Transaction 9: Expected location 'MAUDAHA', got '%s'", tx.Location)
		}
		if tx.Amount != 20246.00 {
			t.Errorf("Transaction 9: Expected amount 20246.00, got %.2f", tx.Amount)
		}
		if tx.PaymentMode != "UPI" {
			t.Errorf("Transaction 9: Expected mode 'UPI', got '%s'", tx.PaymentMode)
		}
	}

	// Verify MANVI MEDICAL STORE
	if len(transactions) >= 10 {
		tx := transactions[9]
		if tx.PartyName != "MANVI MEDICAL STORE" {
			t.Errorf("Transaction 10: Expected party 'MANVI MEDICAL STORE', got '%s'", tx.PartyName)
		}
		if tx.Location != "ALIYAPUR" {
			t.Errorf("Transaction 10: Expected location 'ALIYAPUR', got '%s'", tx.Location)
		}
	}
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
		// POS patterns
		{
			name:      "POS FT-MESPOS",
			narration: "FT-MESPOS SET 10XX174556 010525",
			want:      "POS",
		},
		{
			name:      "POS MESPOS SET",
			narration: "ICICI 192105002017 80318.18 MESPOS SET 10XX174556",
			want:      "POS",
		},
		// CASH patterns
		{
			name:      "BY CASH",
			narration: "BY CASH -KANPUR - BIRHANA ROAD MANISHA",
			want:      "CASH",
		},
		{
			name:      "CAM cash deposit",
			narration: "CAM/40791SRY/CASH DEP-OTHER/31-05-25/1582",
			want:      "CASH",
		},
		// OTHER
		{
			name:      "Unknown pattern",
			narration: "RANDOM PAYMENT 5000",
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

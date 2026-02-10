package extractor

import (
	"testing"
)

func TestUserScenarioFromPattern(t *testing.T) {
	narration := "From:XXXX8723:ASHWANI KUMAR Ag. *DDG029160,"
	ids := Extract(narration)
	t.Logf("Extracted %d identifiers from: %s", len(ids), narration)
	for _, id := range ids {
		t.Logf("  Type: %-15s Value: %s", id.Type, id.Value)
	}

	// Verify we got the expected identifiers
	foundFromAccount := false
	foundFromName := false
	foundAgentCode := false

	for _, id := range ids {
		switch id.Type {
		case TypeFromAccount:
			if id.Value == "XXXX8723" {
				foundFromAccount = true
			}
		case TypeFromName:
			if id.Value == "ASHWANI KUMAR" {
				foundFromName = true
			}
		case TypeCashAgentCode:
			if id.Value == "DDG029160" {
				foundAgentCode = true
			}
		}
	}

	if !foundFromAccount {
		t.Error("Expected to find from_account=XXXX8723")
	}
	if !foundFromName {
		t.Error("Expected to find from_name=ASHWANI KUMAR")
	}
	if !foundAgentCode {
		t.Error("Expected to find cash_agent_code=DDG029160")
	}
}

func TestFromPatternWithSpacedName(t *testing.T) {
	// User's exact scenario: R R DRUG CENTRE (spaced name)
	narration := "From:XXXX2304:R R DRUG CENTRE"
	ids := Extract(narration)
	t.Logf("Extracted %d identifiers from: %s", len(ids), narration)
	for _, id := range ids {
		t.Logf("  Type: %-15s Value: %s", id.Type, id.Value)
	}

	foundFromAccount := false
	foundFromName := false

	for _, id := range ids {
		switch id.Type {
		case TypeFromAccount:
			if id.Value == "XXXX2304" {
				foundFromAccount = true
			}
		case TypeFromName:
			if id.Value == "R R DRUG CENTRE" {
				foundFromName = true
			}
		}
	}

	if !foundFromAccount {
		t.Error("Expected to find from_account=XXXX2304")
	}
	if !foundFromName {
		t.Error("Expected to find from_name=R R DRUG CENTRE")
	}
}

func TestExtractUPIVPA(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "UPI VPA with phone",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT FR/STATE BANK/450854353978",
			want:      []string{"9450852076@YBL"},
		},
		{
			name:      "UPI VPA with name",
			narration: "UPI/SUNEELBHADEVANA@HDFC/PAYMENT",
			want:      []string{"SUNEELBHADEVANA@HDFC"},
		},
		{
			name:      "Multiple VPAs",
			narration: "Transfer from test@paytm to user@upi",
			want:      []string{"TEST@PAYTM", "USER@UPI"},
		},
		{
			name:      "No VPA",
			narration: "NEFT transfer 12345",
			want:      nil,
		},
		{
			name:      "UPI ID from narration format (no @ symbol)",
			narration: "UPI/564031341768/UPI/ANUJ19SENGARR-3/KOTAK MAHINDRA /AXI0E9F3406C3D74904A45A",
			want:      []string{"ANUJ19SENGARR-3"},
		},
		{
			name:      "UPI ID from alternate narration format (PAYMENT FR)",
			narration: "UPI/MR MAHESH/SHRIVASMAHESH2/PAYMENT FR/BANK OF BA/464278460653/YBLE6E8037FC",
			want:      []string{"SHRIVASMAHESH2"},
		},
		{
			name:      "UPI ID first after UPI slash",
			narration: "UPI/ASHISHKUMARPAND/SHRI RADHEY KRI/BANK OF BARODA/102557916140/HDFA655BF2F2",
			want:      []string{"ASHISHKUMARPAND"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeUPIVPA)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractPhone(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Phone in UPI narration",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      []string{"9450852076"},
		},
		{
			name:      "Phone standalone",
			narration: "IMPS/450912345678/9876543210/Payment",
			want:      []string{"9876543210"},
		},
		{
			name:      "Invalid phone (starts with 5)",
			narration: "IMPS/5234567890/Payment",
			want:      nil,
		},
		{
			name:      "No phone",
			narration: "NEFT transfer from account",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypePhone)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractAccountNumber(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Account in NEFT ref at end (no trailing dash)",
			narration: "NEFT-CBINH25360482077-M S VISHNOI MEDICAL STORE-0000000364324",
			want:      []string{"0000000364324"},
		},
		{
			name:      "Account in NEFT ref with trailing dash",
			narration: "NEFT-CBINH25360482077-M S VISHNOI MEDICAL STORE-0000000364324-REF",
			want:      []string{"0000000364324"},
		},
		{
			name:      "Account number pattern",
			narration: "RTGS-HDFC0001234-COMPANY NAME-123456789012-REF",
			want:      []string{"123456789012"},
		},
		{
			name:      "No account number",
			narration: "UPI payment to user@bank",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeAccountNumber)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtract(t *testing.T) {
	narration := "UPI/SANDHYA ME/9450852076@YBL/PAYMENT FR/STATE BANK/450854353978"

	ids := Extract(narration)

	// Should find at least VPA and phone
	foundVPA := false
	foundPhone := false
	for _, id := range ids {
		if id.Type == TypeUPIVPA && id.Value == "9450852076@YBL" {
			foundVPA = true
		}
		if id.Type == TypePhone && id.Value == "9450852076" {
			foundPhone = true
		}
	}

	if !foundVPA {
		t.Error("Expected to find UPI VPA")
	}
	if !foundPhone {
		t.Error("Expected to find phone number")
	}
}

func TestExtractIMPSName(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "IMPS with OK status",
			narration: "MMT/IMPS/518211116991/OK/ANURAG SHA/HDFC BANK",
			want:      []string{"ANURAG SHA"},
		},
		{
			name:      "IMPS with two names",
			narration: "MMT/IMPS/527412932576/DURGA/AGNIHOTRIM/UNION BANKOF I",
			want:      []string{"DURGA", "AGNIHOTRIM"},
		},
		{
			name:      "Non-MMT IMPS format",
			narration: "IMPS/450912345678/9876543210/Payment",
			want:      nil,
		},
		{
			name:      "Non-IMPS narration",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
		{
			name:      "NEFT narration",
			narration: "NEFT-CBINH25360482077-M S VISHNOI MEDICAL STORE-0000000364324",
			want:      nil,
		},
		{
			name:      "IMPS with secondary reference format",
			narration: "MMT/IMPS/528819823026/50000078106642 /RAPIPAY FI/YES BANK LTD",
			want:      []string{"RAPIPAY FI"},
		},
		{
			name:      "IMPS P2A format",
			narration: "MMT/IMPS/528764057172/IMPS P2A DURGA /GUPTA MEDI/UCO BANK",
			want:      []string{"DURGA", "GUPTA MEDI"},
		},
		{
			name:      "IMPS with payment description (filters out PAYMENT suffix)",
			narration: "MMT/IMPS/529811848407/MASTODINPAYMENT/AMANPHARMA/BANK OF BARODA",
			want:      []string{"AMANPHARMA"},
		},
		{
			name:      "IMPS REQPAY format",
			narration: "MMT/IMPS/510615959587/REQPAY/NEWVI9936 /STATE BANK OF I",
			want:      []string{"NEWVI9936"},
		},
		{
			name:      "IMPS simple name/bank format",
			narration: "MMT/IMPS/534315268553/AMAR AGENC/PUNJAB AND SIND",
			want:      []string{"AMAR AGENC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeIMPSName)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractBankName(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "IMPS with OK status - HDFC",
			narration: "MMT/IMPS/518211116991/OK/ANURAG SHA/HDFC BANK",
			want:      []string{"HDFC BANK"},
		},
		{
			name:      "IMPS with two names - Union Bank normalized",
			narration: "MMT/IMPS/527412932576/DURGA/AGNIHOTRIM/UNION BANKOF I",
			want:      []string{"UNION BANK OF INDIA"},
		},
		{
			name:      "Non-MMT IMPS format",
			narration: "IMPS/450912345678/9876543210/Payment",
			want:      nil,
		},
		{
			name:      "Non-IMPS narration",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
		{
			name:      "IMPS with secondary reference format",
			narration: "MMT/IMPS/528819823026/50000078106642 /RAPIPAY FI/YES BANK LTD",
			want:      []string{"YES BANK"},
		},
		{
			name:      "IMPS P2A format",
			narration: "MMT/IMPS/528764057172/IMPS P2A DURGA /GUPTA MEDI/UCO BANK",
			want:      []string{"UCO BANK"},
		},
		{
			name:      "IMPS with payment description - Bank of Baroda normalized",
			narration: "MMT/IMPS/529811848407/MASTODINPAYMENT/AMANPHARMA/BANK OF BARODA",
			want:      []string{"BANK OF BARODA"},
		},
		{
			name:      "IMPS REQPAY format - State Bank normalized",
			narration: "MMT/IMPS/510615959587/REQPAY/NEWVI9936 /STATE BANK OF I",
			want:      []string{"STATE BANK OF INDIA"},
		},
		{
			name:      "IMPS simple format - Punjab And Sind Bank normalized",
			narration: "MMT/IMPS/534315268553/AMAR AGENC/PUNJAB AND SIND",
			want:      []string{"PUNJAB AND SIND BANK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeBankName)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractNEFTName(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "NEFT standard format",
			narration: "NEFT-UCBAN52025040104667985-SHRI SHYAM AGENCY-/FAST/// NEFT-25170210002308-U",
			want:      []string{"SHRI SHYAM AGENCY"},
		},
		{
			name:      "NEFT BARB format",
			narration: "NEFT-BARBN52025040226217799-VAIBHAV LAXMI MEDICALSTORE--37100200000337-BARB0",
			want:      []string{"VAIBHAV LAXMI MEDICALSTORE"},
		},
		{
			name:      "NEFT CNRB format with NA NA",
			narration: "NEFT-CNRBN52025040237124747-VINAY MEDICAL STORE-NA NA-86551400000375-CNRB000",
			want:      []string{"VINAY MEDICAL STORE"},
		},
		{
			name:      "NEFT CBIN format with ATTN",
			narration: "NEFT-CBINN62025040275800299-NEW PALIWAL MEDICAL STORE-//ATTN//-0000000558691",
			want:      []string{"NEW PALIWAL MEDICAL STORE"},
		},
		{
			name:      "NEFT PUNB format with bank code",
			narration: "NEFT-PUNBN62025040557735331-CHEAP PHARMA-PNB-0229002100067241-PUNB0022900",
			want:      []string{"CHEAP PHARMA"},
		},
		{
			name:      "NEFT HDFC format",
			narration: "NEFT-HDFCN52025041579938340-NARAIN MEDICAL STORE-0001-50200039309108-HDFC000",
			want:      []string{"NARAIN MEDICAL STORE"},
		},
		{
			name:      "NEFT YES BANK format",
			narration: "NEFT-YESBN12025040203209954-ONE 97 COMMUNICATIONSLIMITED SETTL--001425000000",
			want:      []string{"ONE 97 COMMUNICATIONSLIMITED SETTL"},
		},
		{
			name:      "NEFT SBIN format with ATTN and more",
			narration: "NEFT-SBINN52025040812556593-AMAR MEDICINE AND COSMETICS-/ATTN//INB//PAYMENT",
			want:      []string{"AMAR MEDICINE AND COSMETICS"},
		},
		{
			name:      "INFT format",
			narration: "INF/INFT/039939724801/DURGAKNP /S S PHARMA",
			want:      []string{"S S PHARMA"},
		},
		{
			name:      "INFT single name format",
			narration: "INF/INFT/041141036691/GAYATRI PHARMA",
			want:      []string{"GAYATRI PHARMA"},
		},
		{
			name:      "BIL/INFT format",
			narration: "BIL/INFT/EDC0857581/ SANJIT KUMAR",
			want:      []string{"SANJIT KUMAR"},
		},
		{
			name:      "NEFT_IN format with agent code",
			narration: "NEFT_IN:null//SBINN52025042334823235/VIJAY MEDICAL STORE Ag. DDG000516",
			want:      []string{"VIJAY MEDICAL STORE"},
		},
		{
			name:      "NEFT_IN format without agent code",
			narration: "NEFT_IN:null//PUNB52025050012345678/SHARMA TRADERS",
			want:      []string{"SHARMA TRADERS"},
		},
		{
			name:      "Non-NEFT narration (UPI)",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
		{
			name:      "Non-NEFT narration (IMPS)",
			narration: "MMT/IMPS/518211116991/OK/ANURAG SHA/HDFC BANK",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeNEFTName)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractIFSC(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "IFSC HDFC format",
			narration: "RTGS-HDFC0001234-COMPANY NAME-123456789012",
			want:      []string{"HDFC0001234"},
		},
		{
			name:      "IFSC SBIN format",
			narration: "NEFT-SBIN0012345-STORE NAME-0000000364324",
			want:      []string{"SBIN0012345"},
		},
		{
			name:      "Multiple IFSC codes",
			narration: "Transfer from SBIN0001234 to ICIC0002345",
			want:      []string{"SBIN0001234", "ICIC0002345"},
		},
		{
			name:      "No IFSC code",
			narration: "UPI/user@ybl/PAYMENT",
			want:      nil,
		},
		{
			name:      "Invalid IFSC (no zero at position 5)",
			narration: "ABCD1234567 is not valid",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeIFSC)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractCashBankCode(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Standard cash deposit with state",
			narration: "BY CASH -733300 TIRWA (UP) Ag. DDG000201",
			want:      []string{"733300"},
		},
		{
			name:      "Cash deposit without agent code",
			narration: "BY CASH -123456 KANPUR",
			want:      []string{"123456"},
		},
		{
			name:      "Cash deposit with longer code",
			narration: "BY CASH -1234567 LUCKNOW (UP)",
			want:      []string{"1234567"},
		},
		{
			name:      "Cash deposit without numeric bank code",
			narration: "BY CASH -KANPUR - BIRHANA ROAD MANISHA",
			want:      nil,
		},
		{
			name:      "Non-cash narration (UPI)",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
		{
			name:      "Non-cash narration (IMPS)",
			narration: "MMT/IMPS/518211116991/OK/ANURAG SHA/HDFC BANK",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeCashBankCode)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractCashLocation(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Cash deposit with state code",
			narration: "BY CASH -733300 TIRWA (UP) Ag. DDG000201",
			want:      []string{"TIRWA (UP)"},
		},
		{
			name:      "Cash deposit without state code",
			narration: "BY CASH -123456 KANPUR",
			want:      []string{"KANPUR"},
		},
		{
			name:      "Cash deposit with longer location name",
			narration: "BY CASH -1234567 LUCKNOW (UP)",
			want:      []string{"LUCKNOW (UP)"},
		},
		{
			name:      "Cash deposit without numeric bank code",
			narration: "BY CASH -KANPUR - BIRHANA ROAD MANISHA",
			want:      nil,
		},
		{
			name:      "Non-cash narration",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeCashLocation)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractCashAgentCode(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Cash deposit with Ag. prefix",
			narration: "BY CASH -733300 TIRWA (UP) Ag. DDG000201",
			want:      []string{"DDG000201"},
		},
		{
			name:      "Cash deposit with longer agent code",
			narration: "BY CASH -733300 LUCKNOW (UP) Ag. ABCD1234567890",
			want:      []string{"ABCD1234567890"},
		},
		{
			name:      "Agent code with asterisk prefix",
			narration: "From:XXXX8723:ASHWANI KUMAR Ag. *DDG029160,",
			want:      []string{"DDG029160"},
		},
		{
			name:      "Cash deposit without agent code",
			narration: "BY CASH -123456 KANPUR",
			want:      nil,
		},
		{
			name:      "Non-cash narration",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeCashAgentCode)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractFromAccount(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Standard From pattern",
			narration: "PNB 0257002100103683 30968.00\nFrom:XXXX8723:ASHWANI KUMAR",
			want:      []string{"XXXX8723"},
		},
		{
			name:      "From pattern with different account",
			narration: "From:XXXX1234:RAJESH SHARMA",
			want:      []string{"XXXX1234"},
		},
		{
			name:      "From pattern lowercase (should match due to uppercase conversion)",
			narration: "from:XXXX5678:SUNIL KUMAR",
			want:      []string{"XXXX5678"},
		},
		{
			name:      "From pattern with agent code",
			narration: "From:XXXX8723:ASHWANI KUMAR Ag. *DDG029160,",
			want:      []string{"XXXX8723"},
		},
		{
			name:      "No From pattern",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
		{
			name:      "Invalid From pattern (not enough X)",
			narration: "From:XXX8723:ASHWANI KUMAR",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeFromAccount)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractFromName(t *testing.T) {
	tests := []struct {
		name      string
		narration string
		want      []string
	}{
		{
			name:      "Standard From pattern",
			narration: "PNB 0257002100103683 30968.00\nFrom:XXXX8723:ASHWANI KUMAR",
			want:      []string{"ASHWANI KUMAR"},
		},
		{
			name:      "From pattern with single name",
			narration: "From:XXXX1234:RAJESH",
			want:      []string{"RAJESH"},
		},
		{
			name:      "From pattern with long name",
			narration: "From:XXXX5678:MOHAMMAD ABDUL RAHMAN",
			want:      []string{"MOHAMMAD ABDUL RAHMAN"},
		},
		{
			name:      "From pattern with agent code (should strip AG suffix)",
			narration: "From:XXXX8723:ASHWANI KUMAR Ag. *DDG029160,",
			want:      []string{"ASHWANI KUMAR"},
		},
		{
			name:      "No From pattern",
			narration: "UPI/SANDHYA ME/9450852076@YBL/PAYMENT",
			want:      nil,
		},
		{
			name:      "From pattern with invalid name (starts with number)",
			narration: "From:XXXX8723:123ABC",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractByType(tt.narration, TypeFromName)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractByType() got %d values %v, want %d values %v", len(got), got, len(tt.want), tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractByType()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

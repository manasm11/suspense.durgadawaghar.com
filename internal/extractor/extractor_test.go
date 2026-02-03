package extractor

import (
	"testing"
)

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

package extractor

import (
	"regexp"
	"strings"
)

// IdentifierType represents the type of identifier extracted
type IdentifierType string

const (
	TypeUPIVPA        IdentifierType = "upi_vpa"
	TypePhone         IdentifierType = "phone"
	TypeAccountNumber IdentifierType = "account_number"
	TypeIFSC          IdentifierType = "ifsc"
	TypeIMPSName      IdentifierType = "imps_name"  // Sender/receiver name from IMPS
	TypeBankName      IdentifierType = "bank_name"  // Bank name from IMPS
)

// Identifier represents an extracted identifier from a narration
type Identifier struct {
	Type  IdentifierType
	Value string
}

var (
	// UPI VPA: user@provider (e.g., 9450852076@YBL, SUNEELBHADEVANA@HDFC, ATKRISHAN12-2@O)
	upiPattern = regexp.MustCompile(`([a-zA-Z0-9][a-zA-Z0-9._-]{1,255}@[a-zA-Z]{1,64})`)

	// UPI ID from narration format: UPI/<txn_id>/UPI/<upi_id>/<bank>
	// Captures the UPI ID (e.g., ANUJ19SENGARR-3 from UPI/564031341768/UPI/ANUJ19SENGARR-3/KOTAK MAHINDRA)
	// Also handles UPI IDs with @ symbol (e.g., ATKRISHAN12-2@O from UPI/112114924711/UPI/ATKRISHAN12-2@O/HDFC BANK LTD)
	upiNarrationPattern = regexp.MustCompile(`UPI/\d+/UPI/([A-Za-z0-9._@-]+)/`)

	// UPI ID from alternate narration format: UPI/<name>/<upi_id>/PAYMENT FR/<bank>/<ref>/<provider_code>
	// Captures the UPI ID (e.g., SHRIVASMAHESH2 from UPI/MR MAHESH/SHRIVASMAHESH2/PAYMENT FR/BANK OF BA/464278460653/YBLE6E8037FC)
	upiNarrationPattern2 = regexp.MustCompile(`UPI/[^/]+/([A-Za-z0-9._-]+)/PAYMENT FR/`)

	// UPI ID from narration format: UPI/<txn_id>/<name>/<upi_id>/<location>/<ref>
	// Captures the UPI ID (e.g., RKROHITKUMAR459 from UPI/112177057693/TULSHI MEDICAL/RKROHITKUMAR459/UTTAR PRADESH G/HDF0C8DB9785)
	upiNarrationPattern3 = regexp.MustCompile(`UPI/\d+/[^/]+/([A-Za-z0-9._@-]+)/`)

	// UPI ID from narration format: UPI/<name>/<upi_id>/<other>/<bank>/<ref>/<code>
	// Captures the UPI ID (e.g., JAYANTSINGH246 from UPI/JAYANT SIN/JAYANTSINGH246/DURGA/KOTAK MAHI/564648156111/ICI7B61D9D2074F4)
	upiNarrationPattern4 = regexp.MustCompile(`UPI/[^/]+/([A-Za-z0-9._@-]+)/[^/]+/[^/]+/\d+/`)

	// Phone: 10 digits starting with 6-9
	phonePattern = regexp.MustCompile(`(?:^|[^\d])([6-9]\d{9})(?:[^\d]|$)`)

	// Account Number: 9-18 digits in NEFT/RTGS refs (pattern like -ACCOUNTNUMBER- or -ACCOUNTNUMBER at end)
	accountPattern = regexp.MustCompile(`-(\d{9,18})(?:-|$)`)

	// Additional account pattern for standalone account numbers in specific contexts
	accountPatternAlt = regexp.MustCompile(`(?:A/C|ACCT?|Account)\s*(?:No\.?|#)?\s*(\d{9,18})`)

	// IFSC Code: 4 letters + 0 + 6 alphanumeric characters
	ifscPattern = regexp.MustCompile(`[A-Z]{4}0[A-Z0-9]{6}`)

	// IMPS patterns for extracting names and bank
	// MMT/IMPS/<ref>/OK/<name>/<bank> - status OK format
	impsOKPattern = regexp.MustCompile(`MMT/IMPS/\d{12}/OK/([^/]+)/(.+)`)
	// MMT/IMPS/<ref>/<name1>/<name2>/<bank> - two names format (name1 could be sender, name2 receiver or vice versa)
	impsTwoNamesPattern = regexp.MustCompile(`MMT/IMPS/\d{12}/([A-Z][A-Z\s]*)/([A-Z][A-Z\s]*)/(.+)`)
	// MMT/IMPS/<ref>/<secondary_ref> /<name>/<bank> - secondary reference format with space before slash
	impsSecondaryRefPattern = regexp.MustCompile(`MMT/IMPS/\d{12}/\d+\s*/([^/]+)/(.+)`)
	// MMT/IMPS/<ref>/IMPS P2A <sender> /<receiver>/<bank> - P2A (Person to Account) format
	impsP2APattern = regexp.MustCompile(`MMT/IMPS/\d{12}/IMPS P2A\s+([^/]+?)\s*/([^/]+)/(.+)`)
)

// bankNormalization maps truncated bank names to full names
var bankNormalization = map[string]string{
	"UNION BANKOF I":    "UNION BANK OF INDIA",
	"STATE BANK O":      "STATE BANK OF INDIA",
	"BANK OF BARO":      "BANK OF BARODA",
	"PUNJAB NATIO":      "PUNJAB NATIONAL BANK",
	"CANARA BANK":       "CANARA BANK",
	"HDFC BANK":         "HDFC BANK",
	"ICICI BANK":        "ICICI BANK",
	"AXIS BANK":         "AXIS BANK",
	"KOTAK MAHIND":      "KOTAK MAHINDRA BANK",
	"INDUSIND BAN":      "INDUSIND BANK",
	"YES BANK":          "YES BANK",
	"IDBI BANK":         "IDBI BANK",
	"CENTRAL BANK":      "CENTRAL BANK OF INDIA",
	"INDIAN BANK":       "INDIAN BANK",
	"INDIAN OVERS":      "INDIAN OVERSEAS BANK",
	"UCO BANK":          "UCO BANK",
	"BANK OF INDI":      "BANK OF INDIA",
	"SYNDICATE BA":      "SYNDICATE BANK",
	"ALLAHABAD BA":      "ALLAHABAD BANK",
	"CORPORATION":       "CORPORATION BANK",
	"ORIENTAL BAN":      "ORIENTAL BANK OF COMMERCE",
	"UNITED BANK":       "UNITED BANK OF INDIA",
	"DENA BANK":         "DENA BANK",
	"VIJAYA BANK":       "VIJAYA BANK",
	"FEDERAL BANK":      "FEDERAL BANK",
	"SOUTH INDIAN":      "SOUTH INDIAN BANK",
	"KARNATAKA BA":      "KARNATAKA BANK",
	"BANDHAN BANK":      "BANDHAN BANK",
	"RBL BANK":          "RBL BANK",
	"IDFC FIRST B":      "IDFC FIRST BANK",
	"AU SMALL FIN":      "AU SMALL FINANCE BANK",
	"EQUITAS SMAL":      "EQUITAS SMALL FINANCE BANK",
	"UJJIVAN SMAL":      "UJJIVAN SMALL FINANCE BANK",
	"PAYTM PAYMEN":      "PAYTM PAYMENTS BANK",
	"AIRTEL PAYME":      "AIRTEL PAYMENTS BANK",
	"FINO PAYMENT":      "FINO PAYMENTS BANK",
	"JIOPAYMENTSB":      "JIO PAYMENTS BANK",
}

// normalizeBank normalizes truncated bank names to full names
func normalizeBank(raw string) string {
	raw = strings.TrimSpace(raw)
	// Try exact match first
	if normalized, ok := bankNormalization[raw]; ok {
		return normalized
	}
	// Try prefix match for even more truncated names
	for truncated, full := range bankNormalization {
		if strings.HasPrefix(truncated, raw) || strings.HasPrefix(raw, truncated) {
			return full
		}
	}
	return raw
}

// isValidIMPSName checks if the extracted name is valid (not a status code or payment description)
func isValidIMPSName(name string) bool {
	name = strings.TrimSpace(name)
	if len(name) < 2 {
		return false
	}
	// Reject status codes
	statusCodes := []string{"OK", "NA", "NULL", "FAIL", "ERROR", "PENDING", "SUCCESS"}
	for _, code := range statusCodes {
		if name == code {
			return false
		}
	}
	// Reject payment descriptions (e.g., "MASTODINPAYMENT", "XYZPAYMENT")
	if strings.HasSuffix(strings.ToUpper(name), "PAYMENT") {
		return false
	}
	// Names should contain at least one letter
	hasLetter := false
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			hasLetter = true
			break
		}
	}
	return hasLetter
}

// extractIMPSData extracts names and bank from IMPS narrations
func extractIMPSData(narration string) (names []string, bank string) {
	upperNarration := strings.ToUpper(narration)

	// Try MMT/IMPS/ref/OK/name/bank pattern first
	if matches := impsOKPattern.FindStringSubmatch(upperNarration); len(matches) > 2 {
		name := strings.TrimSpace(matches[1])
		if isValidIMPSName(name) {
			names = append(names, name)
		}
		bank = normalizeBank(matches[2])
		return
	}

	// Try MMT/IMPS/ref/name1/name2/bank pattern
	if matches := impsTwoNamesPattern.FindStringSubmatch(upperNarration); len(matches) > 3 {
		name1 := strings.TrimSpace(matches[1])
		name2 := strings.TrimSpace(matches[2])
		// Validate that these aren't status codes
		if isValidIMPSName(name1) && name1 != "OK" {
			names = append(names, name1)
		}
		if isValidIMPSName(name2) && name2 != "OK" {
			names = append(names, name2)
		}
		bank = normalizeBank(matches[3])
		return
	}

	// Try MMT/IMPS/ref/secondary_ref /<name>/<bank> pattern (secondary reference format)
	if matches := impsSecondaryRefPattern.FindStringSubmatch(upperNarration); len(matches) > 2 {
		name := strings.TrimSpace(matches[1])
		if isValidIMPSName(name) {
			names = append(names, name)
		}
		bank = normalizeBank(matches[2])
		return
	}

	// Try MMT/IMPS/ref/IMPS P2A <sender> /<receiver>/<bank> pattern (P2A format)
	if matches := impsP2APattern.FindStringSubmatch(upperNarration); len(matches) > 3 {
		sender := strings.TrimSpace(matches[1])
		receiver := strings.TrimSpace(matches[2])
		if isValidIMPSName(sender) {
			names = append(names, sender)
		}
		if isValidIMPSName(receiver) {
			names = append(names, receiver)
		}
		bank = normalizeBank(matches[3])
		return
	}

	return nil, ""
}

// Extract extracts all identifiers from a narration string
func Extract(narration string) []Identifier {
	var identifiers []Identifier
	seen := make(map[string]bool)

	// Normalize narration - convert to uppercase for consistent matching
	upperNarration := strings.ToUpper(narration)

	// Extract UPI VPAs
	upiMatches := upiPattern.FindAllStringSubmatch(narration, -1)
	for _, match := range upiMatches {
		if len(match) > 1 {
			value := strings.ToUpper(match[1])
			key := string(TypeUPIVPA) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeUPIVPA,
					Value: value,
				})
			}
		}
	}

	// Extract UPI IDs from narration format (UPI/<txn_id>/UPI/<upi_id>/<bank>)
	upiNarrationMatches := upiNarrationPattern.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range upiNarrationMatches {
		if len(match) > 1 {
			value := match[1]
			key := string(TypeUPIVPA) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeUPIVPA,
					Value: value,
				})
			}
		}
	}

	// Extract UPI IDs from alternate narration format (UPI/<name>/<upi_id>/PAYMENT FR/)
	upiNarrationMatches2 := upiNarrationPattern2.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range upiNarrationMatches2 {
		if len(match) > 1 {
			value := match[1]
			key := string(TypeUPIVPA) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeUPIVPA,
					Value: value,
				})
			}
		}
	}

	// Extract UPI IDs from narration format (UPI/<txn_id>/<name>/<upi_id>/<location>/)
	upiNarrationMatches3 := upiNarrationPattern3.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range upiNarrationMatches3 {
		if len(match) > 1 {
			value := match[1]
			key := string(TypeUPIVPA) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeUPIVPA,
					Value: value,
				})
			}
		}
	}

	// Extract UPI IDs from narration format (UPI/<name>/<upi_id>/<other>/<bank>/<ref>/<code>)
	upiNarrationMatches4 := upiNarrationPattern4.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range upiNarrationMatches4 {
		if len(match) > 1 {
			value := match[1]
			key := string(TypeUPIVPA) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeUPIVPA,
					Value: value,
				})
			}
		}
	}

	// Extract phone numbers
	phoneMatches := phonePattern.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range phoneMatches {
		if len(match) > 1 {
			value := match[1]
			key := string(TypePhone) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypePhone,
					Value: value,
				})
			}
		}
	}

	// Extract account numbers from NEFT/RTGS patterns
	accountMatches := accountPattern.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range accountMatches {
		if len(match) > 1 {
			value := match[1]
			key := string(TypeAccountNumber) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeAccountNumber,
					Value: value,
				})
			}
		}
	}

	// Also try alternative account pattern
	accountMatchesAlt := accountPatternAlt.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range accountMatchesAlt {
		if len(match) > 1 {
			value := match[1]
			key := string(TypeAccountNumber) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeAccountNumber,
					Value: value,
				})
			}
		}
	}

	// Extract IFSC codes
	ifscMatches := ifscPattern.FindAllString(upperNarration, -1)
	for _, value := range ifscMatches {
		key := string(TypeIFSC) + ":" + value
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeIFSC,
				Value: value,
			})
		}
	}

	// Extract IMPS names and bank names
	names, bank := extractIMPSData(narration)
	for _, name := range names {
		key := string(TypeIMPSName) + ":" + name
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeIMPSName,
				Value: name,
			})
		}
	}
	if bank != "" {
		key := string(TypeBankName) + ":" + bank
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeBankName,
				Value: bank,
			})
		}
	}

	return identifiers
}

// ExtractValues extracts all identifier values as a flat string slice
func ExtractValues(narration string) []string {
	identifiers := Extract(narration)
	values := make([]string, len(identifiers))
	for i, id := range identifiers {
		values[i] = id.Value
	}
	return values
}

// ExtractByType extracts identifiers of a specific type
func ExtractByType(narration string, idType IdentifierType) []string {
	identifiers := Extract(narration)
	var values []string
	for _, id := range identifiers {
		if id.Type == idType {
			values = append(values, id.Value)
		}
	}
	return values
}

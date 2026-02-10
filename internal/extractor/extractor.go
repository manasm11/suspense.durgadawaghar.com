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
	TypeIMPSName      IdentifierType = "imps_name"      // Sender/receiver name from IMPS
	TypeBankName      IdentifierType = "bank_name"      // Bank name from IMPS
	TypeNEFTName      IdentifierType = "neft_name"      // Sender/receiver name from NEFT
	TypeCashBankCode  IdentifierType = "cash_bank_code"  // Bank code from cash deposits
	TypeCashLocation  IdentifierType = "cash_location"   // Location from cash deposits (e.g., TIRWA (UP))
	TypeCashAgentCode IdentifierType = "cash_agent_code" // Agent code from cash deposits (e.g., DDG000201)
	TypeFromAccount   IdentifierType = "from_account"    // Masked account from From: field (e.g., XXXX8723)
	TypeFromName      IdentifierType = "from_name"       // Sender name from From: field
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

	// UPI ID from narration format: UPI/<upi_id>/<name>/<bank>/<ref>/<code>
	// Captures the UPI ID (e.g., ASHISHKUMARPAND from UPI/ASHISHKUMARPAND/SHRI RADHEY KRI/BANK OF BARODA/102557916140/HDFA655BF2F2)
	upiNarrationPattern5 = regexp.MustCompile(`UPI/([A-Za-z0-9._@-]+)/[^/]+/[^/]+/\d+/[A-Za-z0-9]+$`)

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
	// MMT/IMPS/<ref>/REQPAY/<name> /<bank> - REQPAY format (request payment)
	impsREQPAYPattern = regexp.MustCompile(`MMT/IMPS/\d{12}/REQPAY/([^/]+?)\s*/(.+)`)
	// MMT/IMPS/<ref>/<name>/<bank> - simple name/bank format (fallback for formats without OK/REQPAY/etc)
	impsSimplePattern = regexp.MustCompile(`MMT/IMPS/\d{12}/([A-Z][A-Z\s]*)/([A-Z][A-Z\s]+)$`)

	// NEFT pattern: NEFT-<IFSC_PREFIX><REF>-<NAME>-<rest>
	// Examples: NEFT-UCBAN52025040104667985-SHRI SHYAM AGENCY-/FAST///
	//           NEFT-BARBN52025040226217799-VAIBHAV LAXMI MEDICALSTORE--37100200000337
	neftNamePattern = regexp.MustCompile(`NEFT-[A-Z]{4,5}[A-Z0-9]*\d+-([^-]+)-`)

	// INFT pattern: INF/INFT/<ref>/<name1> /<name2>
	// Example: INF/INFT/039939724801/DURGAKNP /S S PHARMA
	// Extracts name2 (the receiver/party name)
	inftNamePattern = regexp.MustCompile(`INF/INFT/\d+/[^/]+\s*/([^/]+)`)

	// INFT single name pattern: INF/INFT/<ref>/<name>
	// Example: INF/INFT/041141036691/GAYATRI PHARMA
	// Extracts the single name at the end
	inftSingleNamePattern = regexp.MustCompile(`INF/INFT/\d+/([A-Z][A-Z\s]+)$`)

	// BIL/INFT pattern: BIL/INFT/<ref>/ <name>
	// Example: BIL/INFT/EDC0857581/ SANJIT KUMAR
	// Extracts the name after the reference
	bilInftNamePattern = regexp.MustCompile(`BIL/INFT/[A-Z0-9]+/\s*([A-Z][A-Z\s]+)`)

	// NEFT_IN pattern: NEFT_IN:null//<ref>/<name>
	// Example: NEFT_IN:null//SBINN52025042334823235/VIJAY MEDICAL STORE Ag. DDG000516
	// Extracts the name after the reference (stops before Ag. if present)
	neftInNamePattern = regexp.MustCompile(`NEFT_IN:[^/]*//[A-Z0-9]+/([A-Z][A-Z\s]+?)(?:\s+AG\.|\s*$)`)

	// Cash deposit bank code pattern: BY CASH -<code> <location>
	// Example: "BY CASH -733300 TIRWA (UP)" -> code="733300"
	cashBankCodePattern = regexp.MustCompile(`BY\s+CASH\s+-(\d{5,8})`)

	// Cash deposit location pattern: BY CASH -<code> <location>
	// Example: "BY CASH -733300 TIRWA (UP)" -> location="TIRWA (UP)"
	// Captures location name with optional state code in parentheses
	cashLocationPattern = regexp.MustCompile(`BY\s+CASH\s+-\d{5,8}\s+([A-Z][A-Za-z]*(?:\s+\([A-Z]{2}\))?)`)

	// Cash deposit agent code pattern: Ag. <code> or similar agent identifiers
	// Example: "BY CASH -733300 TIRWA (UP) Ag. DDG000201" -> agent="DDG000201"
	// Example: "From:XXXX8723:ASHWANI KUMAR Ag. *DDG029160," -> agent="DDG029160"
	// Pattern matches alphanumeric codes that look like agent/agency identifiers
	// Note: uses uppercase because we match against upperNarration
	// The \*? handles optional asterisk prefix before the agent code
	cashAgentCodePattern = regexp.MustCompile(`(?:AG\.?|AGT\.?|AGENCY)\s*\*?([A-Z]{2,4}\d{6,10})`)

	// From pattern: From:XXXX<4digits>:<SENDER NAME>
	// Example: "From:XXXX8723:ASHWANI KUMAR"
	fromPattern = regexp.MustCompile(`FROM:([X]{4}\d{4}):([A-Z][A-Z\s]+)`)
)

// bankNormalization maps truncated bank names to full names
var bankNormalization = map[string]string{
	"UNION BANKOF I":  "UNION BANK OF INDIA",
	"STATE BANK O":    "STATE BANK OF INDIA",
	"STATE BANK OF I": "STATE BANK OF INDIA",
	"BANK OF BARO":    "BANK OF BARODA",
	"PUNJAB NATIO":    "PUNJAB NATIONAL BANK",
	"CANARA BANK":     "CANARA BANK",
	"HDFC BANK":       "HDFC BANK",
	"ICICI BANK":      "ICICI BANK",
	"AXIS BANK":       "AXIS BANK",
	"KOTAK MAHIND":    "KOTAK MAHINDRA BANK",
	"INDUSIND BAN":    "INDUSIND BANK",
	"YES BANK":        "YES BANK",
	"IDBI BANK":       "IDBI BANK",
	"CENTRAL BANK":    "CENTRAL BANK OF INDIA",
	"INDIAN BANK":     "INDIAN BANK",
	"INDIAN OVERS":    "INDIAN OVERSEAS BANK",
	"UCO BANK":        "UCO BANK",
	"BANK OF INDI":    "BANK OF INDIA",
	"SYNDICATE BA":    "SYNDICATE BANK",
	"ALLAHABAD BA":    "ALLAHABAD BANK",
	"CORPORATION":     "CORPORATION BANK",
	"ORIENTAL BAN":    "ORIENTAL BANK OF COMMERCE",
	"UNITED BANK":     "UNITED BANK OF INDIA",
	"DENA BANK":       "DENA BANK",
	"VIJAYA BANK":     "VIJAYA BANK",
	"FEDERAL BANK":    "FEDERAL BANK",
	"SOUTH INDIAN":    "SOUTH INDIAN BANK",
	"KARNATAKA BA":    "KARNATAKA BANK",
	"BANDHAN BANK":    "BANDHAN BANK",
	"RBL BANK":        "RBL BANK",
	"IDFC FIRST B":    "IDFC FIRST BANK",
	"AU SMALL FIN":    "AU SMALL FINANCE BANK",
	"EQUITAS SMAL":    "EQUITAS SMALL FINANCE BANK",
	"UJJIVAN SMAL":    "UJJIVAN SMALL FINANCE BANK",
	"PAYTM PAYMEN":    "PAYTM PAYMENTS BANK",
	"AIRTEL PAYME":    "AIRTEL PAYMENTS BANK",
	"FINO PAYMENT":    "FINO PAYMENTS BANK",
	"JIOPAYMENTSB":    "JIO PAYMENTS BANK",
	"PUNJAB AND SIND": "PUNJAB AND SIND BANK",
	"PUNJAB AND S":    "PUNJAB AND SIND BANK",
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

// isValidExtractedName checks if the extracted name is valid (not a status code or payment description)
func isValidExtractedName(name string) bool {
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
		if isValidExtractedName(name) {
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
		if isValidExtractedName(name1) && name1 != "OK" {
			names = append(names, name1)
		}
		if isValidExtractedName(name2) && name2 != "OK" {
			names = append(names, name2)
		}
		bank = normalizeBank(matches[3])
		return
	}

	// Try MMT/IMPS/ref/secondary_ref /<name>/<bank> pattern (secondary reference format)
	if matches := impsSecondaryRefPattern.FindStringSubmatch(upperNarration); len(matches) > 2 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			names = append(names, name)
		}
		bank = normalizeBank(matches[2])
		return
	}

	// Try MMT/IMPS/ref/IMPS P2A <sender> /<receiver>/<bank> pattern (P2A format)
	if matches := impsP2APattern.FindStringSubmatch(upperNarration); len(matches) > 3 {
		sender := strings.TrimSpace(matches[1])
		receiver := strings.TrimSpace(matches[2])
		if isValidExtractedName(sender) {
			names = append(names, sender)
		}
		if isValidExtractedName(receiver) {
			names = append(names, receiver)
		}
		bank = normalizeBank(matches[3])
		return
	}

	// Try MMT/IMPS/ref/REQPAY/<name> /<bank> pattern (REQPAY format)
	if matches := impsREQPAYPattern.FindStringSubmatch(upperNarration); len(matches) > 2 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			names = append(names, name)
		}
		bank = normalizeBank(matches[2])
		return
	}

	// Try MMT/IMPS/ref/<name>/<bank> pattern (simple name/bank format - fallback)
	if matches := impsSimplePattern.FindStringSubmatch(upperNarration); len(matches) > 2 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			names = append(names, name)
		}
		bank = normalizeBank(matches[2])
		return
	}

	return nil, ""
}

// extractNEFTName extracts party name from NEFT/INFT narrations
// Formats:
//   - NEFT-<IFSC_PREFIX><REF>-<NAME>-<rest>
//   - INF/INFT/<ref>/<name1> /<name2>
//   - BIL/INFT/<ref>/ <name>
func extractNEFTName(narration string) string {
	upperNarration := strings.ToUpper(narration)

	// Try NEFT pattern first
	if matches := neftNamePattern.FindStringSubmatch(upperNarration); len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			return name
		}
	}

	// Try INFT two-name pattern
	if matches := inftNamePattern.FindStringSubmatch(upperNarration); len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			return name
		}
	}

	// Try INFT single name pattern
	if matches := inftSingleNamePattern.FindStringSubmatch(upperNarration); len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			return name
		}
	}

	// Try BIL/INFT pattern
	if matches := bilInftNamePattern.FindStringSubmatch(upperNarration); len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			return name
		}
	}

	// Try NEFT_IN pattern
	if matches := neftInNamePattern.FindStringSubmatch(upperNarration); len(matches) > 1 {
		name := strings.TrimSpace(matches[1])
		if isValidExtractedName(name) {
			return name
		}
	}

	return ""
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

	// Extract UPI IDs from narration format (UPI/<upi_id>/<name>/<bank>/<ref>/<code>)
	upiNarrationMatches5 := upiNarrationPattern5.FindAllStringSubmatch(upperNarration, -1)
	for _, match := range upiNarrationMatches5 {
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

	// Extract NEFT names
	neftName := extractNEFTName(narration)
	if neftName != "" {
		key := string(TypeNEFTName) + ":" + neftName
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeNEFTName,
				Value: neftName,
			})
		}
	}

	// Extract cash deposit bank code
	if cashCodeMatches := cashBankCodePattern.FindStringSubmatch(upperNarration); len(cashCodeMatches) > 1 {
		value := cashCodeMatches[1]
		key := string(TypeCashBankCode) + ":" + value
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeCashBankCode,
				Value: value,
			})
		}
	}

	// Extract cash deposit location
	if locationMatches := cashLocationPattern.FindStringSubmatch(upperNarration); len(locationMatches) > 1 {
		value := strings.TrimSpace(locationMatches[1])
		if value != "" {
			key := string(TypeCashLocation) + ":" + value
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeCashLocation,
					Value: value,
				})
			}
		}
	}

	// Extract cash deposit agent code
	if agentMatches := cashAgentCodePattern.FindStringSubmatch(upperNarration); len(agentMatches) > 1 {
		value := agentMatches[1]
		key := string(TypeCashAgentCode) + ":" + value
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeCashAgentCode,
				Value: value,
			})
		}
	}

	// Extract From: field data (masked account and sender name)
	if fromMatches := fromPattern.FindStringSubmatch(upperNarration); len(fromMatches) > 2 {
		// Extract masked account number (e.g., XXXX8723)
		maskedAccount := fromMatches[1]
		key := string(TypeFromAccount) + ":" + maskedAccount
		if !seen[key] {
			seen[key] = true
			identifiers = append(identifiers, Identifier{
				Type:  TypeFromAccount,
				Value: maskedAccount,
			})
		}

		// Extract sender name (remove trailing " AG" if captured from agent code prefix)
		senderName := strings.TrimSpace(fromMatches[2])
		senderName = strings.TrimSuffix(senderName, " AG")
		senderName = strings.TrimSpace(senderName)
		if isValidExtractedName(senderName) {
			key := string(TypeFromName) + ":" + senderName
			if !seen[key] {
				seen[key] = true
				identifiers = append(identifiers, Identifier{
					Type:  TypeFromName,
					Value: senderName,
				})
			}
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

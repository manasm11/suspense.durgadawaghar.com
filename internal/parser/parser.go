package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Transaction represents a parsed receipt book transaction
type Transaction struct {
	Date        time.Time
	PartyName   string
	Location    string
	Amount      float64
	Narration   string // Combined bank account info and payment details
	PaymentMode string
}

var (
	// Date pattern: "Dec 26", "Jan 1", etc.
	datePattern = regexp.MustCompile(`^(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(\d{1,2})\s+`)

	// Amount pattern: number with optional decimal at end of line
	amountPattern = regexp.MustCompile(`(\d+(?:\.\d{2})?)\s*$`)

	// Bank account line pattern: Bank name followed by account number and amount
	// e.g., "ICICI 192105002017 11145.00"
	bankAccountPattern = regexp.MustCompile(`^(?i)(ICICI|HDFC|SBI|PNB|AXIS|KOTAK|YES|IDBI|CANARA|BOI|BOB|IDFC|UNION|INDIAN|UCO|CENTRAL|PUNJAB|BARODA|ALLAHABAD|ANDHRA|BANK|STATE)\s+\d+\s+[\d,.]+`)

	// Lines to skip
	skipPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^SUB\s+TOTAL`),
		regexp.MustCompile(`(?i)\.\.\.Continued`),
		regexp.MustCompile(`(?i)^SUSPENSE\s+A/C`),
		regexp.MustCompile(`(?i)^\s*$`),
		regexp.MustCompile(`^-+$`),                                    // Separator lines
		regexp.MustCompile(`(?i)^TOTAL\s+[\d,.]+\s+[\d,.]+$`),         // Total line
		regexp.MustCompile(`(?i)^\*\*\*.*\*\*\*$`),                    // *** End of Report ***
		regexp.MustCompile(`(?i)^DATE\s+PARTICULARS\s+DEBIT\s+CREDIT`), // Header line
		regexp.MustCompile(`(?i)^RECEIPT\s+BOOK`),                     // Receipt book header
		regexp.MustCompile(`(?i)^DURGA\s+DAWA\s+GHAR`),                // Company name header
		regexp.MustCompile(`(?i)^\d{2}-\d{2}-\d{4}\s+-\s+\d{2}-\d{2}-\d{4}$`), // Date range header
		regexp.MustCompile(`(?i)^E-Mail\s*:`),                         // Email line
		regexp.MustCompile(`(?i)^D\.?L\.?\s*No\.?\s*:`),               // DL number line
		regexp.MustCompile(`(?i)^GSTIN\s*:`),                          // GSTIN line
		regexp.MustCompile(`(?i)^\d+/\d+,`),                           // Address line (60/33,...)
	}

	// Payment mode detection patterns
	// Note: These patterns match anywhere in the narration since bank account info often comes first
	upiModePattern  = regexp.MustCompile(`(?i)^UPI/|/UPI/|/UPI$|\sUPI/`)
	impsModePattern = regexp.MustCompile(`(?i)IMPS/|/IMPS/|MMT/IMPS`)
	neftModePattern = regexp.MustCompile(`(?i)\sNEFT-|^NEFT-`)
	rtgsModePattern = regexp.MustCompile(`(?i)\sRTGS-|^RTGS-`)
	clgModePattern  = regexp.MustCompile(`(?i)\sCLG/|^CLG/`)
	infModePattern  = regexp.MustCompile(`(?i)\sINF/|^INF/|^INFT/|/INFT/|\sINFT/`)
	chqModePattern  = regexp.MustCompile(`(?i)Chq\.|Cheque|CHQ`)
	posModePattern  = regexp.MustCompile(`(?i)FT-MESPOS|MESPOS\s+SET|POS\s+MACHINE`)
	cashModePattern = regexp.MustCompile(`(?i)^BY\s+CASH|\sBY\s+CASH|CASH\s+DEP|CAM/`)

	// Invoice reference pattern to ignore: "Ag. DDG...", "Ag. *DDG028429,*DDG028437,...", "Ag. DDGT000180", etc.
	// Matches everything after "Ag." since it's all invoice reference data
	invoiceRefPattern = regexp.MustCompile(`\s*Ag\.\s*.*$`)

	// Month name to number mapping
	monthMap = map[string]time.Month{
		"Jan": time.January,
		"Feb": time.February,
		"Mar": time.March,
		"Apr": time.April,
		"May": time.May,
		"Jun": time.June,
		"Jul": time.July,
		"Aug": time.August,
		"Sep": time.September,
		"Oct": time.October,
		"Nov": time.November,
		"Dec": time.December,
	}
)

// Parse parses receipt book text and returns a slice of transactions
func Parse(text string, year int) []Transaction {
	lines := strings.Split(text, "\n")
	var transactions []Transaction
	var currentTx *Transaction
	var narrationLines []string
	var lastDate time.Time

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and known skip patterns
		if shouldSkipLine(line) {
			continue
		}

		// Check if this is a new transaction (starts with date)
		if match := datePattern.FindStringSubmatch(line); match != nil {
			// Save previous transaction if exists
			if currentTx != nil {
				currentTx.Narration = buildNarration(narrationLines)
				currentTx.PaymentMode = detectPaymentMode(currentTx.Narration)
				transactions = append(transactions, *currentTx)
			}

			// Parse new transaction
			currentTx = parseFirstLine(line, match, year)
			lastDate = currentTx.Date
			narrationLines = nil

			// Check if party name is SUSPENSE A/C
			if strings.Contains(strings.ToUpper(currentTx.PartyName), "SUSPENSE A/C") {
				currentTx = nil
				continue
			}
		} else if currentTx != nil {
			// Check if this is a bank account line (should be added to narration)
			if bankAccountPattern.MatchString(line) {
				cleanLine := invoiceRefPattern.ReplaceAllString(line, "")
				cleanLine = strings.TrimSpace(cleanLine)
				if cleanLine != "" {
					narrationLines = append(narrationLines, cleanLine)
				}
				continue
			}

			// Check if this looks like a party line (has amount at end, contains text)
			if isPartyLine(line) {
				// Save current transaction
				currentTx.Narration = buildNarration(narrationLines)
				currentTx.PaymentMode = detectPaymentMode(currentTx.Narration)
				transactions = append(transactions, *currentTx)

				// Create new transaction with inherited date
				currentTx = parsePartyLine(line, lastDate)
				narrationLines = nil

				// Check if party name is SUSPENSE A/C
				if strings.Contains(strings.ToUpper(currentTx.PartyName), "SUSPENSE A/C") {
					currentTx = nil
					continue
				}
				continue
			}

			// This is a continuation line (narration)
			// Remove invoice references
			cleanLine := invoiceRefPattern.ReplaceAllString(line, "")
			cleanLine = strings.TrimSpace(cleanLine)
			if cleanLine != "" {
				narrationLines = append(narrationLines, cleanLine)
			}
		}
	}

	// Don't forget the last transaction
	if currentTx != nil {
		currentTx.Narration = buildNarration(narrationLines)
		currentTx.PaymentMode = detectPaymentMode(currentTx.Narration)
		transactions = append(transactions, *currentTx)
	}

	return transactions
}

func shouldSkipLine(line string) bool {
	if line == "" {
		return true
	}
	for _, pattern := range skipPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func parseFirstLine(line string, dateMatch []string, year int) *Transaction {
	tx := &Transaction{}

	// Parse date
	monthStr := dateMatch[1]
	dayStr := dateMatch[2]
	day, _ := strconv.Atoi(dayStr)
	month := monthMap[monthStr]
	tx.Date = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	// Remove date from line
	remaining := datePattern.ReplaceAllString(line, "")

	// Extract amount from end
	if amountMatch := amountPattern.FindStringSubmatch(remaining); amountMatch != nil {
		tx.Amount, _ = strconv.ParseFloat(amountMatch[1], 64)
		remaining = amountPattern.ReplaceAllString(remaining, "")
	}

	// Remaining is party name + location
	remaining = strings.TrimSpace(remaining)
	tx.PartyName, tx.Location = parsePartyNameLocation(remaining)

	return tx
}

// isPartyLine checks if a line looks like a party name with amount (but no date)
// Used to detect additional parties in multi-party transactions
func isPartyLine(line string) bool {
	// Must have an amount at the end
	if !amountPattern.MatchString(line) {
		return false
	}

	// Should not start with known narration patterns
	upperLine := strings.ToUpper(line)
	narrationPrefixes := []string{
		"UPI/", "NEFT-", "RTGS-", "IMPS/", "MMT/", "CLG/", "INF/", "INFT/",
		"CHQ.", "CHEQUE", "BY CASH", "FT-MESPOS", "BIL/",
	}
	for _, prefix := range narrationPrefixes {
		if strings.HasPrefix(upperLine, prefix) {
			return false
		}
	}

	// Should not be a bank account line
	if bankAccountPattern.MatchString(line) {
		return false
	}

	// Remove the amount and check what's left
	remaining := amountPattern.ReplaceAllString(line, "")
	remaining = strings.TrimSpace(remaining)

	// Should have at least 2 words (party name typically has multiple words)
	words := strings.Fields(remaining)
	if len(words) < 2 {
		return false
	}

	// First word should be alphabetic (party names start with letters)
	firstWord := words[0]
	if len(firstWord) == 0 {
		return false
	}
	firstChar := rune(firstWord[0])
	if firstChar < 'A' || (firstChar > 'Z' && firstChar < 'a') || firstChar > 'z' {
		return false
	}

	return true
}

// parsePartyLine parses a line that has party name and amount but no date
func parsePartyLine(line string, inheritedDate time.Time) *Transaction {
	tx := &Transaction{
		Date: inheritedDate,
	}

	remaining := line

	// Extract amount from end
	if amountMatch := amountPattern.FindStringSubmatch(remaining); amountMatch != nil {
		tx.Amount, _ = strconv.ParseFloat(amountMatch[1], 64)
		remaining = amountPattern.ReplaceAllString(remaining, "")
	}

	// Remaining is party name + location
	remaining = strings.TrimSpace(remaining)
	tx.PartyName, tx.Location = parsePartyNameLocation(remaining)

	return tx
}

func parsePartyNameLocation(text string) (name, location string) {
	text = strings.TrimSpace(text)

	// Words that should NOT be treated as locations even if they look like one
	nonLocationWords := map[string]bool{
		"BUSINESS": true,
		"MACHINE":  true,
		"STORE":    true,
		"AGENCY":   true,
		"TRADERS":  true,
		"PHARMA":   true,
		"CHEMIST":  true,
		"MEDICOS":  true,
		"MEDICAL":  true,
		"DRUG":     true,
		"HOUSE":    true,
		"HALL":     true,
		"CENTRE":   true,
		"CENTER":   true,
	}

	// Common location indicators (uppercase versions)
	// Includes major cities and locations from receipt book data
	locationIndicators := []string{
		// Major Indian cities
		"DELHI", "MUMBAI", "KOLKATA", "CHENNAI", "BANGALORE", "HYDERABAD",
		"AHMEDABAD", "PUNE", "SURAT", "JAIPUR", "LUCKNOW", "KANPUR",
		"NAGPUR", "INDORE", "THANE", "BHOPAL", "PATNA", "VADODARA",
		"GHAZIABAD", "LUDHIANA", "AGRA", "NASHIK", "FARIDABAD", "MEERUT",
		"RAJKOT", "VARANASI", "SRINAGAR", "AURANGABAD", "DHANBAD", "AMRITSAR",
		"JODHPUR", "RAIPUR", "RANCHI", "GWALIOR", "CHANDIGARH", "VIJAYAWADA",
		"MADURAI", "COIMBATORE", "KOCHI", "GUWAHATI", "BHUBANESWAR", "DEHRADUN",
		"NOIDA", "GURUGRAM", "GURGAON", "NCR", "GWALIOUR",
		// UP towns and areas from receipt book
		"SEKHREJ", "SHAMBHUA", "MUSKRA", "BILLHAUR", "RASULABAD", "MUNGISAPUR",
		"JUNIHA", "MAHARAMAU", "AKBARPUR", "AKABARPUR", "CHIBRAMAU", "DHAURA",
		"CHAMIYANI", "CHAUDAGRA", "BARAUR", "INDERGAR", "GHATAMPUR", "BITHOOR",
		"BIGHAPUR", "BAIRAGIHAR", "SIKANDRA", "ACHALGANJ", "PUKHRAYA", "PUKHRAYAN",
		"DIBIAPUR", "DIBIYAPUR", "MIYAGANJ", "AURAIYA", "LALITPUR", "MAKANPUR",
		"RAATH", "KHAKHRERU", "SAHAYAL", "CHANI", "SAJETI", "BASIRAT", "JALLAUN",
		"BANGARMAU", "ALIYAPUR", "TIRWA", "BAKEWAR", "BHAUTY", "KANNOUJ", "KONCH",
		"NAWABGANJ", "FATEHPUR", "ORAI", "HARDOI", "UNNAO", "SITAPUR", "ETAWAH",
		"BANDA", "JHANSI", "HAMEERPUR", "BHEWAN", "NABIPUR", "TISTI", "UMARDA",
		"TALEGRAM", "KENJARI", "KENJARY", "JHIJHAK", "HASEERAN", "SHIVRAJPUR",
		"BAHOSI", "KUDANY", "VISHDHAN", "KAKVAN", "MAUDAHA", "JAHANABAD",
		"MURADIPUR", "PARSAULI", "AJGAIN", "RAMAIPUR", "DHANI", "BARUA", "SAHAR",
		"KHAJUA", "BARUA", "FARRUKHABAD", "LAKHIMPUR", "GONDA", "SHIVLI",
		"MANIMAU", "ROORA", "ROOMA", "RANIA", "NOONARI", "NARWAL", "TIKRA",
		"BHARUA", "CHHIBRAMAU", "FAZALGANJ", "KALYANPUR", "KALYAN", "KAKADEV",
		"BIRHANA", "MANISHA", "SUMER", "BEEGAHPUR", "HASWA", "SIRATHU",
		"VIJAIPUR", "ATARDHANI", "MAURANIPUR", "SACHENDI", "BITHHOR", "BARAIGHAR",
		"HAPUR", "GEHLO", "DEHAT",
		// Additional locations from June 2025 receipt book
		"NAUBASTA", "PANKI", "BHAGHPUR", "NARAMAU", "THATHIA", "REWARI",
		"BAIRAMPUR", "GALUAPUR", "SAROSI", "AGAUS", "PATARA", "BANIPARA",
		"MAQSUDABAD", "TIGAI", "HAIDRABAD", "KHEDA", "ALLIPUR", "ASHOTHAR",
		"THARIYAOAN", "SIMRI", "CHAURA", "CHOWKI", "CHHILLA", "SAHLI",
		"SAKURABAD", "SUMRAHA", "MURADAB", "GURSHAYAN",
		"BARADEVI", "BARRA", "PATARSA", "KHAGA", "KORIYAN",
		"BHOGNIPUR", "RAJPUR", "SAHJHANPUR",
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text, ""
	}

	// Check if last word looks like a location
	lastWord := strings.ToUpper(words[len(words)-1])

	// Skip if it's a known non-location word
	if nonLocationWords[lastWord] {
		return text, ""
	}

	for _, loc := range locationIndicators {
		if lastWord == loc || strings.HasPrefix(lastWord, loc) {
			if len(words) > 1 {
				return strings.Join(words[:len(words)-1], " "), words[len(words)-1]
			}
		}
	}

	// If last word is all caps and short (< 15 chars), might be location
	if len(words) > 1 && len(lastWord) < 15 && lastWord == words[len(words)-1] {
		// Check if it's alphabetic only (typical for place names)
		isAlpha := true
		for _, r := range lastWord {
			if r < 'A' || r > 'Z' {
				isAlpha = false
				break
			}
		}
		if isAlpha && len(lastWord) > 2 {
			return strings.Join(words[:len(words)-1], " "), words[len(words)-1]
		}
	}

	return text, ""
}

func buildNarration(lines []string) string {
	return strings.Join(lines, " ")
}

func detectPaymentMode(narration string) string {
	if rtgsModePattern.MatchString(narration) {
		return "RTGS"
	}
	if neftModePattern.MatchString(narration) {
		return "NEFT"
	}
	if impsModePattern.MatchString(narration) {
		return "IMPS"
	}
	if upiModePattern.MatchString(narration) {
		return "UPI"
	}
	if clgModePattern.MatchString(narration) {
		return "CLG"
	}
	if infModePattern.MatchString(narration) {
		return "INF"
	}
	if chqModePattern.MatchString(narration) {
		return "CHEQUE"
	}
	if posModePattern.MatchString(narration) {
		return "POS"
	}
	if cashModePattern.MatchString(narration) {
		return "CASH"
	}
	return "OTHER"
}

// ParseWithAutoYear parses receipt book text and auto-detects year from content
// or uses the current year as default
func ParseWithAutoYear(text string) []Transaction {
	// Try to find year in text (e.g., "26-12-2025")
	yearPattern := regexp.MustCompile(`\d{2}-\d{2}-(\d{4})`)
	if match := yearPattern.FindStringSubmatch(text); match != nil {
		if year, err := strconv.Atoi(match[1]); err == nil {
			return Parse(text, year)
		}
	}
	// Default to current year
	return Parse(text, time.Now().Year())
}

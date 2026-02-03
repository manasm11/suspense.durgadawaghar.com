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

	// Lines to skip
	skipPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^SUB\s+TOTAL`),
		regexp.MustCompile(`(?i)\.\.\.Continued`),
		regexp.MustCompile(`(?i)^SUSPENSE\s+A/C`),
		regexp.MustCompile(`(?i)^\s*$`),
	}

	// Payment mode detection patterns
	upiModePattern  = regexp.MustCompile(`(?i)^UPI/|/UPI/|/UPI$`)
	impsModePattern = regexp.MustCompile(`(?i)IMPS/|/IMPS/|MMT/IMPS`)
	neftModePattern = regexp.MustCompile(`(?i)^NEFT-`)
	rtgsModePattern = regexp.MustCompile(`(?i)^RTGS-`)
	clgModePattern  = regexp.MustCompile(`(?i)^CLG/`)
	infModePattern  = regexp.MustCompile(`(?i)^INF/|^INFT/|/INFT/`)
	chqModePattern  = regexp.MustCompile(`(?i)Chq\.|Cheque|CHQ`)

	// Invoice reference pattern to ignore: "Ag. DDG..."
	invoiceRefPattern = regexp.MustCompile(`\s*Ag\.\s*DDG\d+`)

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
			narrationLines = nil

			// Check if party name is SUSPENSE A/C
			if strings.Contains(strings.ToUpper(currentTx.PartyName), "SUSPENSE A/C") {
				currentTx = nil
				continue
			}
		} else if currentTx != nil {
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

func parsePartyNameLocation(text string) (name, location string) {
	text = strings.TrimSpace(text)

	// Common location indicators (uppercase versions)
	locationIndicators := []string{
		"DELHI", "MUMBAI", "KOLKATA", "CHENNAI", "BANGALORE", "HYDERABAD",
		"AHMEDABAD", "PUNE", "SURAT", "JAIPUR", "LUCKNOW", "KANPUR",
		"NAGPUR", "INDORE", "THANE", "BHOPAL", "PATNA", "VADODARA",
		"GHAZIABAD", "LUDHIANA", "AGRA", "NASHIK", "FARIDABAD", "MEERUT",
		"RAJKOT", "VARANASI", "SRINAGAR", "AURANGABAD", "DHANBAD", "AMRITSAR",
		"JODHPUR", "RAIPUR", "RANCHI", "GWALIOR", "CHANDIGARH", "VIJAYAWADA",
		"MADURAI", "COIMBATORE", "KOCHI", "GUWAHATI", "BHUBANESWAR", "DEHRADUN",
		"NOIDA", "GURUGRAM", "GURGAON", "NCR",
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text, ""
	}

	// Check if last word looks like a location
	lastWord := strings.ToUpper(words[len(words)-1])
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

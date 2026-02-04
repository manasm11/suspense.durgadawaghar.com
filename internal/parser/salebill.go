package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SaleBill represents a parsed sale bill entry
type SaleBill struct {
	BillNumber string
	Date       time.Time
	PartyName  string
	Amount     float64
	IsCashSale bool
}

var (
	// Header pattern to extract year: SALE FROM DD-MM-YYYY TO DD-MM-YYYY
	saleHeaderPattern = regexp.MustCompile(`(?i)SALE\s+FROM\s+\d{2}-\d{2}-(\d{4})\s+TO\s+\d{2}-\d{2}-(\d{4})`)

	// Bill line pattern: BILLNUM DD-MM PARTY NAME AMOUNT
	// e.g., A240100001 01-04 PARTY NAME HERE 1,234.56
	billLinePattern = regexp.MustCompile(`^([A-Z0-9]+)\s+(\d{2}-\d{2})\s+(.+?)\s+([\d,]+\.\d{2})$`)

	// CASH party pattern: CASH (PARTY NAME)
	cashPartyPattern = regexp.MustCompile(`(?i)^CASH\s*\(([^)]+)\)`)
)

// ParseSaleBills parses sale bill data and returns a slice of SaleBill
func ParseSaleBills(data string, defaultYear int) []SaleBill {
	lines := strings.Split(data, "\n")
	var bills []SaleBill

	// Try to extract year from header
	year := defaultYear
	for _, line := range lines {
		if matches := saleHeaderPattern.FindStringSubmatch(line); matches != nil {
			// Use the "TO" year (second year in range)
			if y, err := strconv.Atoi(matches[2]); err == nil {
				year = y
			}
			break
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header lines, page markers, totals, and separators
		if shouldSkipSaleBillLine(line) {
			continue
		}

		// Try to parse as a bill line
		bill := parseBillLine(line, year)
		if bill != nil {
			bills = append(bills, *bill)
		}
	}

	return bills
}

// shouldSkipSaleBillLine returns true if the line should be skipped
func shouldSkipSaleBillLine(line string) bool {
	upperLine := strings.ToUpper(line)

	// Skip header patterns
	if strings.Contains(upperLine, "SALE FROM") {
		return true
	}
	if strings.Contains(upperLine, "BILL NO") || strings.Contains(upperLine, "BILLNO") {
		return true
	}
	if strings.Contains(upperLine, "PARTY NAME") || strings.Contains(upperLine, "PARTYNAME") {
		return true
	}

	// Skip page markers
	if strings.HasPrefix(upperLine, "PAGE") {
		return true
	}

	// Skip totals
	if strings.HasPrefix(upperLine, "TOTAL") {
		return true
	}
	if strings.HasPrefix(upperLine, "GRAND TOTAL") {
		return true
	}

	// Skip continuation markers
	if strings.Contains(upperLine, "CONTINUED") {
		return true
	}

	// Skip separators
	if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
		return true
	}

	// Skip empty lines with just numbers (page numbers, etc.)
	if regexp.MustCompile(`^\d+$`).MatchString(line) {
		return true
	}

	return false
}

// parseBillLine parses a single bill line and returns a SaleBill or nil
func parseBillLine(line string, year int) *SaleBill {
	matches := billLinePattern.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	billNumber := matches[1]
	dateStr := matches[2]
	partyName := strings.TrimSpace(matches[3])
	amountStr := matches[4]

	// Parse date (DD-MM format, add year)
	parts := strings.Split(dateStr, "-")
	if len(parts) != 2 {
		return nil
	}
	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

	// Parse amount (remove commas)
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return nil
	}

	// Check if it's a CASH sale and extract party name from parentheses
	isCashSale := false
	if cashMatches := cashPartyPattern.FindStringSubmatch(partyName); cashMatches != nil {
		isCashSale = true
		partyName = strings.TrimSpace(cashMatches[1])
	} else if strings.ToUpper(partyName) == "CASH" {
		isCashSale = true
	}

	return &SaleBill{
		BillNumber: billNumber,
		Date:       date,
		PartyName:  partyName,
		Amount:     amount,
		IsCashSale: isCashSale,
	}
}

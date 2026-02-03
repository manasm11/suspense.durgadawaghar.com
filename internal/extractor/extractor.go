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
)

// Identifier represents an extracted identifier from a narration
type Identifier struct {
	Type  IdentifierType
	Value string
}

var (
	// UPI VPA: user@provider (e.g., 9450852076@YBL, SUNEELBHADEVANA@HDFC)
	upiPattern = regexp.MustCompile(`([a-zA-Z0-9][a-zA-Z0-9._-]{1,255}@[a-zA-Z]{2,64})`)

	// Phone: 10 digits starting with 6-9
	phonePattern = regexp.MustCompile(`(?:^|[^\d])([6-9]\d{9})(?:[^\d]|$)`)

	// Account Number: 9-18 digits in NEFT/RTGS refs (pattern like -ACCOUNTNUMBER- or -ACCOUNTNUMBER at end)
	accountPattern = regexp.MustCompile(`-(\d{9,18})(?:-|$)`)

	// Additional account pattern for standalone account numbers in specific contexts
	accountPatternAlt = regexp.MustCompile(`(?:A/C|ACCT?|Account)\s*(?:No\.?|#)?\s*(\d{9,18})`)

	// IFSC Code: 4 letters + 0 + 6 alphanumeric characters
	ifscPattern = regexp.MustCompile(`[A-Z]{4}0[A-Z0-9]{6}`)
)

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

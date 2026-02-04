package matcher

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"

	"suspense.durgadawaghar.com/internal/db/sqlc"
	"suspense.durgadawaghar.com/internal/extractor"
)

// MatchResult represents a party match with confidence score
type MatchResult struct {
	Party            sqlc.Party          // Primary party (first found)
	PartyIDs         []int64             // All party IDs with this name
	Confidence       float64
	MatchedOn        []MatchedIdentifier
	TransactionCount int64
	TotalAmount      float64
	RecentTxns       []sqlc.Transaction
}

// MatchedIdentifier represents an identifier that matched
type MatchedIdentifier struct {
	Type  string
	Value string
}

// Confidence weights for different identifier types
const (
	UPIVPAWeight        = 0.95
	PhoneWeight         = 0.85
	AccountNumberWeight = 0.80
	IMPSNameWeight      = 0.50 // Medium - names can be truncated/similar
	NEFTNameWeight      = 0.50 // Medium - same as IMPS, names can be truncated
	BankNameWeight      = 0.20 // Low - many parties use same bank
)

// Matcher handles party matching logic
type Matcher struct {
	queries *sqlc.Queries
}

// NewMatcher creates a new Matcher instance
func NewMatcher(q *sqlc.Queries) *Matcher {
	return &Matcher{queries: q}
}

// Match finds parties matching the given narration and returns scored results
func (m *Matcher) Match(ctx context.Context, narration string) ([]MatchResult, error) {
	// Extract identifiers from the narration
	identifiers := extractor.Extract(narration)

	var matches []sqlc.FindPartiesByIdentifierValuesRow

	// Only try identifier matching if we have identifiers
	if len(identifiers) > 0 {
		// Get unique values for database query
		values := make([]string, len(identifiers))
		for i, id := range identifiers {
			values[i] = id.Value
		}

		// Query database for matching parties
		var err error
		matches, err = m.queries.FindPartiesByIdentifierValues(ctx, values)
		if err != nil {
			return nil, err
		}
	}

	// If no identifier matches found, try fallback narration search
	if len(matches) == 0 {
		return m.matchByNarration(ctx, narration, identifiers)
	}

	// Group matches by party name (not ID) and calculate scores
	partyMatches := make(map[string]*MatchResult)

	for _, match := range matches {
		result, exists := partyMatches[match.Name]
		if !exists {
			result = &MatchResult{
				Party: sqlc.Party{
					ID:        match.ID,
					Name:      match.Name,
					Location:  match.Location,
					CreatedAt: match.CreatedAt,
				},
				PartyIDs:   []int64{match.ID},
				Confidence: 0,
				MatchedOn:  []MatchedIdentifier{},
			}
			partyMatches[match.Name] = result
		} else {
			// Add party ID if not already present
			if !containsInt64(result.PartyIDs, match.ID) {
				result.PartyIDs = append(result.PartyIDs, match.ID)
			}
		}

		// Add matched identifier (dedupe by type+value)
		identifier := MatchedIdentifier{
			Type:  match.MatchType,
			Value: match.MatchValue,
		}
		if !containsIdentifier(result.MatchedOn, identifier) {
			result.MatchedOn = append(result.MatchedOn, identifier)
		}
	}

	// Calculate confidence scores and fetch transaction stats
	results := make([]MatchResult, 0, len(partyMatches))

	for _, result := range partyMatches {
		// Calculate base confidence from identifier matches
		result.Confidence = calculateConfidence(result.MatchedOn)

		// Aggregate stats from all party IDs
		var totalTxCount int64
		var totalAmount float64
		var allRecentTxns []sqlc.Transaction

		for _, partyID := range result.PartyIDs {
			stats, err := m.queries.GetPartyWithTransactionCount(ctx, partyID)
			if err == nil {
				totalTxCount += stats.TransactionCount
				if stats.TotalAmount.Valid {
					totalAmount += stats.TotalAmount.Float64
				}
			}

			// Get recent transactions for this party ID
			recentTxns, err := m.queries.GetRecentTransactionsByPartyID(ctx, sqlc.GetRecentTransactionsByPartyIDParams{
				PartyID: partyID,
				Limit:   5,
			})
			if err == nil {
				allRecentTxns = append(allRecentTxns, recentTxns...)
			}
		}

		result.TransactionCount = totalTxCount
		result.TotalAmount = totalAmount

		// Sort all recent transactions by date and limit to 5
		sort.Slice(allRecentTxns, func(i, j int) bool {
			return allRecentTxns[i].TransactionDate.After(allRecentTxns[j].TransactionDate)
		})
		if len(allRecentTxns) > 5 {
			allRecentTxns = allRecentTxns[:5]
		}
		result.RecentTxns = allRecentTxns

		// Apply history boost: 1.0 + log10(tx_count) * 0.1
		if totalTxCount > 0 {
			historyBoost := 1.0 + math.Log10(float64(totalTxCount))*0.1
			result.Confidence = math.Min(result.Confidence*historyBoost, 100.0)
		}

		results = append(results, *result)
	}

	// Sort by confidence (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	return results, nil
}

func calculateConfidence(matches []MatchedIdentifier) float64 {
	if len(matches) == 0 {
		return 0
	}

	// Use cumulative scoring for multiple matches
	var confidence float64 = 0
	matchTypes := make(map[string]bool)

	for _, match := range matches {
		// Only count each type once
		if matchTypes[match.Type] {
			continue
		}
		matchTypes[match.Type] = true

		var weight float64
		switch match.Type {
		case string(extractor.TypeUPIVPA):
			weight = UPIVPAWeight * 100
		case string(extractor.TypePhone):
			weight = PhoneWeight * 100
		case string(extractor.TypeAccountNumber):
			weight = AccountNumberWeight * 100
		case string(extractor.TypeIMPSName):
			weight = IMPSNameWeight * 100
		case string(extractor.TypeNEFTName):
			weight = NEFTNameWeight * 100
		case string(extractor.TypeBankName):
			weight = BankNameWeight * 100
		default:
			weight = 50 // Unknown type, moderate confidence
		}

		// Cumulative scoring: each additional match adds diminishing value
		if confidence == 0 {
			confidence = weight
		} else {
			// Add remaining percentage of the weight
			remaining := 100 - confidence
			confidence += remaining * (weight / 100) * 0.5
		}
	}

	return math.Min(confidence, 100.0)
}

// MatchSingle finds the best matching party for a narration
func (m *Matcher) MatchSingle(ctx context.Context, narration string) (*MatchResult, error) {
	results, err := m.Match(ctx, narration)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// MatchWithBank finds parties matching the given narration filtered by bank
func (m *Matcher) MatchWithBank(ctx context.Context, narration string, bank string) ([]MatchResult, error) {
	// Extract identifiers from the narration
	identifiers := extractor.Extract(narration)

	var matches []sqlc.FindPartiesByIdentifierValuesAndBankRow

	// Only try identifier matching if we have identifiers
	if len(identifiers) > 0 {
		// Get unique values for database query
		values := make([]string, len(identifiers))
		for i, id := range identifiers {
			values[i] = id.Value
		}

		// Query database for matching parties filtered by bank
		var err error
		matches, err = m.queries.FindPartiesByIdentifierValuesAndBank(ctx, sqlc.FindPartiesByIdentifierValuesAndBankParams{
			Values: values,
			Bank:   bank,
		})
		if err != nil {
			return nil, err
		}
	}

	// If no identifier matches found, try fallback narration search
	if len(matches) == 0 {
		return m.matchByNarrationWithBank(ctx, narration, identifiers, bank)
	}

	// Group matches by party name (not ID) and calculate scores
	partyMatches := make(map[string]*MatchResult)

	for _, match := range matches {
		result, exists := partyMatches[match.Name]
		if !exists {
			result = &MatchResult{
				Party: sqlc.Party{
					ID:        match.ID,
					Name:      match.Name,
					Location:  match.Location,
					CreatedAt: match.CreatedAt,
				},
				PartyIDs:   []int64{match.ID},
				Confidence: 0,
				MatchedOn:  []MatchedIdentifier{},
			}
			partyMatches[match.Name] = result
		} else {
			// Add party ID if not already present
			if !containsInt64(result.PartyIDs, match.ID) {
				result.PartyIDs = append(result.PartyIDs, match.ID)
			}
		}

		// Add matched identifier (dedupe by type+value)
		identifier := MatchedIdentifier{
			Type:  match.MatchType,
			Value: match.MatchValue,
		}
		if !containsIdentifier(result.MatchedOn, identifier) {
			result.MatchedOn = append(result.MatchedOn, identifier)
		}
	}

	// Calculate confidence scores and fetch transaction stats
	results := make([]MatchResult, 0, len(partyMatches))

	for _, result := range partyMatches {
		// Calculate base confidence from identifier matches
		result.Confidence = calculateConfidence(result.MatchedOn)

		// Aggregate stats from all party IDs
		var totalTxCount int64
		var totalAmount float64
		var allRecentTxns []sqlc.Transaction

		for _, partyID := range result.PartyIDs {
			stats, err := m.queries.GetPartyWithTransactionCountByBank(ctx, sqlc.GetPartyWithTransactionCountByBankParams{
				Bank: bank,
				ID:   partyID,
			})
			if err == nil {
				totalTxCount += stats.TransactionCount
				if amount, ok := stats.TotalAmount.(float64); ok {
					totalAmount += amount
				}
			}

			// Get recent transactions for this party ID and bank
			recentTxns, err := m.queries.GetRecentTransactionsByPartyIDAndBank(ctx, sqlc.GetRecentTransactionsByPartyIDAndBankParams{
				PartyID: partyID,
				Bank:    bank,
				Limit:   5,
			})
			if err == nil {
				allRecentTxns = append(allRecentTxns, recentTxns...)
			}
		}

		result.TransactionCount = totalTxCount
		result.TotalAmount = totalAmount

		// Sort all recent transactions by date and limit to 5
		sort.Slice(allRecentTxns, func(i, j int) bool {
			return allRecentTxns[i].TransactionDate.After(allRecentTxns[j].TransactionDate)
		})
		if len(allRecentTxns) > 5 {
			allRecentTxns = allRecentTxns[:5]
		}
		result.RecentTxns = allRecentTxns

		// Apply history boost: 1.0 + log10(tx_count) * 0.1
		if totalTxCount > 0 {
			historyBoost := 1.0 + math.Log10(float64(totalTxCount))*0.1
			result.Confidence = math.Min(result.Confidence*historyBoost, 100.0)
		}

		results = append(results, *result)
	}

	// Sort by confidence (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	return results, nil
}

// matchByNarrationWithBank searches for parties by matching narration patterns filtered by bank
func (m *Matcher) matchByNarrationWithBank(ctx context.Context, narration string, identifiers []extractor.Identifier, bank string) ([]MatchResult, error) {
	// Build search patterns from extracted identifiers (e.g., IMPS names, NEFT names)
	var patterns []string
	for _, id := range identifiers {
		// Use IMPS names and NEFT names as search patterns
		if id.Type == extractor.TypeIMPSName || id.Type == extractor.TypeNEFTName {
			patterns = append(patterns, "%"+id.Value+"%")
		}
	}

	// If no good patterns, try to extract key parts from the narration itself
	if len(patterns) == 0 {
		// Extract the IMPS reference pattern if present (e.g., MMT/IMPS/529816026379)
		if strings.Contains(strings.ToUpper(narration), "MMT/IMPS/") {
			parts := strings.Split(narration, "/")
			for _, part := range parts {
				// Look for 12-digit IMPS reference numbers
				part = strings.TrimSpace(part)
				if len(part) == 12 {
					allDigits := true
					for _, c := range part {
						if c < '0' || c > '9' {
							allDigits = false
							break
						}
					}
					if allDigits {
						patterns = append(patterns, "%"+part+"%")
					}
				}
			}
		}
	}

	if len(patterns) == 0 {
		return nil, nil
	}

	// Query for each pattern and collect results
	// Group by party name (not ID)
	partyMatches := make(map[string]*MatchResult)

	for _, pattern := range patterns {
		matches, err := m.queries.FindPartiesByNarrationPatternAndBank(ctx, sqlc.FindPartiesByNarrationPatternAndBankParams{
			Narration: sql.NullString{String: pattern, Valid: true},
			Bank:      bank,
		})
		if err != nil {
			continue
		}

		for _, match := range matches {
			result, exists := partyMatches[match.Name]
			if !exists {
				partyMatches[match.Name] = &MatchResult{
					Party: sqlc.Party{
						ID:        match.ID,
						Name:      match.Name,
						Location:  match.Location,
						CreatedAt: match.CreatedAt,
					},
					PartyIDs:   []int64{match.ID},
					Confidence: 40, // Lower confidence for narration-based matches
					MatchedOn: []MatchedIdentifier{{
						Type:  "narration",
						Value: strings.TrimPrefix(strings.TrimSuffix(pattern, "%"), "%"),
					}},
				}
			} else {
				// Add party ID if not already present
				if !containsInt64(result.PartyIDs, match.ID) {
					result.PartyIDs = append(result.PartyIDs, match.ID)
				}
			}
		}
	}

	// Calculate final scores and fetch stats
	results := make([]MatchResult, 0, len(partyMatches))

	for _, result := range partyMatches {
		// Aggregate stats from all party IDs
		var totalTxCount int64
		var totalAmount float64
		var allRecentTxns []sqlc.Transaction

		for _, partyID := range result.PartyIDs {
			stats, err := m.queries.GetPartyWithTransactionCountByBank(ctx, sqlc.GetPartyWithTransactionCountByBankParams{
				Bank: bank,
				ID:   partyID,
			})
			if err == nil {
				totalTxCount += stats.TransactionCount
				if amount, ok := stats.TotalAmount.(float64); ok {
					totalAmount += amount
				}
			}

			// Get recent transactions for this party ID and bank
			recentTxns, err := m.queries.GetRecentTransactionsByPartyIDAndBank(ctx, sqlc.GetRecentTransactionsByPartyIDAndBankParams{
				PartyID: partyID,
				Bank:    bank,
				Limit:   5,
			})
			if err == nil {
				allRecentTxns = append(allRecentTxns, recentTxns...)
			}
		}

		result.TransactionCount = totalTxCount
		result.TotalAmount = totalAmount

		// Sort all recent transactions by date and limit to 5
		sort.Slice(allRecentTxns, func(i, j int) bool {
			return allRecentTxns[i].TransactionDate.After(allRecentTxns[j].TransactionDate)
		})
		if len(allRecentTxns) > 5 {
			allRecentTxns = allRecentTxns[:5]
		}
		result.RecentTxns = allRecentTxns

		// Apply history boost
		if totalTxCount > 0 {
			historyBoost := 1.0 + math.Log10(float64(totalTxCount))*0.1
			result.Confidence = math.Min(result.Confidence*historyBoost, 100.0)
		}

		results = append(results, *result)
	}

	// Sort by confidence (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	return results, nil
}

// matchByNarration searches for parties by matching narration patterns in transactions
// This is a fallback when no identifier matches are found
func (m *Matcher) matchByNarration(ctx context.Context, narration string, identifiers []extractor.Identifier) ([]MatchResult, error) {
	// Build search patterns from extracted identifiers (e.g., IMPS names, NEFT names)
	var patterns []string
	for _, id := range identifiers {
		// Use IMPS names and NEFT names as search patterns
		if id.Type == extractor.TypeIMPSName || id.Type == extractor.TypeNEFTName {
			patterns = append(patterns, "%"+id.Value+"%")
		}
	}

	// If no good patterns, try to extract key parts from the narration itself
	if len(patterns) == 0 {
		// Extract the IMPS reference pattern if present (e.g., MMT/IMPS/529816026379)
		if strings.Contains(strings.ToUpper(narration), "MMT/IMPS/") {
			parts := strings.Split(narration, "/")
			for _, part := range parts {
				// Look for 12-digit IMPS reference numbers
				part = strings.TrimSpace(part)
				if len(part) == 12 {
					allDigits := true
					for _, c := range part {
						if c < '0' || c > '9' {
							allDigits = false
							break
						}
					}
					if allDigits {
						patterns = append(patterns, "%"+part+"%")
					}
				}
			}
		}
	}

	if len(patterns) == 0 {
		return nil, nil
	}

	// Query for each pattern and collect results
	// Group by party name (not ID)
	partyMatches := make(map[string]*MatchResult)

	for _, pattern := range patterns {
		matches, err := m.queries.FindPartiesByNarrationPattern(ctx, sql.NullString{String: pattern, Valid: true})
		if err != nil {
			continue
		}

		for _, match := range matches {
			result, exists := partyMatches[match.Name]
			if !exists {
				partyMatches[match.Name] = &MatchResult{
					Party: sqlc.Party{
						ID:        match.ID,
						Name:      match.Name,
						Location:  match.Location,
						CreatedAt: match.CreatedAt,
					},
					PartyIDs:   []int64{match.ID},
					Confidence: 40, // Lower confidence for narration-based matches
					MatchedOn: []MatchedIdentifier{{
						Type:  "narration",
						Value: strings.TrimPrefix(strings.TrimSuffix(pattern, "%"), "%"),
					}},
				}
			} else {
				// Add party ID if not already present
				if !containsInt64(result.PartyIDs, match.ID) {
					result.PartyIDs = append(result.PartyIDs, match.ID)
				}
			}
		}
	}

	// Calculate final scores and fetch stats
	results := make([]MatchResult, 0, len(partyMatches))

	for _, result := range partyMatches {
		// Aggregate stats from all party IDs
		var totalTxCount int64
		var totalAmount float64
		var allRecentTxns []sqlc.Transaction

		for _, partyID := range result.PartyIDs {
			stats, err := m.queries.GetPartyWithTransactionCount(ctx, partyID)
			if err == nil {
				totalTxCount += stats.TransactionCount
				if stats.TotalAmount.Valid {
					totalAmount += stats.TotalAmount.Float64
				}
			}

			// Get recent transactions for this party ID
			recentTxns, err := m.queries.GetRecentTransactionsByPartyID(ctx, sqlc.GetRecentTransactionsByPartyIDParams{
				PartyID: partyID,
				Limit:   5,
			})
			if err == nil {
				allRecentTxns = append(allRecentTxns, recentTxns...)
			}
		}

		result.TransactionCount = totalTxCount
		result.TotalAmount = totalAmount

		// Sort all recent transactions by date and limit to 5
		sort.Slice(allRecentTxns, func(i, j int) bool {
			return allRecentTxns[i].TransactionDate.After(allRecentTxns[j].TransactionDate)
		})
		if len(allRecentTxns) > 5 {
			allRecentTxns = allRecentTxns[:5]
		}
		result.RecentTxns = allRecentTxns

		// Apply history boost
		if totalTxCount > 0 {
			historyBoost := 1.0 + math.Log10(float64(totalTxCount))*0.1
			result.Confidence = math.Min(result.Confidence*historyBoost, 100.0)
		}

		results = append(results, *result)
	}

	// Sort by confidence (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	return results, nil
}

// containsInt64 checks if a slice contains a value
func containsInt64(slice []int64, val int64) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// containsIdentifier checks if an identifier is already in the slice
func containsIdentifier(slice []MatchedIdentifier, id MatchedIdentifier) bool {
	for _, v := range slice {
		if v.Type == id.Type && v.Value == id.Value {
			return true
		}
	}
	return false
}

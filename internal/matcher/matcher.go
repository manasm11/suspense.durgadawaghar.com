package matcher

import (
	"context"
	"math"
	"sort"

	"suspense.durgadawaghar.com/internal/db/sqlc"
	"suspense.durgadawaghar.com/internal/extractor"
)

// MatchResult represents a party match with confidence score
type MatchResult struct {
	Party            sqlc.Party
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
	if len(identifiers) == 0 {
		return nil, nil
	}

	// Get unique values for database query
	values := make([]string, len(identifiers))
	for i, id := range identifiers {
		values[i] = id.Value
	}

	// Query database for matching parties
	matches, err := m.queries.FindPartiesByIdentifierValues(ctx, values)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, nil
	}

	// Group matches by party ID and calculate scores
	partyMatches := make(map[int64]*MatchResult)

	for _, match := range matches {
		result, exists := partyMatches[match.ID]
		if !exists {
			result = &MatchResult{
				Party: sqlc.Party{
					ID:        match.ID,
					Name:      match.Name,
					Location:  match.Location,
					CreatedAt: match.CreatedAt,
				},
				Confidence: 0,
				MatchedOn:  []MatchedIdentifier{},
			}
			partyMatches[match.ID] = result
		}

		// Add matched identifier
		result.MatchedOn = append(result.MatchedOn, MatchedIdentifier{
			Type:  match.MatchType,
			Value: match.MatchValue,
		})
	}

	// Calculate confidence scores and fetch transaction stats
	results := make([]MatchResult, 0, len(partyMatches))

	for _, result := range partyMatches {
		// Calculate base confidence from identifier matches
		result.Confidence = calculateConfidence(result.MatchedOn)

		// Get transaction stats
		stats, err := m.queries.GetPartyWithTransactionCount(ctx, result.Party.ID)
		if err == nil {
			result.TransactionCount = stats.TransactionCount
			result.TotalAmount = stats.TotalAmount

			// Apply history boost: 1.0 + log10(tx_count) * 0.1
			if stats.TransactionCount > 0 {
				historyBoost := 1.0 + math.Log10(float64(stats.TransactionCount))*0.1
				result.Confidence = math.Min(result.Confidence*historyBoost, 100.0)
			}
		}

		// Get recent transactions (limit 5)
		recentTxns, err := m.queries.GetRecentTransactionsByPartyID(ctx, result.Party.ID, 5)
		if err == nil {
			result.RecentTxns = recentTxns
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

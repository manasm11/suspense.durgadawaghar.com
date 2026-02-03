package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"suspense.durgadawaghar.com/internal/db/sqlc"
	"suspense.durgadawaghar.com/internal/extractor"
	"suspense.durgadawaghar.com/internal/matcher"
	"suspense.durgadawaghar.com/internal/parser"
	"suspense.durgadawaghar.com/internal/views/pages"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	queries *sqlc.Queries
	db      *sql.DB
	matcher *matcher.Matcher
}

// NewHandler creates a new Handler instance
func NewHandler(db *sql.DB) *Handler {
	queries := sqlc.New(db)
	return &Handler{
		queries: queries,
		db:      db,
		matcher: matcher.NewMatcher(queries),
	}
}

// Home renders the search page
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	pages.Home().Render(r.Context(), w)
}

// Search handles narration search requests
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	narration := r.FormValue("narration")
	if narration == "" {
		w.Write([]byte(`<div class="error">Please enter a narration to search.</div>`))
		return
	}

	results, err := h.matcher.Match(r.Context(), narration)
	if err != nil {
		w.Write([]byte(fmt.Sprintf(`<div class="error">Search error: %s</div>`, err.Error())))
		return
	}

	// Show extracted identifiers
	ids := extractor.Extract(narration)
	extractedIDs := make([]pages.ExtractedID, len(ids))
	for i, id := range ids {
		extractedIDs[i] = pages.ExtractedID{Type: string(id.Type), Value: id.Value}
	}

	pages.ExtractedIdentifiers(extractedIDs).Render(r.Context(), w)
	pages.SearchResults(results, narration).Render(r.Context(), w)
}

// Import renders the import page
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	pages.Import().Render(r.Context(), w)
}

// ImportPreview parses and previews import data
func (h *Handler) ImportPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := r.FormValue("data")
	yearStr := r.FormValue("year")
	year := 2025
	if y, err := strconv.Atoi(yearStr); err == nil {
		year = y
	}

	transactions := parser.Parse(data, year)

	previewTxns := make([]pages.PreviewTransaction, len(transactions))
	for i, tx := range transactions {
		ids := extractor.Extract(tx.Narration)
		previewIDs := make([]pages.PreviewIdentifier, len(ids))
		for j, id := range ids {
			previewIDs[j] = pages.PreviewIdentifier{Type: string(id.Type), Value: id.Value}
		}

		previewTxns[i] = pages.PreviewTransaction{
			Date:        tx.Date.Format("02 Jan 2006"),
			PartyName:   tx.PartyName,
			Location:    tx.Location,
			Amount:      fmt.Sprintf("%.2f", tx.Amount),
			PaymentMode: tx.PaymentMode,
			Identifiers: previewIDs,
		}
	}

	pages.ImportPreview(previewTxns, data, year).Render(r.Context(), w)
}

// ImportConfirm executes the import
func (h *Handler) ImportConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := r.FormValue("data")
	yearStr := r.FormValue("year")
	year := 2025
	if y, err := strconv.Atoi(yearStr); err == nil {
		year = y
	}

	transactions := parser.Parse(data, year)

	ctx := r.Context()
	imported := 0
	skipped := 0
	var errors []string

	for _, tx := range transactions {
		err := h.importTransaction(ctx, tx)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", tx.PartyName, err.Error()))
			skipped++
		} else {
			imported++
		}
	}

	pages.ImportResult(imported, skipped, errors).Render(r.Context(), w)
}

func (h *Handler) importTransaction(ctx context.Context, tx parser.Transaction) error {
	// Extract identifiers from narration
	ids := extractor.Extract(tx.Narration)

	// Try to find existing party by identifier
	var partyID int64
	for _, id := range ids {
		existing, err := h.queries.GetIdentifierByTypeValue(ctx, sqlc.GetIdentifierByTypeValueParams{
			Type:  string(id.Type),
			Value: id.Value,
		})
		if err == nil {
			partyID = existing.PartyID
			break
		}
	}

	// If no existing party found, create new one
	if partyID == 0 {
		party, err := h.queries.CreateParty(ctx, sqlc.CreatePartyParams{
			Name:     tx.PartyName,
			Location: sql.NullString{String: tx.Location, Valid: tx.Location != ""},
		})
		if err != nil {
			return fmt.Errorf("creating party: %w", err)
		}
		partyID = party.ID
	}

	// Insert identifiers (upsert - will update party_id if exists)
	for _, id := range ids {
		_, err := h.queries.CreateIdentifier(ctx, sqlc.CreateIdentifierParams{
			PartyID: partyID,
			Type:    string(id.Type),
			Value:   id.Value,
		})
		if err != nil {
			// Log but don't fail on identifier insert errors
			continue
		}
	}

	// Insert transaction
	_, err := h.queries.CreateTransaction(ctx, sqlc.CreateTransactionParams{
		PartyID:         partyID,
		Amount:          tx.Amount,
		TransactionDate: tx.Date,
		PaymentMode:     sql.NullString{String: tx.PaymentMode, Valid: tx.PaymentMode != ""},
		Narration:       sql.NullString{String: tx.Narration, Valid: tx.Narration != ""},
	})
	if err != nil {
		return fmt.Errorf("creating transaction: %w", err)
	}

	return nil
}

// PartyDetail shows a single party's details
func (h *Handler) PartyDetail(w http.ResponseWriter, r *http.Request) {
	// Extract party ID from path
	idStr := r.URL.Path[len("/party/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid party ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	party, err := h.queries.GetPartyWithTransactionCount(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	identifiers, _ := h.queries.GetIdentifiersByPartyID(ctx, id)
	transactions, _ := h.queries.GetTransactionsByPartyID(ctx, id)

	pages.PartyDetail(party, identifiers, transactions).Render(ctx, w)
}

// PartyList shows all parties
func (h *Handler) PartyList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	parties, err := h.queries.GetAllPartiesWithStats(ctx)
	if err != nil {
		http.Error(w, "Error loading parties", http.StatusInternalServerError)
		return
	}

	pages.PartyList(parties).Render(ctx, w)
}

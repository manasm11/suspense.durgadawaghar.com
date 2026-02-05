package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"suspense.durgadawaghar.com/internal/db/sqlc"
	"suspense.durgadawaghar.com/internal/extractor"
	"suspense.durgadawaghar.com/internal/matcher"
	"suspense.durgadawaghar.com/internal/parser"
	"suspense.durgadawaghar.com/internal/views/pages"
)

// errDuplicate is returned when a transaction already exists
var errDuplicate = errors.New("duplicate transaction")

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

	// Try to extract year from header first
	extractedYear := parser.ExtractYearFromHeader(data)

	// Determine which year to use
	year := time.Now().Year() // Default to current year
	if extractedYear > 0 {
		year = extractedYear
	}
	// User-provided year overrides extraction (if different from default)
	if y, err := strconv.Atoi(yearStr); err == nil && y != time.Now().Year() {
		year = y
		extractedYear = 0 // Don't show "auto-detected" if user overrode it
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

	pages.ImportPreview(previewTxns, data, year, extractedYear).Render(r.Context(), w)
}

// ImportConfirm executes the import
func (h *Handler) ImportConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := r.FormValue("data")
	yearStr := r.FormValue("year")

	// Use the year from the form (which was already set correctly in preview)
	year := time.Now().Year()
	if y, err := strconv.Atoi(yearStr); err == nil {
		year = y
	}

	transactions := parser.Parse(data, year)

	ctx := r.Context()
	imported := 0
	duplicates := 0
	var importErrors []string

	for _, tx := range transactions {
		err := h.importTransaction(ctx, tx)
		if err != nil {
			if errors.Is(err, errDuplicate) {
				duplicates++
			} else {
				importErrors = append(importErrors, fmt.Sprintf("%s: %s", tx.PartyName, err.Error()))
			}
		} else {
			imported++
		}
	}

	pages.ImportResult(imported, duplicates, importErrors).Render(r.Context(), w)
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
		// Check for UNIQUE constraint violation (SQLite error)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errDuplicate
		}
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

// ImportSaleBills renders the sale bill import form
func (h *Handler) ImportSaleBills(w http.ResponseWriter, r *http.Request) {
	pages.ImportSaleBills().Render(r.Context(), w)
}

// ImportSaleBillsPreview parses and previews sale bill import data
func (h *Handler) ImportSaleBillsPreview(w http.ResponseWriter, r *http.Request) {
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

	bills := parser.ParseSaleBills(data, year)

	previewBills := make([]pages.PreviewSaleBill, len(bills))
	for i, bill := range bills {
		previewBills[i] = pages.PreviewSaleBill{
			BillNumber: bill.BillNumber,
			Date:       bill.Date.Format("02 Jan 2006"),
			PartyName:  bill.PartyName,
			Amount:     fmt.Sprintf("%.2f", bill.Amount),
			IsCashSale: bill.IsCashSale,
		}
	}

	pages.ImportSaleBillsPreview(previewBills, data, year).Render(r.Context(), w)
}

// ImportSaleBillsConfirm executes the sale bill import
func (h *Handler) ImportSaleBillsConfirm(w http.ResponseWriter, r *http.Request) {
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

	bills := parser.ParseSaleBills(data, year)

	ctx := r.Context()
	imported := 0
	duplicates := 0
	var importErrors []string

	for _, bill := range bills {
		_, err := h.queries.CreateSaleBill(ctx, sqlc.CreateSaleBillParams{
			BillNumber: bill.BillNumber,
			BillDate:   bill.Date,
			PartyName:  bill.PartyName,
			Amount:     bill.Amount,
			IsCashSale: sql.NullBool{Bool: bill.IsCashSale, Valid: true},
		})
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				duplicates++
			} else {
				importErrors = append(importErrors, fmt.Sprintf("%s: %s", bill.BillNumber, err.Error()))
			}
		} else {
			imported++
		}
	}

	pages.ImportSaleBillsResult(imported, duplicates, importErrors).Render(r.Context(), w)
}

// SearchSaleBills renders the sale bill search form
func (h *Handler) SearchSaleBills(w http.ResponseWriter, r *http.Request) {
	// Default from date is 1 year ago, till date is today
	defaultFromDate := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	defaultTillDate := time.Now().Format("2006-01-02")
	pages.SearchSaleBills(defaultFromDate, defaultTillDate).Render(r.Context(), w)
}

// SearchSaleBillsResults executes the sale bill search
func (h *Handler) SearchSaleBillsResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	amountStr := r.FormValue("amount")
	variationStr := r.FormValue("variation")
	fromDateStr := r.FormValue("from_date")
	tillDateStr := r.FormValue("till_date")

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		w.Write([]byte(`<div class="error">Invalid amount.</div>`))
		return
	}

	variation := 0.0
	if v, err := strconv.ParseFloat(variationStr, 64); err == nil {
		variation = v
	}

	fromDate := time.Now().AddDate(-1, 0, 0)
	if fromDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", fromDateStr); err == nil {
			fromDate = parsed
		}
	}

	tillDate := time.Now()
	if tillDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", tillDateStr); err == nil {
			tillDate = parsed
		}
	}

	minAmount := amount - variation
	maxAmount := amount + variation

	bills, err := h.queries.SearchSaleBillsByAmountRange(r.Context(), sqlc.SearchSaleBillsByAmountRangeParams{
		Amount:     minAmount,
		Amount_2:   maxAmount,
		BillDate:   fromDate,
		BillDate_2: tillDate,
	})
	if err != nil {
		w.Write([]byte(fmt.Sprintf(`<div class="error">Search error: %s</div>`, err.Error())))
		return
	}

	results := make([]pages.SaleBillSearchResult, len(bills))
	for i, bill := range bills {
		isCash := false
		if bill.IsCashSale.Valid {
			isCash = bill.IsCashSale.Bool
		}
		results[i] = pages.SaleBillSearchResult{
			ID:         bill.ID,
			BillNumber: bill.BillNumber,
			Date:       bill.BillDate.Format("02 Jan 2006"),
			PartyName:  bill.PartyName,
			Amount:     fmt.Sprintf("%.2f", bill.Amount),
			IsCashSale: isCash,
		}
	}

	pages.SaleBillSearchResults(results, amountStr, variationStr).Render(r.Context(), w)
}

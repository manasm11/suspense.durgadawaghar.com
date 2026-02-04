# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Generate templ files and build
make build           # or: go generate ./... && go build -o bin/server ./cmd/server

# Run the server (generates templ first)
make run             # or: go run ./cmd/server

# Run tests
make test            # or: go test ./...

# Format code
make fmt             # runs: go fmt ./... && templ fmt .

# Install dependencies (templ, sqlc)
make deps

# Regenerate sqlc database code (after schema/query changes)
make sqlc            # or: sqlc generate
```

Server runs on port 8005 by default. Override with `-port` flag.

## Architecture Overview

This is a Go web application for managing suspense account transactions from receipt books. It parses transaction data from copied receipt book text, extracts payment identifiers (UPI VPAs, phone numbers, bank accounts), and automatically matches transactions to parties.

### Data Flow

1. **Parser** (`internal/parser/`) - Parses raw receipt book text into structured `Transaction` objects
   - Handles multiple receipt book formats (varies by month/year)
   - Detects payment modes (UPI, IMPS, NEFT, RTGS, CHEQUE, etc.)
   - Parses multi-party transactions (multiple parties sharing one bank entry)
   - Extracts party name and location from combined text

2. **Extractor** (`internal/extractor/`) - Extracts identifiers from transaction narrations
   - UPI VPAs (e.g., `user@ybl`)
   - Phone numbers (Indian 10-digit)
   - Bank account numbers (9-18 digits)
   - IFSC codes
   - IMPS names (sender/receiver names from IMPS transactions)
   - Bank names (extracted and normalized from IMPS narrations)

3. **Matcher** (`internal/matcher/`) - Matches transactions to parties using extracted identifiers
   - Bank-filtered matching (transactions matched within same bank)
   - Confidence scoring based on identifier type:
     - UPI VPA: 95%, Phone: 85%, Account: 80%, IMPS Name: 50%, Bank Name: 20%
   - History boost for parties with more transactions
   - Fallback narration search using IMPS names and reference numbers

4. **Handler** (`internal/handler/`) - HTTP handlers using templ templates
   - Import flow: paste text -> preview parsed data -> confirm import
   - Search by narration text to find matching parties (requires bank selection)
   - Bank-filtered queries for all party lookups

### Database

SQLite with sqlc-generated Go code. Schema in `internal/db/schema.sql`:
- `parties` - unique business entities
- `identifiers` - links parties to their UPI/phone/account/imps_name/bank_name identifiers
- `transactions` - imported receipt book entries (includes `bank` column for multi-bank support)

### Templating

Uses [templ](https://templ.guide/) for HTML templates. Templates are in `internal/views/`. Run `templ generate` (or `make templ`) after editing `.templ` files.

## Key Patterns

### Receipt Book Format

The parser handles receipt book text with this general structure:
```
DATE PARTICULARS DEBIT CREDIT
Apr 1 PARTY NAME LOCATION AMOUNT
BANK ACCOUNT LINE
NARRATION (UPI/NEFT/RTGS/etc details)
```

Multi-party transactions have multiple party lines before the bank account line.

### Adding New Locations

The parser maintains a list of known location indicators in `internal/parser/parser.go` (`locationIndicators` slice). Add new locations here when the parser fails to separate party name from location.

### Adding New Payment Modes

Payment mode detection uses regex patterns in `internal/parser/parser.go` (e.g., `upiModePattern`, `neftModePattern`). Add new patterns following the existing format.

### IMPS Format Patterns

The extractor (`internal/extractor/extractor.go`) handles multiple IMPS narration formats:
- Standard OK format: `MMT/IMPS/<ref>/OK/<name>/<bank>`
- Two names format: `MMT/IMPS/<ref>/<name1>/<name2>/<bank>`
- Secondary reference format: `MMT/IMPS/<ref>/<secondary_ref> /<name>/<bank>`
- P2A (Person to Account) format: `MMT/IMPS/<ref>/IMPS P2A <sender>/<receiver>/<bank>`

Bank names are normalized from truncated forms (e.g., "HDFC BAN" → "HDFC BANK"). See `bankNameMap` in extractor.go.

### Multi-Bank Support

Transactions are associated with a bank (extracted from the bank account line in receipt book). Currently, the UI limits bank selection to ICICI and HDFC. The `bank` field is used to filter searches and match parties within the same bank context.

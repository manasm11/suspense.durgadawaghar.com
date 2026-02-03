package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"

	_ "modernc.org/sqlite"

	"suspense.durgadawaghar.com/internal/handler"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dbPath := flag.String("db", "suspense.db", "SQLite database path")
	flag.Parse()

	// Initialize database
	db, err := initDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create handler
	h := handler.NewHandler(db)

	// Setup routes
	mux := http.NewServeMux()

	// Static files - serve from filesystem
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Pages
	mux.HandleFunc("/", h.Home)
	mux.HandleFunc("/search", h.Search)
	mux.HandleFunc("/import", h.Import)
	mux.HandleFunc("/import/preview", h.ImportPreview)
	mux.HandleFunc("/import/confirm", h.ImportConfirm)
	mux.HandleFunc("/party/", h.PartyDetail)
	mux.HandleFunc("/parties", h.PartyList)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Create schema using embedded SQL
	_, err = db.Exec(schemaSQL)
	if err != nil {
		// Tables might already exist, which is fine
		log.Printf("Schema exec (may be already applied): %v", err)
	}

	return db, nil
}

const schemaSQL = `
-- parties: stores unique business entities
CREATE TABLE IF NOT EXISTS parties (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    location TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- identifiers: normalized storage for UPI VPAs, phones, account numbers
CREATE TABLE IF NOT EXISTS identifiers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    party_id INTEGER NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('upi_vpa', 'phone', 'account_number', 'ifsc')),
    value TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, value)
);

-- transactions: imported receipt book entries
CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    party_id INTEGER NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    amount REAL NOT NULL,
    transaction_date DATE NOT NULL,
    payment_mode TEXT,
    narration TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_identifiers_value ON identifiers(value);
CREATE INDEX IF NOT EXISTS idx_identifiers_type_value ON identifiers(type, value);
CREATE INDEX IF NOT EXISTS idx_transactions_party_id ON transactions(party_id);
`

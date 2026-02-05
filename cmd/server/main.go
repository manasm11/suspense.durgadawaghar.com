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
	port := flag.Int("port", 8005, "HTTP server port")
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

	// Sale Bills
	mux.HandleFunc("/sale-bills/import", h.ImportSaleBills)
	mux.HandleFunc("/sale-bills/import/preview", h.ImportSaleBillsPreview)
	mux.HandleFunc("/sale-bills/import/confirm", h.ImportSaleBillsConfirm)
	mux.HandleFunc("/sale-bills/search", h.SearchSaleBills)
	mux.HandleFunc("/sale-bills/search/results", h.SearchSaleBillsResults)

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

	// Run migrations for existing databases
	if err := migrateDB(db); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func migrateDB(db *sql.DB) error {
	// Check if bank column exists and remove it
	_, err := db.Exec("SELECT bank FROM transactions LIMIT 1")
	if err == nil {
		// Bank column exists, need to drop it
		// SQLite doesn't support DROP COLUMN directly, need to recreate table
		log.Printf("Migration: Removing bank column from transactions table...")

		// Create new table without bank column
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS transactions_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				party_id INTEGER NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
				amount REAL NOT NULL,
				transaction_date DATE NOT NULL,
				payment_mode TEXT,
				narration TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("creating new transactions table: %w", err)
		}

		// Copy data (INSERT OR IGNORE handles duplicates from tighter unique constraint)
		_, err = db.Exec(`
			INSERT OR IGNORE INTO transactions_new (id, party_id, amount, transaction_date, payment_mode, narration, created_at)
			SELECT id, party_id, amount, transaction_date, payment_mode, narration, created_at FROM transactions
		`)
		if err != nil {
			return fmt.Errorf("copying transactions data: %w", err)
		}

		// Drop old table
		_, err = db.Exec("DROP TABLE transactions")
		if err != nil {
			return fmt.Errorf("dropping old transactions table: %w", err)
		}

		// Rename new table
		_, err = db.Exec("ALTER TABLE transactions_new RENAME TO transactions")
		if err != nil {
			return fmt.Errorf("renaming transactions table: %w", err)
		}

		// Recreate indexes
		_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_party_id ON transactions(party_id)")
		if err != nil {
			log.Printf("Migration: Warning - could not create party_id index: %v", err)
		}
		_, err = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique ON transactions(party_id, amount, transaction_date, payment_mode, narration)")
		if err != nil {
			log.Printf("Migration: Warning - could not create unique index: %v", err)
		}

		log.Printf("Migration: Removed bank column from transactions table")
	}

	// Migrate sale_bills table
	if err := migrateSaleBillsTable(db); err != nil {
		return fmt.Errorf("migrating sale_bills table: %w", err)
	}

	return nil
}

func migrateSaleBillsTable(db *sql.DB) error {
	// Check if sale_bills table exists by trying to query it
	_, err := db.Exec("SELECT id FROM sale_bills LIMIT 1")
	if err != nil {
		// Table doesn't exist, create it
		_, err = db.Exec(`
			CREATE TABLE sale_bills (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				bill_number TEXT NOT NULL,
				bill_date DATE NOT NULL,
				party_name TEXT NOT NULL,
				amount REAL NOT NULL,
				is_cash_sale BOOLEAN DEFAULT FALSE,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("creating sale_bills table: %w", err)
		}
		log.Printf("Migration: Created sale_bills table")

		// Create indexes
		_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_sale_bills_amount ON sale_bills(amount)")
		if err != nil {
			log.Printf("Migration: Warning - could not create amount index: %v", err)
		}
		_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_sale_bills_date ON sale_bills(bill_date)")
		if err != nil {
			log.Printf("Migration: Warning - could not create date index: %v", err)
		}
		_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_sale_bills_amount_date ON sale_bills(amount, bill_date)")
		if err != nil {
			log.Printf("Migration: Warning - could not create amount_date index: %v", err)
		}
		_, err = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sale_bills_unique ON sale_bills(bill_number, bill_date, party_name, amount)")
		if err != nil {
			log.Printf("Migration: Warning - could not create unique index: %v", err)
		}
	}
	return nil
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
    type TEXT NOT NULL CHECK (type IN ('upi_vpa', 'phone', 'account_number', 'ifsc', 'imps_name', 'bank_name', 'neft_name')),
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

-- sale_bills: imported sale bill entries
CREATE TABLE IF NOT EXISTS sale_bills (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_number TEXT NOT NULL,
    bill_date DATE NOT NULL,
    party_name TEXT NOT NULL,
    amount REAL NOT NULL,
    is_cash_sale BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sale_bills_amount ON sale_bills(amount);
CREATE INDEX IF NOT EXISTS idx_sale_bills_date ON sale_bills(bill_date);
CREATE INDEX IF NOT EXISTS idx_sale_bills_amount_date ON sale_bills(amount, bill_date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sale_bills_unique ON sale_bills(bill_number, bill_date, party_name, amount);
`

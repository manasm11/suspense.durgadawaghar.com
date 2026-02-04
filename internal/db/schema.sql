-- parties: stores unique business entities
CREATE TABLE parties (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    location TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- identifiers: normalized storage for UPI VPAs, phones, account numbers
CREATE TABLE identifiers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    party_id INTEGER NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('upi_vpa', 'phone', 'account_number', 'ifsc', 'imps_name', 'bank_name')),
    value TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, value)
);

-- transactions: imported receipt book entries
CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    party_id INTEGER NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    amount REAL NOT NULL,
    transaction_date DATE NOT NULL,
    payment_mode TEXT,
    narration TEXT,
    bank TEXT NOT NULL DEFAULT 'ICICI',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_identifiers_value ON identifiers(value);
CREATE INDEX idx_identifiers_type_value ON identifiers(type, value);
CREATE INDEX idx_transactions_party_id ON transactions(party_id);
CREATE INDEX idx_transactions_bank ON transactions(bank);

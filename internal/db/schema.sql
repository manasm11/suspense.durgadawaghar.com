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
    type TEXT NOT NULL CHECK (type IN ('upi_vpa', 'phone', 'account_number', 'ifsc', 'imps_name', 'bank_name', 'neft_name')),
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

-- Unique constraint to prevent duplicate transactions
CREATE UNIQUE INDEX idx_transactions_unique
ON transactions(party_id, amount, transaction_date, payment_mode, narration, bank);

-- sale_bills: imported sale bill entries
CREATE TABLE sale_bills (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bill_number TEXT NOT NULL,
    bill_date DATE NOT NULL,
    party_name TEXT NOT NULL,
    amount REAL NOT NULL,
    is_cash_sale BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_sale_bills_amount ON sale_bills(amount);
CREATE INDEX idx_sale_bills_date ON sale_bills(bill_date);
CREATE INDEX idx_sale_bills_amount_date ON sale_bills(amount, bill_date);
CREATE UNIQUE INDEX idx_sale_bills_unique ON sale_bills(bill_number, bill_date, party_name, amount);

-- Migration: Add unique constraint to prevent duplicate transactions
-- A duplicate is defined as having identical: party_id, amount, transaction_date, payment_mode, narration, bank

-- Delete duplicates, keeping the earliest entry (lowest id)
DELETE FROM transactions
WHERE id NOT IN (
    SELECT MIN(id) FROM transactions
    GROUP BY party_id, amount, transaction_date, payment_mode, narration, bank
);

-- Add unique constraint to prevent future duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique
ON transactions(party_id, amount, transaction_date, payment_mode, narration, bank);

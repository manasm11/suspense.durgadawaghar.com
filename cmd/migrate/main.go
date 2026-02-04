package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "./suspense.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check for duplicates first
	var dupeCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM transactions t1
		WHERE EXISTS (
			SELECT 1 FROM transactions t2
			WHERE t2.id < t1.id
			AND t2.party_id = t1.party_id
			AND t2.amount = t1.amount
			AND t2.transaction_date = t1.transaction_date
			AND COALESCE(t2.payment_mode, '') = COALESCE(t1.payment_mode, '')
			AND COALESCE(t2.narration, '') = COALESCE(t1.narration, '')
			AND t2.bank = t1.bank
		)`).Scan(&dupeCount)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Duplicates found: %d\n", dupeCount)

	// Delete duplicates, keeping the earliest entry (lowest id)
	result, err := db.Exec(`DELETE FROM transactions
		WHERE id NOT IN (
			SELECT MIN(id) FROM transactions
			GROUP BY party_id, amount, transaction_date, payment_mode, narration, bank
		)`)
	if err != nil {
		log.Fatal("Error deleting duplicates:", err)
	}
	deleted, _ := result.RowsAffected()
	fmt.Printf("Deleted %d duplicate transactions\n", deleted)

	// Check if index already exists
	var indexExists int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
		WHERE type='index' AND name='idx_transactions_unique'`).Scan(&indexExists)
	if err != nil {
		log.Fatal(err)
	}

	if indexExists > 0 {
		fmt.Println("Unique index already exists")
	} else {
		// Add unique constraint to prevent future duplicates
		_, err = db.Exec(`CREATE UNIQUE INDEX idx_transactions_unique
			ON transactions(party_id, amount, transaction_date, payment_mode, narration, bank)`)
		if err != nil {
			log.Fatal("Error creating unique index:", err)
		}
		fmt.Println("Created unique index idx_transactions_unique")
	}

	fmt.Println("Migration complete!")
}

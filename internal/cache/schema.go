package cache

import (
	"context"
	"database/sql"
	"fmt"
)

// migrate creates the cache database schema if it does not already exist.
// Each statement is executed separately inside a transaction to avoid relying on
// multi-statement support, which is driver-dependent.
func migrate(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // superseded by Commit; rollback error is irrelevant on success

	if _, err := tx.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS filings (
			ch_number    TEXT    NOT NULL,
			doc_id       TEXT    NOT NULL,
			local_path   TEXT    NOT NULL,
			content_type TEXT    NOT NULL,
			file_size    INTEGER NOT NULL,
			cached_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (ch_number, doc_id)
		)`,
	); err != nil {
		return fmt.Errorf("create filings table: %w", err)
	}

	// Reserved for future identifier resolution (ticker/ISIN → ch_number via OpenFIGI/GLEIF).
	if _, err := tx.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS identifiers (
			name_query   TEXT    NOT NULL PRIMARY KEY,
			ch_number    TEXT,
			isin         TEXT,
			lei          TEXT,
			looked_up_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	); err != nil {
		return fmt.Errorf("create identifiers table: %w", err)
	}

	return tx.Commit()
}

package cache

import (
	"context"
	"database/sql"
	"fmt"
)

// migrate creates the cache database schema if it does not already exist.
// Each statement is executed separately inside a transaction to avoid relying on
// multi-statement support, which is driver-dependent.
func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // superseded by Commit; rollback error is irrelevant on success

	if _, err := tx.ExecContext(ctx, `
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
	if _, err := tx.ExecContext(ctx, `
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

	// zip_entries indexes all documents extracted from a zip archive filing.
	// Each (ch_number, doc_id, filename) triple is unique. The primary document
	// (is_primary=1) is also indexed in filings for backward compatibility with Get.
	// total_in_archive is the count of non-directory entries in the source archive
	// before the MaxEntries cap. It is identical for every row of a given
	// (ch_number, doc_id) pair — stored redundantly on each row so that any single-row
	// query (e.g. SELECT … LIMIT 1) can read it without a JOIN or sub-query.
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS zip_entries (
			ch_number        TEXT    NOT NULL,
			doc_id           TEXT    NOT NULL,
			filename         TEXT    NOT NULL,
			local_path       TEXT    NOT NULL,
			content_type     TEXT    NOT NULL,
			file_size        INTEGER NOT NULL,
			is_primary       INTEGER NOT NULL DEFAULT 0,
			total_in_archive INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (ch_number, doc_id, filename)
		)`,
	); err != nil {
		return fmt.Errorf("create zip_entries table: %w", err)
	}

	return tx.Commit()
}

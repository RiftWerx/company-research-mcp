// Package cache implements local storage for downloaded company filings.
// It owns the cache directory layout, file I/O, and the SQLite filing index.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver for database/sql
)

// cacheSubDir is the path under BaseDir where filing files are stored.
const cacheSubDir = "cache/uk"

// dbFileName is the name of the SQLite database file within BaseDir.
const dbFileName = "index.db"

// NewDefaultConfig returns the default Cache configuration.
func NewDefaultConfig() Config {
	return Config{}
}

// Config holds the configuration for the Cache.
// The zero value is valid; empty fields are replaced with defaults.
type Config struct {
	// BaseDir is the root directory for cached files and the index database.
	// Default: {os.UserCacheDir()}/company-research.mcp
	// (e.g. ~/.cache/company-research.mcp on Linux,
	//  ~/Library/Caches/company-research.mcp on macOS,
	//  %LOCALAPPDATA%\company-research.mcp on Windows)
	BaseDir string
}

// Cache stores downloaded filings on disk and indexes them in SQLite.
// Construct with New; the zero value is not usable.
type Cache struct {
	db      *sql.DB
	baseDir string
}

// New opens or creates the cache at the configured BaseDir.
// The SQLite database and cache directories are created if they do not exist.
func New(cfg Config) (*Cache, error) {
	baseDir := cfg.BaseDir
	if baseDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("resolve cache dir: %w", err)
		}
		baseDir = filepath.Join(cacheDir, "company-research.mcp")
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(baseDir, dbFileName))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// WAL mode improves write performance and allows readers and writers to proceed
	// concurrently without blocking each other. This is not required for stdio MCP
	// transport (which is single-threaded), but makes the cache safer if the server
	// is ever extended to support an HTTP/SSE transport with concurrent tool calls.
	if _, err := db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Cache{db: db, baseDir: baseDir}, nil
}

// Put stores body as a local file and records the filing in the index.
// If an entry for (chNumber, docID) already exists it is overwritten.
// Returns the local file path and the number of bytes written.
//
// The write is atomic: body is written to a temp file in the same directory
// and renamed over the final path only after a successful close. This prevents
// a partially-written file from being visible as a valid cache entry.
func (c *Cache) Put(ctx context.Context, chNumber, docID, contentType string, body io.Reader) (localPath string, written int64, err error) {
	dir := filepath.Join(c.baseDir, cacheSubDir, chNumber, docID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create cache dir: %w", err)
	}

	finalPath := filepath.Join(dir, "filing"+fileExt(contentType))

	// Write to a temp file in the same directory so os.Rename is atomic (same fs).
	tmp, err := os.CreateTemp(dir, "filing-*.tmp")
	if err != nil {
		return "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	written, copyErr := io.Copy(tmp, body)
	closeErr := tmp.Close() // close before Rename — required on Windows
	switch {
	case copyErr != nil:
		os.Remove(tmpName)
		return "", 0, fmt.Errorf("write file: %w", copyErr)
	case closeErr != nil:
		os.Remove(tmpName)
		return "", 0, fmt.Errorf("close temp file: %w", closeErr)
	}

	if err := os.Rename(tmpName, finalPath); err != nil {
		os.Remove(tmpName)
		return "", 0, fmt.Errorf("commit file: %w", err)
	}

	_, err = c.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO filings (ch_number, doc_id, local_path, content_type, file_size)
		 VALUES (?, ?, ?, ?, ?)`,
		chNumber, docID, finalPath, contentType, written,
	)
	if err != nil {
		os.Remove(finalPath)
		return "", 0, fmt.Errorf("index filing: %w", err)
	}

	return finalPath, written, nil
}

// Get looks up a previously cached filing by company number and document ID.
// Returns found=false if the filing is not indexed or the local file has been deleted.
func (c *Cache) Get(ctx context.Context, chNumber, docID string) (localPath, contentType string, fileSize int64, found bool, err error) {
	row := c.db.QueryRowContext(ctx,
		`SELECT local_path, content_type, file_size FROM filings WHERE ch_number = ? AND doc_id = ?`,
		chNumber, docID,
	)
	scanErr := row.Scan(&localPath, &contentType, &fileSize)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return "", "", 0, false, nil //nolint:nilerr // sql.ErrNoRows is not a failure; found=false signals the cache miss
	}
	if scanErr != nil {
		return "", "", 0, false, fmt.Errorf("query filing: %w", scanErr)
	}

	if _, err := os.Stat(localPath); err != nil {
		return "", "", 0, false, nil //nolint:nilerr // file missing from disk is a cache miss, not a failure
	}

	return localPath, contentType, fileSize, true, nil
}

// Clear removes cached filings from disk and deletes their index records.
// If chNumber is non-empty only that company's filings are removed; otherwise all filings are removed.
// Returns the number of deleted files, freed bytes, and deleted database records.
func (c *Cache) Clear(ctx context.Context, chNumber string) (deletedFiles, freedBytes, dbRecords int64, err error) {
	targetDir := filepath.Join(c.baseDir, cacheSubDir)
	if chNumber != "" {
		targetDir = filepath.Join(targetDir, chNumber)
	}

	// Count files before removal.
	_ = filepath.WalkDir(targetDir, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // non-existent or inaccessible entry — treat as empty
		}
		if !d.IsDir() {
			if info, infoErr := d.Info(); infoErr == nil {
				deletedFiles++
				freedBytes += info.Size()
			}
		}
		return nil
	})

	if err := os.RemoveAll(targetDir); err != nil {
		return 0, 0, 0, fmt.Errorf("remove cache dir: %w", err)
	}

	var result sql.Result
	if chNumber != "" {
		result, err = c.db.ExecContext(ctx, `DELETE FROM filings WHERE ch_number = ?`, chNumber)
	} else {
		result, err = c.db.ExecContext(ctx, `DELETE FROM filings`)
	}
	if err != nil {
		return deletedFiles, freedBytes, 0, fmt.Errorf("delete index records: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return deletedFiles, freedBytes, 0, fmt.Errorf("rows affected: %w", err)
	}
	return deletedFiles, freedBytes, rows, nil
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

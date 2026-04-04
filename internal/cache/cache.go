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
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver for database/sql
)

// cacheSubDir is the path under BaseDir where filing files are stored.
const cacheSubDir = "cache/uk"

// dbFileName is the name of the SQLite database file within BaseDir.
const dbFileName = "index.db"

// maxFileSizeBytes is the maximum permitted size for a single cached filing document.
// Responses larger than this limit are rejected to prevent unbounded disk usage.
const maxFileSizeBytes int64 = 200 * 1024 * 1024 // 200 MiB

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
	root    *os.Root
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

	root, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("open cache root: %w", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(baseDir, dbFileName))
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}

	// WAL mode improves write performance and allows readers and writers to proceed
	// concurrently without blocking each other. This is not required for stdio MCP
	// transport (which is single-threaded), but makes the cache safer if the server
	// is ever extended to support an HTTP/SSE transport with concurrent tool calls.
	if _, err := db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		_ = root.Close()
		_ = db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := migrate(context.Background(), db); err != nil {
		_ = root.Close()
		_ = db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Cache{db: db, root: root, baseDir: baseDir}, nil
}

// Put stores body as a local file and records the filing in the index.
// If an entry for (chNumber, docID) already exists it is overwritten.
// Returns the local file path and the number of bytes written.
//
// The write is atomic: body is written to a temp file in the same directory
// and renamed over the final path only after a successful close. This prevents
// a partially-written file from being visible as a valid cache entry.
func (c *Cache) Put(ctx context.Context, chNumber, docID, contentType string, body io.Reader) (localPath string, written int64, err error) {
	relDir := filepath.Join(cacheSubDir, chNumber, docID)
	if err := c.root.MkdirAll(relDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create cache dir: %w", err)
	}

	relFinal := filepath.Join(relDir, "filing"+fileExt(contentType))
	finalPath := filepath.Join(c.baseDir, relFinal) // absolute path stored in the DB and returned to callers

	// Write to a temp file in the same directory so Rename is atomic (same fs).
	relTmp := filepath.Join(relDir, fmt.Sprintf("filing-%016x.tmp", rand.Uint64())) //nolint:gosec // G404: temp file suffix does not require cryptographic randomness
	tmp, err := c.root.OpenFile(relTmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("create temp file: %w", err)
	}

	// Allow one extra byte so lr.N == 0 unambiguously signals the body exceeded the limit.
	lr := &io.LimitedReader{R: body, N: maxFileSizeBytes + 1}
	written, copyErr := io.Copy(tmp, lr)
	closeErr := tmp.Close() // close before Rename — required on Windows
	switch {
	case copyErr != nil:
		_ = c.root.Remove(relTmp)
		return "", 0, fmt.Errorf("write file: %w", copyErr)
	case closeErr != nil:
		_ = c.root.Remove(relTmp)
		return "", 0, fmt.Errorf("close temp file: %w", closeErr)
	case lr.N == 0:
		// LimitedReader exhausted — body is at least maxFileSizeBytes+1 bytes.
		_ = c.root.Remove(relTmp)
		return "", 0, fmt.Errorf("filing exceeds %d-byte size limit", maxFileSizeBytes)
	}

	if err := c.root.Rename(relTmp, relFinal); err != nil {
		_ = c.root.Remove(relTmp)
		return "", 0, fmt.Errorf("commit file: %w", err)
	}

	_, err = c.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO filings (ch_number, doc_id, local_path, content_type, file_size)
		 VALUES (?, ?, ?, ?, ?)`,
		chNumber, docID, finalPath, contentType, written,
	)
	if err != nil {
		_ = c.root.Remove(relFinal)
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

	// Convert the stored absolute path to a root-relative path for the existence check.
	// localPath is always written by Put as filepath.Join(c.baseDir, ...) so Rel succeeds.
	relPath, relErr := filepath.Rel(c.baseDir, localPath)
	if relErr != nil || strings.HasPrefix(relPath, "..") {
		return "", "", 0, false, nil //nolint:nilerr // path outside cache root — treat as miss
	}
	if _, statErr := c.root.Stat(relPath); statErr != nil {
		return "", "", 0, false, nil //nolint:nilerr // file missing from disk is a cache miss, not a failure
	}

	return localPath, contentType, fileSize, true, nil
}

// Clear removes cached filings from disk and deletes their index records.
// If chNumber is non-empty only that company's filings are removed; otherwise all filings are removed.
// Returns the number of deleted files, freed bytes, and deleted database records.
//
// The index is updated before files are removed so that a failed file removal
// leaves the cache in a consistent state (index and disk match).
func (c *Cache) Clear(ctx context.Context, chNumber string) (deletedFiles, freedBytes, dbRecords int64, err error) {
	var result sql.Result
	if chNumber != "" {
		result, err = c.db.ExecContext(ctx, `DELETE FROM filings WHERE ch_number = ?`, chNumber)
	} else {
		result, err = c.db.ExecContext(ctx, `DELETE FROM filings`)
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("delete index records: %w", err)
	}

	dbRecords, err = result.RowsAffected()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("rows affected: %w", err)
	}

	targetDir := cacheSubDir
	if chNumber != "" {
		targetDir = filepath.Join(cacheSubDir, chNumber)
	}

	// Count files and bytes before removal, treating a missing directory as empty.
	_ = fs.WalkDir(c.root.FS(), targetDir, func(_ string, d fs.DirEntry, walkErr error) error {
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

	if err := c.root.RemoveAll(targetDir); err != nil {
		return deletedFiles, freedBytes, dbRecords, fmt.Errorf("remove cache dir: %w", err)
	}

	return deletedFiles, freedBytes, dbRecords, nil
}

// Close closes the underlying root and database connection.
func (c *Cache) Close() error {
	return errors.Join(c.root.Close(), c.db.Close())
}

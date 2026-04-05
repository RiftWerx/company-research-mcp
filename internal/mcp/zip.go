package mcp

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strings"

	"github.com/riftwerx/company-research-mcp/internal/mime"
)

// errZipTooLarge is returned by readZipBody when the zip body exceeds the size limit.
var errZipTooLarge = errors.New("zip filing exceeds maximum size limit")

// safeInt64Size converts a uint64 byte count to int64 for use with io.LimitedReader.
// If n exceeds math.MaxInt64 it returns math.MaxInt64, which is the largest limit
// LimitedReader can enforce.
func safeInt64Size(n uint64) int64 {
	if n >= math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(n)
}

// readZipBody reads at most maxBytes of zip data from r.
// Returns errZipTooLarge if the body exceeds maxBytes, or a wrapped error on read failure.
func readZipBody(r io.Reader, maxBytes uint64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: safeInt64Size(maxBytes) + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("read zip body: %w", err)
	}
	if lr.N == 0 {
		return nil, errZipTooLarge
	}
	return data, nil
}

// extractFromZip selects the primary document from a zip archive and returns its
// content, base filename, and MIME type. Selection priority: largest .xhtml →
// largest .html → largest .pdf → largest file of any type.
//
// Returns an error if the total uncompressed size of all entries exceeds maxBytes
// (zip bomb defence), the archive is malformed, or it contains no files.
func extractFromZip(zipData []byte, maxBytes uint64) (content []byte, filename string, contentType string, err error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, "", "", fmt.Errorf("open zip: %w", err)
	}

	// Zip bomb defence: check total uncompressed size before extracting anything.
	var totalUncompressed uint64
	for _, f := range r.File {
		if !f.FileInfo().IsDir() {
			totalUncompressed += f.UncompressedSize64
		}
	}
	if totalUncompressed > maxBytes {
		return nil, "", "", fmt.Errorf("zip uncompressed content (%d bytes) exceeds %d-byte limit", totalUncompressed, maxBytes)
	}

	// Priority tiers for document selection.
	const (
		tierXHTML = 0
		tierHTML  = 1
		tierPDF   = 2
		tierOther = 3
	)

	type candidate struct {
		f    *zip.File
		tier int
	}

	var best *candidate

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(path.Ext(f.Name))
		var tier int
		switch ext {
		case ".xhtml":
			tier = tierXHTML
		case ".html", ".htm":
			tier = tierHTML
		case ".pdf":
			tier = tierPDF
		default:
			tier = tierOther
		}

		if best == nil ||
			tier < best.tier ||
			(tier == best.tier && f.UncompressedSize64 > best.f.UncompressedSize64) {
			best = &candidate{f: f, tier: tier}
		}
	}

	if best == nil {
		return nil, "", "", fmt.Errorf("zip contains no files")
	}

	rc, err := best.f.Open()
	if err != nil {
		return nil, "", "", fmt.Errorf("open zip entry: %w", err)
	}
	defer rc.Close()

	// A second size guard: UncompressedSize64 in the zip header is written by the
	// archive creator and can be falsified (e.g. set to 0 while the entry actually
	// expands to gigabytes). Reading through a LimitedReader ensures extraction is
	// bounded even if the header-sum check above was defeated.
	lr := &io.LimitedReader{R: rc, N: safeInt64Size(maxBytes) + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, "", "", fmt.Errorf("read zip entry: %w", err)
	}
	if lr.N == 0 {
		return nil, "", "", fmt.Errorf("zip entry exceeds %d-byte limit", maxBytes)
	}

	ext := strings.ToLower(path.Ext(best.f.Name))
	return data, path.Base(best.f.Name), mime.TypeFromExt(ext), nil
}

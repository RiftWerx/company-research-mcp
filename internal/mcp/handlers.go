package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/riftwerx/company-research-mcp/internal/cache"
	"github.com/riftwerx/company-research-mcp/internal/companyhouse"
	"github.com/riftwerx/company-research-mcp/internal/xbrl"
)

// CompanyHouseService is the subset of companyhouse.Service that MCP handlers require.
type CompanyHouseService interface {
	SearchCompanies(ctx context.Context, query string, maxResults int) ([]companyhouse.SearchResult, error)
	GetCompanyProfile(ctx context.Context, chNumber string) (*companyhouse.CompanyProfile, error)
	GetFilingHistory(ctx context.Context, chNumber string, opts companyhouse.ListFilingsOptions) ([]companyhouse.Filing, error)
	GetDocument(ctx context.Context, documentURL string) (*companyhouse.Document, error)
}

// FilingCache is the subset of cache.Cache that MCP handlers require.
type FilingCache interface {
	Get(ctx context.Context, chNumber, docID string) (*cache.FilingEntry, error)
	Put(ctx context.Context, chNumber, docID, contentType, filename string, body io.Reader) (localPath string, written int64, err error)
	Clear(ctx context.Context, chNumber string) (cache.ClearResult, error)
	ValidatePath(path string) (string, error)
}

// defaultSearchLimit is the maximum number of search results returned when the caller does not specify a limit.
const defaultSearchLimit = 10

// defaultFilingsLimit is the maximum number of filings returned when the caller does not specify a limit.
const defaultFilingsLimit = 20

// searchResult is the minimal per-company response for search_company.
type searchResult struct {
	CHNumber string `json:"ch_number"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Locality string `json:"locality,omitempty"`
}

// profileAddress is the address sub-object in get_company_profile responses.
type profileAddress struct {
	Line1    string `json:"line1,omitempty"`
	Line2    string `json:"line2,omitempty"`
	Locality string `json:"locality,omitempty"`
	Postcode string `json:"postcode,omitempty"`
	Country  string `json:"country,omitempty"`
}

// profileResult is the minimal response for get_company_profile.
type profileResult struct {
	CHNumber       string         `json:"ch_number"`
	Name           string         `json:"name"`
	Status         string         `json:"status"`
	Type           string         `json:"type"`
	DateOfCreation string         `json:"date_of_creation,omitempty"`
	SICCodes       []string       `json:"sic_codes"`
	Address        profileAddress `json:"address"`
}

// filingResult is the minimal per-filing response for list_filings.
type filingResult struct {
	TransactionID string `json:"transaction_id"`
	Type          string `json:"type"`
	Description   string `json:"description"`
	Date          string `json:"date"` // YYYY-MM-DD
	DocumentURL   string `json:"document_url"`
}

// fetchResult is the response for fetch_filing and get_latest.
type fetchResult struct {
	LocalPath     string `json:"local_path"`
	ContentType   string `json:"content_type"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	Source        string `json:"source"`
}

// clearCacheResult is the response for clear_cache.
type clearCacheResult struct {
	DeletedFiles     int64 `json:"deleted_files"`
	FreedBytes       int64 `json:"freed_bytes"`
	DBRecordsRemoved int64 `json:"db_records_removed"`
}

// handleSearchCompany implements the search_company tool.
func (s *Server) handleSearchCompany(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return toolError("query is required")
	}
	limit := req.GetInt("limit", defaultSearchLimit)

	results, err := s.chSvc.SearchCompanies(ctx, query, limit)
	if err != nil {
		return toolResultForCHError(err, "search companies")
	}

	out := make([]searchResult, len(results))
	for i, r := range results {
		out[i] = searchResult{
			CHNumber: r.CompanyNumber,
			Name:     r.Title,
			Status:   r.CompanyStatus,
			Type:     r.CompanyType,
			Locality: r.Address.Locality,
		}
	}
	return toolResultJSON(out)
}

// handleGetCompanyProfile implements the get_company_profile tool.
func (s *Server) handleGetCompanyProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chNumber, err := req.RequireString("ch_number")
	if err != nil {
		return toolError("ch_number is required")
	}
	if !companyhouse.ValidateCHNumber(chNumber) {
		return toolError("ch_number contains invalid characters")
	}

	profile, err := s.chSvc.GetCompanyProfile(ctx, chNumber)
	if err != nil {
		return toolResultForCHError(err, "get company profile")
	}

	sicCodes := profile.SICCodes
	if sicCodes == nil {
		sicCodes = []string{}
	}
	out := profileResult{
		CHNumber:       profile.CompanyNumber,
		Name:           profile.CompanyName,
		Status:         profile.CompanyStatus,
		Type:           profile.CompanyType,
		DateOfCreation: profile.DateOfCreation,
		SICCodes:       sicCodes,
		Address: profileAddress{
			Line1:    profile.RegisteredOffice.AddressLine1,
			Line2:    profile.RegisteredOffice.AddressLine2,
			Locality: profile.RegisteredOffice.Locality,
			Postcode: profile.RegisteredOffice.PostalCode,
			Country:  profile.RegisteredOffice.Country,
		},
	}
	return toolResultJSON(out)
}

// handleListFilings implements the list_filings tool.
func (s *Server) handleListFilings(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chNumber, err := req.RequireString("ch_number")
	if err != nil {
		return toolError("ch_number is required")
	}
	if !companyhouse.ValidateCHNumber(chNumber) {
		return toolError("ch_number contains invalid characters")
	}
	category := req.GetString("category", "")
	start := req.GetInt("start", 0)
	limit := req.GetInt("limit", defaultFilingsLimit)

	filings, err := s.chSvc.GetFilingHistory(ctx, chNumber, companyhouse.ListFilingsOptions{
		Category:     category,
		StartIndex:   start,
		ItemsPerPage: limit,
	})
	if err != nil {
		return toolResultForCHError(err, "list filings")
	}

	// Omit filings that have no downloadable document — they cannot be used with
	// fetch_filing and would only produce confusing errors if the LLM tried.
	out := make([]filingResult, 0, len(filings))
	for _, f := range filings {
		if f.DocumentURL == "" {
			continue
		}
		date := ""
		if !f.Date.IsZero() {
			date = f.Date.Format("2006-01-02")
		}
		out = append(out, filingResult{
			TransactionID: f.TransactionID,
			Type:          f.Type,
			Description:   f.Description,
			Date:          date,
			DocumentURL:   f.DocumentURL,
		})
	}
	return toolResultJSON(out)
}

// handleFetchFiling implements the fetch_filing tool.
func (s *Server) handleFetchFiling(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chNumber, err := req.RequireString("ch_number")
	if err != nil {
		return toolError("ch_number is required")
	}
	if !companyhouse.ValidateCHNumber(chNumber) {
		return toolError("ch_number contains invalid characters")
	}
	documentURL, err := req.RequireString("document_url")
	if err != nil {
		return toolError("document_url is required")
	}
	return s.fetchDocument(ctx, chNumber, documentURL)
}

// handleGetLatest implements the get_latest tool.
func (s *Server) handleGetLatest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chNumber, err := req.RequireString("ch_number")
	if err != nil {
		return toolError("ch_number is required")
	}
	if !companyhouse.ValidateCHNumber(chNumber) {
		return toolError("ch_number contains invalid characters")
	}
	category, err := req.RequireString("category")
	if err != nil {
		return toolError("category is required")
	}

	filings, err := s.chSvc.GetFilingHistory(ctx, chNumber, companyhouse.ListFilingsOptions{
		Category:     category,
		ItemsPerPage: 1,
	})
	if err != nil {
		return toolResultForCHError(err, "list filings")
	}
	if len(filings) == 0 {
		return toolError("no filings found for that category")
	}
	if filings[0].DocumentURL == "" {
		return toolError("most recent filing in that category has no downloadable document")
	}

	return s.fetchDocument(ctx, chNumber, filings[0].DocumentURL)
}

// fetchDocument retrieves a filing from the cache or downloads it from CH.
// Returns a cached result immediately if available; otherwise fetches from CH and stores it.
func (s *Server) fetchDocument(ctx context.Context, chNumber, documentURL string) (*mcp.CallToolResult, error) {
	if !companyhouse.ValidateDocumentURL(documentURL) {
		return toolError("document_url must be a Companies House document API URL (document-api.company-information.service.gov.uk)")
	}

	docID, ok := companyhouse.ParseDocumentID(documentURL)
	if !ok {
		return toolError("document_url does not contain a recognisable CH document ID (.../document/{id} or .../document/{id}/content)")
	}
	if !companyhouse.ValidateDocID(docID) {
		return toolError("document_url contains an invalid document ID")
	}

	entry, err := s.cache.Get(ctx, chNumber, docID)
	if err != nil {
		return nil, fmt.Errorf("check cache: %w", err)
	}
	if entry != nil {
		return toolResultJSON(fetchResult{
			LocalPath:     entry.LocalPath,
			ContentType:   entry.ContentType,
			FileSizeBytes: entry.FileSize,
			Source:        "cache",
		})
	}

	doc, err := s.chSvc.GetDocument(ctx, documentURL)
	if err != nil {
		return toolResultForCHError(err, "fetch document")
	}
	defer doc.Body.Close() // captures original body; safe to replace doc.Body below

	// Detect zip by Content-Type first; fall back to magic bytes PK\x03\x04.
	peek := make([]byte, 4)
	n, _ := io.ReadFull(doc.Body, peek)
	peeked := peek[:n]
	isZip := doc.ContentType == "application/zip" ||
		(n >= 4 && peeked[0] == 'P' && peeked[1] == 'K' && peeked[2] == 0x03 && peeked[3] == 0x04)
	// Reconstruct the body so the peeked bytes are not consumed.
	doc.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peeked), doc.Body))

	cacheFilename := ""
	if isZip {
		zipData, readErr := readZipBody(doc.Body, cache.MaxFileSizeBytes)
		if errors.Is(readErr, errZipTooLarge) {
			// Too-large is a user-facing condition; other read errors are unexpected and propagate.
			return toolError(readErr.Error())
		}
		if readErr != nil {
			return nil, fmt.Errorf("read zip: %w", readErr)
		}
		extracted, extractedName, extractedType, extractErr := extractFromZip(zipData, cache.MaxFileSizeBytes)
		if extractErr != nil {
			return toolError(fmt.Sprintf("unpack zip: %s", extractErr))
		}
		doc.Body = io.NopCloser(bytes.NewReader(extracted))
		doc.ContentType = extractedType
		cacheFilename = extractedName
	}

	localPath, written, err := s.cache.Put(ctx, chNumber, docID, doc.ContentType, cacheFilename, doc.Body)
	if err != nil {
		return nil, fmt.Errorf("cache document: %w", err)
	}

	return toolResultJSON(fetchResult{
		LocalPath:     localPath,
		ContentType:   doc.ContentType,
		FileSizeBytes: written,
		Source:        "companies_house",
	})
}

// handleClearCache implements the clear_cache tool.
func (s *Server) handleClearCache(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chNumber := req.GetString("ch_number", "")
	if chNumber != "" && !companyhouse.ValidateCHNumber(chNumber) {
		return toolError("ch_number contains invalid characters")
	}

	cleared, err := s.cache.Clear(ctx, chNumber)
	if err != nil {
		return nil, fmt.Errorf("clear cache: %w", err)
	}

	return toolResultJSON(clearCacheResult{
		DeletedFiles:     cleared.DeletedFiles,
		FreedBytes:       cleared.FreedBytes,
		DBRecordsRemoved: cleared.DBRecords,
	})
}

// handleExtractXBRLFacts implements the extract_xbrl_facts tool.
func (s *Server) handleExtractXBRLFacts(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	localPath, err := req.RequireString("local_path")
	if err != nil {
		return toolError("local_path is required")
	}
	ext := strings.ToLower(filepath.Ext(localPath))
	if ext != ".xhtml" && ext != ".html" {
		return toolError("local_path must point to an .xhtml or .html file")
	}

	// Resolve symlinks and verify the path is within the cache file subtree.
	// This prevents reading arbitrary files or escaping via symlinks.
	realPath, pathErr := s.cache.ValidatePath(localPath)
	if errors.Is(pathErr, cache.ErrOutsideCache) {
		return toolError("local_path is not within the cache directory")
	}
	if pathErr != nil {
		return toolError("local_path does not point to a readable file")
	}

	info, statErr := os.Stat(realPath)
	if statErr != nil || !info.Mode().IsRegular() {
		return toolError("local_path does not point to a readable file")
	}

	opts := xbrl.Options{
		NamePrefix:       req.GetString("name_prefix", ""),
		IncludeTextFacts: req.GetBool("include_text_facts", false),
	}
	facts, truncated, parseErr := xbrl.ParseFacts(realPath, opts)
	if parseErr != nil {
		return toolError(fmt.Sprintf("parse iXBRL: %s", parseErr))
	}
	return toolResultJSON(xbrlFactsResult{Facts: facts, Count: len(facts), Truncated: truncated})
}

// xbrlFactsResult is the response envelope for extract_xbrl_facts.
// Truncated is true when the document contained more facts than the MaxFacts cap;
// callers should use name_prefix to narrow the query when this occurs.
type xbrlFactsResult struct {
	Facts     []xbrl.Fact `json:"facts"`
	Count     int         `json:"count"`
	Truncated bool        `json:"truncated"`
}

// toolResultForCHError maps CH sentinel errors to tool error results.
// Returns (errResult, nil) for known errors, (nil, wrappedErr) for unexpected errors.
func toolResultForCHError(err error, op string) (*mcp.CallToolResult, error) {
	if errors.Is(err, companyhouse.ErrNotFound) {
		return toolError("not found")
	}
	if errors.Is(err, companyhouse.ErrUnauthorized) {
		return toolError("CH API key invalid or missing")
	}
	if errors.Is(err, companyhouse.ErrRateLimited) {
		return toolError("CH rate limit hit, retry shortly")
	}
	return nil, fmt.Errorf("%s: %w", op, err)
}

// toolError wraps a user-facing message as a tool error result.
// MCP tool input and validation errors are signalled as tool error results (IsError=true),
// not as Go errors, so the MCP client receives a structured error rather than a transport failure.
func toolError(msg string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(msg), nil
}

// toolResultJSON marshals v to JSON and wraps it in a text tool result.
func toolResultJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}

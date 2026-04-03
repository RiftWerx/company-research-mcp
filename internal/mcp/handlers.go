package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/riftwerx/company-research-mcp/internal/companyhouse"
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
	Get(ctx context.Context, chNumber, docID string) (localPath, contentType string, fileSize int64, found bool, err error)
	Put(ctx context.Context, chNumber, docID, contentType string, body io.Reader) (localPath string, written int64, err error)
	Clear(ctx context.Context, chNumber string) (deletedFiles, freedBytes, dbRecords int64, err error)
}

// defaultSearchLimit is the maximum number of search results returned when the caller does not specify a limit.
const defaultSearchLimit = 10

// defaultFilingsLimit is the maximum number of filings returned when the caller does not specify a limit.
const defaultFilingsLimit = 20

// chDocumentAPIHost is the only hostname from which filing documents may be fetched.
// document_url inputs are validated against this domain to prevent SSRF.
const chDocumentAPIHost = "document-api.company-information.service.gov.uk"

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
	CHNumber string         `json:"ch_number"`
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Type     string         `json:"type"`
	SICCodes []string       `json:"sic_codes"`
	Address  profileAddress `json:"address"`
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
		return mcp.NewToolResultError("query is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
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
		return mcp.NewToolResultError("ch_number is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
	}

	profile, err := s.chSvc.GetCompanyProfile(ctx, chNumber)
	if err != nil {
		return toolResultForCHError(err, "get company profile")
	}

	out := profileResult{
		CHNumber: profile.CompanyNumber,
		Name:     profile.CompanyName,
		Status:   profile.CompanyStatus,
		Type:     profile.CompanyType,
		SICCodes: profile.SICCodes,
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
		return mcp.NewToolResultError("ch_number is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
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
		return mcp.NewToolResultError("ch_number is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
	}
	documentURL, err := req.RequireString("document_url")
	if err != nil {
		return mcp.NewToolResultError("document_url is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
	}
	return s.fetchDocument(ctx, chNumber, documentURL)
}

// handleGetLatest implements the get_latest tool.
func (s *Server) handleGetLatest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chNumber, err := req.RequireString("ch_number")
	if err != nil {
		return mcp.NewToolResultError("ch_number is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
	}
	category, err := req.RequireString("category")
	if err != nil {
		return mcp.NewToolResultError("category is required"), nil //nolint:nilerr // MCP tool input errors are returned as tool error results, not Go errors
	}

	filings, err := s.chSvc.GetFilingHistory(ctx, chNumber, companyhouse.ListFilingsOptions{
		Category:     category,
		ItemsPerPage: 1,
	})
	if err != nil {
		return toolResultForCHError(err, "list filings")
	}
	if len(filings) == 0 {
		return mcp.NewToolResultError("no filings found for that category"), nil
	}
	if filings[0].DocumentURL == "" {
		return mcp.NewToolResultError("most recent filing in that category has no downloadable document"), nil
	}

	return s.fetchDocument(ctx, chNumber, filings[0].DocumentURL)
}

// fetchDocument retrieves a filing from the cache or downloads it from CH.
// Returns a cached result immediately if available; otherwise fetches from CH and stores it.
func (s *Server) fetchDocument(ctx context.Context, chNumber, documentURL string) (*mcp.CallToolResult, error) {
	if !isAllowedDocumentURL(documentURL) {
		return mcp.NewToolResultError("document_url must be a Companies House document API URL (document-api.company-information.service.gov.uk)"), nil
	}

	docID, ok := docIDFromURL(documentURL)
	if !ok {
		return mcp.NewToolResultError("document_url does not contain a recognisable CH document ID (.../document/{id} or .../document/{id}/content)"), nil
	}

	localPath, contentType, fileSize, found, err := s.cache.Get(ctx, chNumber, docID)
	if err != nil {
		return nil, fmt.Errorf("check cache: %w", err)
	}
	if found {
		return toolResultJSON(fetchResult{
			LocalPath:     localPath,
			ContentType:   contentType,
			FileSizeBytes: fileSize,
			Source:        "cache",
		})
	}

	doc, err := s.chSvc.GetDocument(ctx, documentURL)
	if err != nil {
		return toolResultForCHError(err, "fetch document")
	}
	defer doc.Body.Close()

	localPath, written, err := s.cache.Put(ctx, chNumber, docID, doc.ContentType, doc.Body)
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

	deleted, freed, dbRecs, err := s.cache.Clear(ctx, chNumber)
	if err != nil {
		return nil, fmt.Errorf("clear cache: %w", err)
	}

	return toolResultJSON(clearCacheResult{
		DeletedFiles:     deleted,
		FreedBytes:       freed,
		DBRecordsRemoved: dbRecs,
	})
}

// toolResultForCHError maps CH sentinel errors to tool error results.
// Returns (errResult, nil) for known errors, (nil, wrappedErr) for unexpected errors.
func toolResultForCHError(err error, op string) (*mcp.CallToolResult, error) {
	if errors.Is(err, companyhouse.ErrNotFound) {
		return mcp.NewToolResultError("not found"), nil
	}
	if errors.Is(err, companyhouse.ErrUnauthorized) {
		return mcp.NewToolResultError("CH API key invalid or missing"), nil
	}
	if errors.Is(err, companyhouse.ErrRateLimited) {
		return mcp.NewToolResultError("CH rate limit hit, retry shortly"), nil
	}
	return nil, fmt.Errorf("%s: %w", op, err)
}

// toolResultJSON marshals v to JSON and wraps it in a text tool result.
func toolResultJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}

// isAllowedDocumentURL returns true if rawURL is a valid CH document API URL.
// Absolute URLs (those with a scheme or host) must use HTTPS and resolve to
// chDocumentAPIHost. Relative paths (no scheme, no host) are accepted as-is;
// they originate from the CH API and are not valid targets for SSRF.
func isAllowedDocumentURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "" || u.Host != "" {
		return u.Scheme == "https" && u.Hostname() == chDocumentAPIHost
	}
	return true // relative path — not externally addressable
}

// docIDFromURL extracts the document ID from a CH document URL.
// Handles both the metadata URL form (.../document/{id}) and the content URL form
// (.../document/{id}/content). Returns the ID and true on success, or "", false if
// the URL cannot be parsed or does not contain a "document" path segment followed by an ID.
func docIDFromURL(documentURL string) (string, bool) {
	u, err := url.Parse(documentURL)
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "document" && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1], true
		}
	}
	return "", false
}

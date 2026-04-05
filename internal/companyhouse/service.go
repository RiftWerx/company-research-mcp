package companyhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.company-information.service.gov.uk"

// HTTPDoer is the HTTP transport interface required by Service.
// *client.Client satisfies this interface.
type HTTPDoer interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

// Service wraps the Companies House REST API.
// Construct with New; the zero value is not usable.
type Service struct {
	http    HTTPDoer
	baseURL string
	apiKey  string
}

// New constructs a Service using the given HTTP client and CH API key.
func New(http HTTPDoer, apiKey string) *Service {
	return newWithBaseURL(http, apiKey, defaultBaseURL)
}

// newWithBaseURL constructs a Service with a custom base URL. Used in tests.
func newWithBaseURL(http HTTPDoer, apiKey, baseURL string) *Service {
	return &Service{
		http:    http,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
	}
}

// SearchCompanies searches for companies by name or partial name.
// maxResults controls the page size; pass 0 to use the CH API default (20).
func (s *Service) SearchCompanies(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	params := url.Values{"q": {query}}
	if maxResults > 0 {
		params.Set("items_per_page", strconv.Itoa(maxResults))
	}

	resp, err := s.get(ctx, "/search/companies?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("search companies: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Items []struct {
			CompanyNumber           string `json:"company_number"`
			Title                   string `json:"title"`
			CompanyType             string `json:"company_type"`
			CompanyStatus           string `json:"company_status"`
			DateOfCreation          string `json:"date_of_creation"`
			RegisteredOfficeAddress struct {
				AddressLine1 string `json:"address_line_1"`
				AddressLine2 string `json:"address_line_2"`
				Locality     string `json:"locality"`
				PostalCode   string `json:"postal_code"`
				Country      string `json:"country"`
			} `json:"registered_office_address"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("search companies: decode response: %w", err)
	}

	results := make([]SearchResult, len(body.Items))
	for i, item := range body.Items {
		results[i] = SearchResult{
			CompanyNumber:  item.CompanyNumber,
			Title:          item.Title,
			CompanyType:    item.CompanyType,
			CompanyStatus:  item.CompanyStatus,
			DateOfCreation: item.DateOfCreation,
			Address: RegisteredAddress{
				AddressLine1: item.RegisteredOfficeAddress.AddressLine1,
				AddressLine2: item.RegisteredOfficeAddress.AddressLine2,
				Locality:     item.RegisteredOfficeAddress.Locality,
				PostalCode:   item.RegisteredOfficeAddress.PostalCode,
				Country:      item.RegisteredOfficeAddress.Country,
			},
		}
	}
	return results, nil
}

// GetCompanyProfile returns the full profile for the given Companies House number.
func (s *Service) GetCompanyProfile(ctx context.Context, chNumber string) (*CompanyProfile, error) {
	resp, err := s.get(ctx, "/company/"+padCHNumber(chNumber))
	if err != nil {
		return nil, fmt.Errorf("get company profile: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		CompanyNumber           string   `json:"company_number"`
		CompanyName             string   `json:"company_name"`
		CompanyType             string   `json:"type"`
		CompanyStatus           string   `json:"company_status"`
		DateOfCreation          string   `json:"date_of_creation"`
		SICCodes                []string `json:"sic_codes"`
		RegisteredOfficeAddress struct {
			AddressLine1 string `json:"address_line_1"`
			AddressLine2 string `json:"address_line_2"`
			Locality     string `json:"locality"`
			PostalCode   string `json:"postal_code"`
			Country      string `json:"country"`
		} `json:"registered_office_address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("get company profile: decode response: %w", err)
	}

	return &CompanyProfile{
		CompanyNumber:  body.CompanyNumber,
		CompanyName:    body.CompanyName,
		CompanyType:    body.CompanyType,
		CompanyStatus:  body.CompanyStatus,
		DateOfCreation: body.DateOfCreation,
		SICCodes:       body.SICCodes,
		RegisteredOffice: RegisteredAddress{
			AddressLine1: body.RegisteredOfficeAddress.AddressLine1,
			AddressLine2: body.RegisteredOfficeAddress.AddressLine2,
			Locality:     body.RegisteredOfficeAddress.Locality,
			PostalCode:   body.RegisteredOfficeAddress.PostalCode,
			Country:      body.RegisteredOfficeAddress.Country,
		},
	}, nil
}

// GetFilingHistory returns the filing history for a company.
// Use opts to filter by category, or paginate results.
func (s *Service) GetFilingHistory(ctx context.Context, chNumber string, opts ListFilingsOptions) ([]Filing, error) {
	params := url.Values{}
	if opts.Category != "" {
		params.Set("category", opts.Category)
	}
	if opts.StartIndex > 0 {
		params.Set("start_index", strconv.Itoa(opts.StartIndex))
	}
	if opts.ItemsPerPage > 0 {
		params.Set("items_per_page", strconv.Itoa(opts.ItemsPerPage))
	}

	path := "/company/" + padCHNumber(chNumber) + "/filing-history"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := s.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("get filing history: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Items []struct {
			TransactionID string `json:"transaction_id"`
			Type          string `json:"type"`
			Description   string `json:"description"`
			Date          string `json:"date"`
			Links         struct {
				DocumentMetadata string `json:"document_metadata"`
			} `json:"links"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("get filing history: decode response: %w", err)
	}

	filings := make([]Filing, 0, len(body.Items))
	for _, item := range body.Items {
		date, err := time.Parse("2006-01-02", item.Date)
		if err != nil {
			// Unparseable date: include the filing with zero time rather than dropping it.
			date = time.Time{}
		}
		filings = append(filings, Filing{
			TransactionID: item.TransactionID,
			Type:          item.Type,
			Description:   item.Description,
			Date:          date,
			DocumentURL:   item.Links.DocumentMetadata,
		})
	}
	return filings, nil
}

// GetDocument downloads a document using its metadata URL (Filing.DocumentURL).
// It appends "/content" to resolve the downloadable file; the 302 redirect to
// the actual file is followed transparently by the HTTP client.
//
// The caller is responsible for closing Document.Body.
func (s *Service) GetDocument(ctx context.Context, documentURL string) (*Document, error) {
	// Strip any existing /content suffix before appending; callers may pass either
	// the metadata URL form (.../document/{id}) or the content URL form
	// (.../document/{id}/content) — both should resolve to the same download URL.
	base := strings.TrimSuffix(strings.TrimRight(documentURL, "/"), "/content")
	contentURL := base + "/content"
	resp, err := retryOnRateLimit(ctx, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, contentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.SetBasicAuth(s.apiKey, "")
		// xhtml is listed first so CH prefers iXBRL/zip responses for companies that support
		// them; the MCP layer transparently extracts the primary document from the zip.
		req.Header.Set("Accept", "application/xhtml+xml,application/pdf,*/*")
		return s.http.Do(ctx, req)
	})
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	resp, err = checkStatus(resp) //nolint:bodyclose // checkStatus closes the body on error; on success body is transferred to Document.Body
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return &Document{
		Body:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
	}, nil
}

// get executes an authenticated GET request to the given path.
// If the server responds with 429 it waits for the Retry-After duration and
// retries once. A second 429 is returned as ErrRateLimited.
func (s *Service) get(ctx context.Context, path string) (*http.Response, error) {
	resp, err := retryOnRateLimit(ctx, func() (*http.Response, error) {
		return s.execute(ctx, path)
	})
	if err != nil {
		return nil, err
	}
	return checkStatus(resp)
}

// retryOnRateLimit calls op and, if the response is 429, waits for the Retry-After
// duration and calls op once more. A second 429 is passed through to the caller.
// The timer is stopped promptly if ctx is cancelled.
func retryOnRateLimit(ctx context.Context, op func() (*http.Response, error)) (*http.Response, error) {
	resp, err := op()
	if err != nil || resp.StatusCode != http.StatusTooManyRequests {
		return resp, err
	}

	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	resp.Body.Close()

	timer := time.NewTimer(retryAfter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}

	return op()
}

// execute builds and sends a single authenticated GET request. It does not
// inspect the response status — callers are responsible for that.
func (s *Service) execute(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.SetBasicAuth(s.apiKey, "")
	return s.http.Do(ctx, req)
}

// checkStatus maps well-known CH API HTTP status codes to sentinel errors.
// It closes resp.Body on error; on success the caller owns the body.
func checkStatus(resp *http.Response) (*http.Response, error) {
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusUnauthorized:
		resp.Body.Close()
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		resp.Body.Close()
		return nil, ErrNotFound
	case http.StatusTooManyRequests:
		resp.Body.Close()
		return nil, ErrRateLimited
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("CH API %d", resp.StatusCode)
	}
}

// parseRetryAfter parses the Retry-After response header (integer seconds).
// Returns 5 seconds as a conservative default when the header is absent or unparseable.
func parseRetryAfter(header string) time.Duration {
	if header != "" {
		if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return 5 * time.Second
}

// padCHNumber zero-pads a Companies House company number to 8 characters.
// The CH API requires this format (e.g. "12345" → "00012345").
// Numbers with letter prefixes (e.g. "SC123456" for Scottish companies) are always
// 8 characters and pass through unchanged.
func padCHNumber(n string) string {
	if len(n) >= 8 {
		return n
	}
	return strings.Repeat("0", 8-len(n)) + n
}

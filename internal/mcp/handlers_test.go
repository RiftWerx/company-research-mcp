package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/riftwerx/company-research-mcp/internal/cache"
	"github.com/riftwerx/company-research-mcp/internal/companyhouse"
)

// mockCHService is a testify mock for CompanyHouseService.
type mockCHService struct {
	mock.Mock
}

func (m *mockCHService) SearchCompanies(ctx context.Context, query string, limit int) ([]companyhouse.SearchResult, error) {
	args := m.Called(ctx, query, limit)
	results, _ := args.Get(0).([]companyhouse.SearchResult)
	return results, args.Error(1)
}

func (m *mockCHService) GetCompanyProfile(ctx context.Context, number string) (*companyhouse.CompanyProfile, error) {
	args := m.Called(ctx, number)
	profile, _ := args.Get(0).(*companyhouse.CompanyProfile)
	return profile, args.Error(1)
}

func (m *mockCHService) GetFilingHistory(ctx context.Context, number string, opts companyhouse.ListFilingsOptions) ([]companyhouse.Filing, error) {
	args := m.Called(ctx, number, opts)
	filings, _ := args.Get(0).([]companyhouse.Filing)
	return filings, args.Error(1)
}

func (m *mockCHService) GetDocument(ctx context.Context, url string) (*companyhouse.Document, error) {
	args := m.Called(ctx, url)
	doc, _ := args.Get(0).(*companyhouse.Document)
	return doc, args.Error(1)
}

// mockFilingCache is a testify mock for FilingCache.
type mockFilingCache struct {
	mock.Mock
}

func (m *mockFilingCache) Get(ctx context.Context, chNumber, docID string) (*cache.FilingEntry, error) {
	args := m.Called(ctx, chNumber, docID)
	entry, _ := args.Get(0).(*cache.FilingEntry)
	return entry, args.Error(1)
}

func (m *mockFilingCache) Put(ctx context.Context, chNumber, docID, contentType, filename string, body io.Reader) (string, int64, error) {
	args := m.Called(ctx, chNumber, docID, contentType, filename, body)
	localPath, _ := args.Get(0).(string)
	written, _ := args.Get(1).(int64)
	return localPath, written, args.Error(2)
}

func (m *mockFilingCache) Clear(ctx context.Context, chNumber string) (cache.ClearResult, error) {
	args := m.Called(ctx, chNumber)
	result, _ := args.Get(0).(cache.ClearResult)
	return result, args.Error(1)
}

// callTool is a test helper that calls the given handler with the provided arguments.
func callTool(handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return handler(context.Background(), req)
}

// isToolError returns true if the result is a tool error result.
func isToolError(r *mcp.CallToolResult) bool {
	if r == nil {
		return false
	}
	return r.IsError
}

// resultText extracts the text payload from the first content item of a tool result.
func resultText(r *mcp.CallToolResult) string {
	if len(r.Content) == 0 {
		return ""
	}
	tc, ok := r.Content[0].(mcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}

// newTestServer builds a Server with the given CH service and a blank cache mock.
// Tests that exercise the cache should build the Server directly with their own mockFilingCache.
func newTestServer(svc CompanyHouseService) *Server {
	return New(svc, &mockFilingCache{})
}

func TestHandleSearchCompany(t *testing.T) {
	t.Parallel()

	t.Run("should return results for a valid query", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("SearchCompanies", mock.Anything, "Tesco", defaultSearchLimit).Return(
			[]companyhouse.SearchResult{
				{CompanyNumber: "00445790", Title: "TESCO PLC", CompanyStatus: "active", CompanyType: "plc"},
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleSearchCompany, map[string]any{"query": "Tesco"})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
	})

	t.Run("should return a tool error when query is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleSearchCompany, map[string]any{})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when company is not found", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("SearchCompanies", mock.Anything, "NoSuchCompany", defaultSearchLimit).Return(nil, companyhouse.ErrNotFound)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleSearchCompany, map[string]any{"query": "NoSuchCompany"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when unauthorized", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("SearchCompanies", mock.Anything, "Tesco", defaultSearchLimit).Return(nil, companyhouse.ErrUnauthorized)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleSearchCompany, map[string]any{"query": "Tesco"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when rate limited", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("SearchCompanies", mock.Anything, "Tesco", defaultSearchLimit).Return(nil, companyhouse.ErrRateLimited)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleSearchCompany, map[string]any{"query": "Tesco"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should propagate unexpected errors", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("SearchCompanies", mock.Anything, "Tesco", defaultSearchLimit).Return(nil, errors.New("network failure"))
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		_, err := callTool(srv.handleSearchCompany, map[string]any{"query": "Tesco"})

		// Assert
		assert.Error(t, err)
	})
}

func TestHandleGetCompanyProfile(t *testing.T) {
	t.Parallel()

	t.Run("should return the profile for a valid company number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetCompanyProfile", mock.Anything, "00445790").Return(
			&companyhouse.CompanyProfile{
				CompanyNumber: "00445790",
				CompanyName:   "TESCO PLC",
				CompanyStatus: "active",
				CompanyType:   "plc",
				SICCodes:      []string{"47110"},
				RegisteredOffice: companyhouse.RegisteredAddress{
					AddressLine1: "Tesco House",
					Locality:     "Welwyn Garden City",
					PostalCode:   "AL7 1GA",
				},
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleGetCompanyProfile, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
	})

	t.Run("should return a tool error when company is not found", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetCompanyProfile", mock.Anything, "99999999").Return(nil, companyhouse.ErrNotFound)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleGetCompanyProfile, map[string]any{"ch_number": "99999999"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error for an invalid ch_number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act — path traversal attempt
		result, err := callTool(srv.handleGetCompanyProfile, map[string]any{"ch_number": "../../etc/passwd"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when ch_number is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleGetCompanyProfile, map[string]any{})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should propagate unexpected errors", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetCompanyProfile", mock.Anything, "00445790").Return(nil, errors.New("network failure"))
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		_, err := callTool(srv.handleGetCompanyProfile, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.Error(t, err)
	})
}

func TestHandleListFilings(t *testing.T) {
	t.Parallel()

	t.Run("should return filings for a valid company number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetFilingHistory", mock.Anything, "00445790", companyhouse.ListFilingsOptions{
			ItemsPerPage: defaultFilingsLimit,
		}).Return(
			[]companyhouse.Filing{
				{
					TransactionID: "MzI1MDk3NjkxOGFkaXF6a2N4",
					Type:          "AA",
					Description:   "full accounts made up to 25 February 2024",
					Date:          time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC),
					DocumentURL:   "https://document-api.company-information.service.gov.uk/document/abc123",
				},
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleListFilings, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
	})

	t.Run("should return a tool error when ch_number is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleListFilings, map[string]any{})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error for an invalid ch_number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleListFilings, map[string]any{"ch_number": "../../etc"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should propagate unexpected errors", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetFilingHistory", mock.Anything, "00445790", companyhouse.ListFilingsOptions{
			ItemsPerPage: defaultFilingsLimit,
		}).Return(nil, errors.New("network failure"))
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		_, err := callTool(srv.handleListFilings, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.Error(t, err)
	})
}

func TestHandleFetchFiling(t *testing.T) {
	t.Parallel()

	// docURL uses the metadata URL form — as returned by list_filings / GetFilingHistory.
	const docURL = "https://document-api.company-information.service.gov.uk/document/abc123"

	t.Run("should return a cached document when already stored", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return(&cache.FilingEntry{LocalPath: "/cache/filing.pdf", ContentType: "application/pdf", FileSize: int64(100)}, nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out fetchResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.Equal(t, "cache", out.Source)
		assert.Equal(t, "/cache/filing.pdf", out.LocalPath)
	})

	t.Run("should download and cache a document on cache miss", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetDocument", mock.Anything, docURL).Return(
			&companyhouse.Document{
				Body:        io.NopCloser(strings.NewReader("PDF content")),
				ContentType: "application/pdf",
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return((*cache.FilingEntry)(nil), nil)
		fc.On("Put", mock.Anything, "00445790", "abc123", "application/pdf", "", mock.Anything).Return("/cache/filing.pdf", int64(11), nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out fetchResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.Equal(t, "companies_house", out.Source)
	})

	t.Run("should return a tool error when document is not found", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetDocument", mock.Anything, docURL).Return(nil, companyhouse.ErrNotFound)
		defer svc.AssertExpectations(t)
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return((*cache.FilingEntry)(nil), nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when ch_number is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{"document_url": docURL})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when document_url is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error for an invalid ch_number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "../../etc/passwd",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should extract primary xhtml from a zip response", func(t *testing.T) {
		t.Parallel()

		// Arrange
		xhtmlContent := "<html><body>iXBRL content</body></html>"
		zipBody := buildZip(t, [][2]string{
			{"dir/report-2024-T01.xhtml", xhtmlContent},
		})
		svc := &mockCHService{}
		svc.On("GetDocument", mock.Anything, docURL).Return(
			&companyhouse.Document{
				Body:        io.NopCloser(bytes.NewReader(zipBody)),
				ContentType: "application/zip",
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return((*cache.FilingEntry)(nil), nil)
		fc.On("Put", mock.Anything, "00445790", "abc123", "application/xhtml+xml", "report-2024-T01.xhtml", mock.Anything).
			Return("/cache/report-2024-T01.xhtml", int64(len(xhtmlContent)), nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out fetchResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.Equal(t, "application/xhtml+xml", out.ContentType)
		assert.Equal(t, "companies_house", out.Source)
	})

	t.Run("should detect zip by magic bytes when Content-Type is wrong", func(t *testing.T) {
		t.Parallel()

		// Arrange — same zip payload but served with wrong Content-Type
		zipBody := buildZip(t, [][2]string{
			{"report.xhtml", "<html/>"},
		})
		svc := &mockCHService{}
		svc.On("GetDocument", mock.Anything, docURL).Return(
			&companyhouse.Document{
				Body:        io.NopCloser(bytes.NewReader(zipBody)),
				ContentType: "application/octet-stream",
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return((*cache.FilingEntry)(nil), nil)
		fc.On("Put", mock.Anything, "00445790", "abc123", "application/xhtml+xml", "report.xhtml", mock.Anything).
			Return("/cache/report.xhtml", int64(7), nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out fetchResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.Equal(t, "application/xhtml+xml", out.ContentType)
	})

	t.Run("should return tool error when zip is malformed", func(t *testing.T) {
		t.Parallel()

		// Arrange — PK magic bytes but not a valid zip
		svc := &mockCHService{}
		svc.On("GetDocument", mock.Anything, docURL).Return(
			&companyhouse.Document{
				Body:        io.NopCloser(strings.NewReader("PK\x03\x04not a real zip")),
				ContentType: "application/zip",
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return((*cache.FilingEntry)(nil), nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": docURL,
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})
}

func TestHandleGetLatest(t *testing.T) {
	t.Parallel()

	const docURL = "https://document-api.company-information.service.gov.uk/document/abc123"

	t.Run("should fetch and cache the latest filing for a category", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetFilingHistory", mock.Anything, "00445790", companyhouse.ListFilingsOptions{
			Category:     "accounts",
			ItemsPerPage: 1,
		}).Return(
			[]companyhouse.Filing{
				{
					TransactionID: "MzI1MDk3NjkxOGFkaXF6a2N4",
					Type:          "AA",
					Date:          time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC),
					DocumentURL:   docURL,
				},
			},
			nil,
		)
		svc.On("GetDocument", mock.Anything, docURL).Return(
			&companyhouse.Document{
				Body:        io.NopCloser(strings.NewReader("PDF content")),
				ContentType: "application/pdf",
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		fc := &mockFilingCache{}
		fc.On("Get", mock.Anything, "00445790", "abc123").Return((*cache.FilingEntry)(nil), nil)
		fc.On("Put", mock.Anything, "00445790", "abc123", "application/pdf", "", mock.Anything).Return("/cache/filing.pdf", int64(11), nil)
		defer fc.AssertExpectations(t)
		srv := New(svc, fc)

		// Act
		result, err := callTool(srv.handleGetLatest, map[string]any{
			"ch_number": "00445790",
			"category":  "accounts",
		})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
	})

	t.Run("should return a tool error when no filings exist for the category", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetFilingHistory", mock.Anything, "00445790", companyhouse.ListFilingsOptions{
			Category:     "accounts",
			ItemsPerPage: 1,
		}).Return([]companyhouse.Filing{}, nil)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleGetLatest, map[string]any{
			"ch_number": "00445790",
			"category":  "accounts",
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when the latest filing has no downloadable document", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetFilingHistory", mock.Anything, "00445790", companyhouse.ListFilingsOptions{
			Category:     "accounts",
			ItemsPerPage: 1,
		}).Return(
			[]companyhouse.Filing{
				{
					TransactionID: "MzI1MDk3NjkxOGFkaXF6a2N4",
					Type:          "AA",
					Date:          time.Date(2024, 6, 21, 0, 0, 0, 0, time.UTC),
					DocumentURL:   "", // no downloadable document
				},
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleGetLatest, map[string]any{
			"ch_number": "00445790",
			"category":  "accounts",
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when ch_number is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleGetLatest, map[string]any{"category": "accounts"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error when category is missing", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleGetLatest, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error for an invalid ch_number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleGetLatest, map[string]any{
			"ch_number": "../../etc",
			"category":  "accounts",
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})
}

func TestHandleClearCache(t *testing.T) {
	t.Parallel()

	t.Run("should delete all cached files and report freed space", func(t *testing.T) {
		t.Parallel()

		// Arrange
		fc := &mockFilingCache{}
		fc.On("Clear", mock.Anything, "").Return(cache.ClearResult{DeletedFiles: 2, FreedBytes: 500, DBRecords: 2}, nil)
		defer fc.AssertExpectations(t)
		srv := New(&mockCHService{}, fc)

		// Act
		result, err := callTool(srv.handleClearCache, map[string]any{})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out clearCacheResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.EqualValues(t, 2, out.DeletedFiles)
		assert.EqualValues(t, 500, out.FreedBytes)
		assert.EqualValues(t, 2, out.DBRecordsRemoved)
	})

	t.Run("should delete only the specified company's cache", func(t *testing.T) {
		t.Parallel()

		// Arrange
		fc := &mockFilingCache{}
		fc.On("Clear", mock.Anything, "00445790").Return(cache.ClearResult{DeletedFiles: 1, FreedBytes: 250, DBRecords: 1}, nil)
		defer fc.AssertExpectations(t)
		srv := New(&mockCHService{}, fc)

		// Act
		result, err := callTool(srv.handleClearCache, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out clearCacheResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.EqualValues(t, 1, out.DeletedFiles)
	})

	t.Run("should succeed when the cache is empty", func(t *testing.T) {
		t.Parallel()

		// Arrange
		fc := &mockFilingCache{}
		fc.On("Clear", mock.Anything, "").Return(cache.ClearResult{}, nil)
		defer fc.AssertExpectations(t)
		srv := New(&mockCHService{}, fc)

		// Act
		result, err := callTool(srv.handleClearCache, map[string]any{})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out clearCacheResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.EqualValues(t, 0, out.DeletedFiles)
		assert.EqualValues(t, int64(0), out.FreedBytes)
	})

	t.Run("should propagate cache errors", func(t *testing.T) {
		t.Parallel()

		// Arrange
		fc := &mockFilingCache{}
		fc.On("Clear", mock.Anything, "").Return(cache.ClearResult{}, errors.New("disk error"))
		defer fc.AssertExpectations(t)
		srv := New(&mockCHService{}, fc)

		// Act
		_, err := callTool(srv.handleClearCache, map[string]any{})

		// Assert
		assert.Error(t, err)
	})

	t.Run("should return a tool error for an invalid ch_number", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleClearCache, map[string]any{"ch_number": "../../etc"})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})
}

func TestIsAllowedDocumentURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url  string
		want bool
	}{
		// Valid CH document API URLs
		{"https://document-api.company-information.service.gov.uk/document/abc123", true},
		{"https://document-api.company-information.service.gov.uk/document/abc123/content", true},
		// Relative paths — trusted CH API paths, not externally addressable
		{"/document/abc123", true},
		{"/document/abc123/content", true},
		// Blocked: wrong domain
		{"https://evil.com/document/abc123", false},
		{"https://evil.company-information.service.gov.uk/document/abc123", false},
		// Blocked: HTTP (not HTTPS)
		{"http://document-api.company-information.service.gov.uk/document/abc123", false},
		// Blocked: protocol-relative (no scheme but has host)
		{"//document-api.company-information.service.gov.uk/document/abc123", false},
		// Blocked: SSRF to metadata endpoint
		{"http://169.254.169.254/document/foo", false},
	}

	for _, test := range cases {
		t.Run(test.url, func(t *testing.T) {
			t.Parallel()

			// Act
			got := isAllowedDocumentURL(test.url)

			// Assert
			assert.Equal(t, test.want, got, "isAllowedDocumentURL(%q)", test.url)
		})
	}
}

func TestFetchFilingSSRFValidation(t *testing.T) {
	t.Parallel()

	t.Run("should return a tool error for a non-CH document URL", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": "http://169.254.169.254/document/sensitive",
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})

	t.Run("should return a tool error for an HTTPS URL on a non-CH domain", func(t *testing.T) {
		t.Parallel()

		// Arrange
		srv := newTestServer(&mockCHService{})

		// Act
		result, err := callTool(srv.handleFetchFiling, map[string]any{
			"ch_number":    "00445790",
			"document_url": "https://evil.com/document/abc123",
		})

		// Assert
		assert.NoError(t, err)
		assert.True(t, isToolError(result))
	})
}

func TestHandleListFilingsFiltering(t *testing.T) {
	t.Parallel()

	t.Run("should omit filings that have no document URL", func(t *testing.T) {
		t.Parallel()

		// Arrange
		svc := &mockCHService{}
		svc.On("GetFilingHistory", mock.Anything, "00445790", companyhouse.ListFilingsOptions{
			ItemsPerPage: defaultFilingsLimit,
		}).Return(
			[]companyhouse.Filing{
				{
					TransactionID: "with-doc",
					Type:          "AA",
					DocumentURL:   "https://document-api.company-information.service.gov.uk/document/abc123",
				},
				{
					TransactionID: "no-doc",
					Type:          "CS01",
					DocumentURL:   "", // no downloadable document
				},
			},
			nil,
		)
		defer svc.AssertExpectations(t)
		srv := newTestServer(svc)

		// Act
		result, err := callTool(srv.handleListFilings, map[string]any{"ch_number": "00445790"})

		// Assert
		assert.NoError(t, err)
		assert.False(t, isToolError(result))
		var out []filingResult
		assert.NoError(t, json.Unmarshal([]byte(resultText(result)), &out))
		assert.Len(t, out, 1)
		assert.Equal(t, "with-doc", out[0].TransactionID)
	})
}

func TestDocIDFromURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url    string
		wantID string
		wantOK bool
	}{
		// Content URLs (with /content suffix) — returned by fetch_filing tool input
		{"https://api.company-information.service.gov.uk/document/abc123/content", "abc123", true},
		{"/document/xyz789/content", "xyz789", true},
		// Metadata URLs (without /content) — returned by list_filings / GetFilingHistory
		{"https://document-api.company-information.service.gov.uk/document/MzAxNjM4NjM3NWFkaXF6a2N4", "MzAxNjM4NjM3NWFkaXF6a2N4", true},
		{"/document/abc456", "abc456", true},
		// Invalid inputs
		{"invalid-url-no-document-segment", "", false},
		{"https://example.com/other/path", "", false},
		// Edge case: /document/content — the segment after "document" is the literal
		// word "content", which is an unlikely but valid document ID at the CH API.
		{"/document/content", "content", true},
	}

	for _, test := range cases {
		t.Run(test.url, func(t *testing.T) {
			t.Parallel()

			// Act
			gotID, gotOK := docIDFromURL(test.url)

			// Assert
			assert.Equal(t, test.wantOK, gotOK, "docIDFromURL(%q) ok", test.url)
			assert.Equal(t, test.wantID, gotID, "docIDFromURL(%q) id", test.url)
		})
	}
}

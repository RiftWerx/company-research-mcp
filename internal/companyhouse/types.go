package companyhouse

import (
	"io"
	"time"
)

// RegisteredAddress is a company's registered office address.
type RegisteredAddress struct {
	AddressLine1 string
	AddressLine2 string
	Locality     string
	PostalCode   string
	Country      string
}

// SearchResult is a single company match returned by SearchCompanies.
type SearchResult struct {
	CompanyNumber  string
	Title          string
	CompanyType    string
	CompanyStatus  string
	DateOfCreation string
	Address        RegisteredAddress
}

// CompanyProfile is the full profile returned by GetCompanyProfile.
type CompanyProfile struct {
	CompanyNumber    string
	CompanyName      string
	CompanyType      string
	CompanyStatus    string
	DateOfCreation   string
	SICCodes         []string
	RegisteredOffice RegisteredAddress
}

// Filing is a single entry from the Companies House filing history.
type Filing struct {
	TransactionID string
	Type          string // e.g. "AA" (annual accounts), "CS01" (confirmation statement)
	Description   string
	Date          time.Time
	DocumentURL   string // full URL to the document metadata endpoint; pass to GetDocument
}

// ListFilingsOptions filters the filing history returned by GetFilingHistory.
// Zero values are omitted from the request.
type ListFilingsOptions struct {
	Category     string // e.g. "accounts", "confirmation-statement"
	StartIndex   int
	ItemsPerPage int
}

// Document holds the response from a document download.
// The caller is responsible for closing Body.
type Document struct {
	Body        io.ReadCloser
	ContentType string // e.g. "application/pdf" or "application/xhtml+xml"
}

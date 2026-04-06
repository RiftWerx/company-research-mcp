package companyhouse

import (
	"net/url"
	"regexp"
	"strings"
)

// documentAPIHost is the only hostname from which filing documents may be fetched.
// Kept unexported — callers use ValidateDocumentURL; they don't need the constant directly.
const documentAPIHost = "document-api.company-information.service.gov.uk"

// chNumberRe matches valid Companies House numbers. English companies use 8 digits
// (e.g. "00445790"); Scottish, Northern Irish, and LLP numbers use a 1–2 letter
// prefix followed by digits (e.g. "SC123456", "NI012345", "OC300001"). The regex
// accepts 6–10 alphanumeric characters to cover all known formats. Case-insensitive.
// This allow-list guards against path traversal: ch_number is used as a directory
// component in the cache layer and must not contain path separators or traversal sequences.
var chNumberRe = regexp.MustCompile(`(?i)^[A-Z0-9]{6,10}$`)

// ValidateCHNumber reports whether s is a plausible Companies House number.
func ValidateCHNumber(s string) bool {
	return chNumberRe.MatchString(s)
}

// docIDRe matches the opaque document-ID segment extracted from Companies House document URLs.
// CH document IDs are base64url-encoded opaque strings (e.g. "MzAyNDA3MzA4NmFkaXF6a2N4").
// This allow-list guards against path traversal: doc_id is used as a directory component
// in the cache layer and must not contain path separators or traversal sequences.
var docIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)

// ValidateDocID reports whether s is a plausible CH document ID.
func ValidateDocID(s string) bool {
	return docIDRe.MatchString(s)
}

// ValidateDocumentURL returns true if rawURL is a valid CH document API URL.
// The URL must be absolute, use HTTPS, and resolve to documentAPIHost.
func ValidateDocumentURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "https" && u.Hostname() == documentAPIHost
}

// ParseDocumentID extracts the document ID from a CH document URL.
// Handles both the metadata URL form (.../document/{id}) and the content URL form
// (.../document/{id}/content). Returns the ID and true on success, or "", false if
// the URL cannot be parsed or does not contain a "document" path segment followed by an ID.
func ParseDocumentID(documentURL string) (string, bool) {
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

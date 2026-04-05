// Package mime provides MIME type utilities for filing documents.
package mime

import "strings"

// Ext returns a file extension for the given MIME type.
// Parameters such as "; charset=utf-8" are stripped before matching.
func Ext(contentType string) string {
	ct := strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch ct {
	case "application/pdf":
		return ".pdf"
	case "application/xhtml+xml":
		return ".xhtml"
	case "text/html":
		return ".html"
	default:
		return ".bin"
	}
}

// TypeFromExt returns the MIME type for a lowercase file extension.
func TypeFromExt(ext string) string {
	switch ext {
	case ".xhtml":
		return "application/xhtml+xml"
	case ".html", ".htm":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

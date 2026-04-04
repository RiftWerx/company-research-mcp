package cache

import "strings"

// fileExt returns a file extension for the given MIME type.
func fileExt(contentType string) string {
	// Strip parameters such as "; charset=utf-8".
	ct := strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch ct {
	case "application/pdf":
		return ".pdf"
	case "application/xhtml+xml":
		return ".xhtml"
	default:
		return ".bin"
	}
}

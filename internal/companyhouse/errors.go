package companyhouse

import "errors"

// Sentinel errors returned by Service methods. MCP tool handlers should use
// errors.Is to branch on these rather than inspecting error strings.
var (
	// ErrNotFound is returned when the CH API responds with 404.
	ErrNotFound = errors.New("not found")

	// ErrUnauthorized is returned when the CH API responds with 401.
	// This typically means the API key is missing or invalid.
	ErrUnauthorized = errors.New("unauthorized — check CH_API_KEY")

	// ErrRateLimited is returned when the CH API responds with 429.
	ErrRateLimited = errors.New("rate limited")
)

package companyhouse

import "time"

// DefaultRate is the target request rate for the Companies House API.
// CH allows 600 requests per 5 minutes (2 req/sec absolute maximum).
// We target 80% of the cap (~1.5 req/sec = ~450 req/5min) to stay clear of the limit.
const DefaultRate = 1.5 // requests per second

// DefaultBurst is the token bucket burst size for CH API requests.
// A burst of 1 means no queued tokens can accumulate — requests are paced
// strictly at DefaultRate with no ability to burst above the steady rate.
const DefaultBurst = 1

// DefaultTimeout is the per-request HTTP timeout for CH API calls.
const DefaultTimeout = 10 * time.Second

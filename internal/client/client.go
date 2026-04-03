package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// Config holds the configuration for a rate-limited HTTP client.
// Each external source should have its own Config and Client instance.
type Config struct {
	// Rate is the number of requests permitted per second.
	Rate float64
	// Burst is the maximum number of requests permitted in a burst.
	// Use 1 for conservative behaviour (no bursting).
	Burst int
	// Timeout is the maximum duration for a single request, independent of context.
	Timeout time.Duration
	// UserAgent is sent as the User-Agent header on every request.
	UserAgent string
}

// Client is a rate-limited HTTP client. Each external source should have its
// own Client instance with its own rate limiter — sources do not share limits.
//
// The zero value is not usable; construct with New.
type Client struct {
	http    *http.Client
	limiter *rate.Limiter
	ua      string
}

// New constructs a Client from the given Config.
func New(cfg Config) *Client {
	return &Client{
		http: &http.Client{
			Timeout: cfg.Timeout,
		},
		limiter: rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst),
		ua:      cfg.UserAgent,
	}
}

// Do executes req, waiting for a rate limit token before sending.
//
// The rate limiter respects ctx: if the context is cancelled or its deadline
// exceeded before a token becomes available, Do returns immediately with the
// context error. This ensures MCP tool handlers can propagate agent cancellation
// cleanly without hanging.
//
// On success the caller is responsible for closing resp.Body.
// On error resp is nil; net/http closes the body before returning a transport error.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	if c.ua != "" {
		req = req.Clone(ctx)
		req.Header.Set("User-Agent", c.ua)
	} else {
		req = req.WithContext(ctx)
	}

	// User-supplied document URLs are validated against the CH document API domain
	// in MCP tool handlers before reaching this layer.
	resp, err := c.http.Do(req) //nolint:gosec // G107: document_url inputs are domain-validated in the MCP handler layer
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Get is a convenience wrapper around Do for simple GET requests.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return c.Do(ctx, req)
}

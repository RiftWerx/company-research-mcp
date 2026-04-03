package companyhouse

// NewForTest constructs a Service with an injectable base URL for use in tests.
// Pass an empty baseURL to use the production default.
func NewForTest(h HTTPDoer, apiKey, baseURL string) *Service {
	if baseURL == "" {
		return New(h, apiKey)
	}
	return newWithBaseURL(h, apiKey, baseURL)
}

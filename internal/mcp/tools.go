package mcp

import "github.com/mark3labs/mcp-go/mcp"

// searchCompanyTool returns the tool definition for search_company.
func searchCompanyTool() mcp.Tool {
	return mcp.NewTool("search_company",
		mcp.WithDescription("Search for a UK company by name. Returns a list of matching companies with their Companies House numbers."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Company name or partial name to search for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default 10)"),
		),
	)
}

// getCompanyProfileTool returns the tool definition for get_company_profile.
func getCompanyProfileTool() mcp.Tool {
	return mcp.NewTool("get_company_profile",
		mcp.WithDescription("Get the profile of a UK company by its Companies House number. Returns name, status, type, SIC codes, registered address, and date of incorporation."),
		mcp.WithString("ch_number",
			mcp.Required(),
			mcp.Description("Companies House number (e.g. '00445790'). Zero-padded to 8 digits."),
		),
	)
}

// listFilingsTool returns the tool definition for list_filings.
func listFilingsTool() mcp.Tool {
	return mcp.NewTool("list_filings",
		mcp.WithDescription("List the filing history for a UK company. Returns filing metadata including the document URL needed for fetch_filing."),
		mcp.WithString("ch_number",
			mcp.Required(),
			mcp.Description("Companies House number"),
		),
		mcp.WithString("category",
			mcp.Description("Filter by filing category. Common values: 'accounts', 'confirmation-statement'. Omit to return all categories."),
		),
		mcp.WithNumber("start",
			mcp.Description("Pagination offset — index of the first result to return (default 0)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of filings to return (default 20)"),
		),
	)
}

// fetchFilingTool returns the tool definition for fetch_filing.
func fetchFilingTool() mcp.Tool {
	return mcp.NewTool("fetch_filing",
		mcp.WithDescription("Download a specific filing document to the local cache. Use the document_url from list_filings output. Returns the local file path and metadata."),
		mcp.WithString("ch_number",
			mcp.Required(),
			mcp.Description("Companies House number of the company the filing belongs to"),
		),
		mcp.WithString("document_url",
			mcp.Required(),
			mcp.Description("Document URL from list_filings output"),
		),
	)
}

// getLatestTool returns the tool definition for get_latest.
func getLatestTool() mcp.Tool {
	return mcp.NewTool("get_latest",
		mcp.WithDescription("Fetch the most recent filing of a given category for a UK company. Combines list_filings and fetch_filing in a single call. Returns the local file path and metadata."),
		mcp.WithString("ch_number",
			mcp.Required(),
			mcp.Description("Companies House number"),
		),
		mcp.WithString("category",
			mcp.Required(),
			mcp.Description("Filing category to fetch. Common values: 'accounts', 'confirmation-statement'."),
		),
	)
}

// extractXBRLFactsTool returns the tool definition for extract_xbrl_facts.
func extractXBRLFactsTool() mcp.Tool {
	return mcp.NewTool("extract_xbrl_facts",
		mcp.WithDescription("Parse a cached iXBRL .xhtml file and return structured financial facts as JSON. "+
			"Use the local_path returned by fetch_filing or get_latest when content_type is application/xhtml+xml. "+
			"Returns {facts, count, truncated, render_type, warnings?}. "+
			"render_type is \"native_ixbrl\" or \"pdf_rendered\"; when \"pdf_rendered\", a warnings array explains that narrative text is not reliably accessible. "+
			"When truncated is true the document contained more facts than the cap — use name_prefix to narrow the query and retrieve the facts you need."),
		mcp.WithString("local_path",
			mcp.Required(),
			mcp.Description("Absolute path to a cached iXBRL .xhtml file, as returned by fetch_filing or get_latest"),
		),
		mcp.WithString("name_prefix",
			mcp.Description("Optional concept name prefix filter. Only facts whose name starts with this value are returned (e.g. 'Revenue' returns all Revenue* facts). Case-sensitive. Applies to both numeric and text facts."),
		),
		mcp.WithBoolean("include_text_facts",
			mcp.Description("Include ix:nonNumeric text facts such as company name and director names. Default false — only numeric ix:nonFraction facts are returned. name_prefix filtering applies to text facts when this is true."),
		),
	)
}

// clearCacheTool returns the tool definition for clear_cache.
func clearCacheTool() mcp.Tool {
	return mcp.NewTool("clear_cache",
		mcp.WithDescription("Delete downloaded filing documents from the local cache. Pass ch_number to clear one company; omit it to clear everything. Returns counts of deleted files and bytes freed."),
		mcp.WithString("ch_number",
			mcp.Description("Companies House number to scope the clear. Omit to clear all cached filings."),
		),
	)
}

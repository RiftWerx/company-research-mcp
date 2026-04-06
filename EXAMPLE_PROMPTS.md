# Example Prompts

Example prompts for each tool provided by this MCP server.

## `search_company`

> Search for UK companies by name.

- "Search for companies named Acme"
- "Find all companies matching 'British Steel'"
- "Search for 'OpenAI' and show the top 5 results"

## `get_company_profile`

> Get a company's profile by its Companies House number.

- "Get the profile for company 00445790"
- "What is the registered address of company 01234567?"
- "Is company 09876543 still active?"

## `list_filings`

> List the filing history for a company.

- "List all filings for company 00445790"
- "Show the last 5 accounts filings for company 01234567"
- "List confirmation statements for company 09876543"
- "Show all filings for company 00445790 starting from the 20th"

## `fetch_filing`

> Download a specific filing document.

- "Download the filing at /document/abc123 for company 00445790"
- "Fetch the document at the URL returned by list_filings for company 01234567"

## `get_latest`

> Fetch the most recent filing of a given category in one call.

- "Get the latest accounts for company 00445790"
- "Fetch the most recent confirmation statement for company 01234567"
- "Download the latest accounts filing for company 09876543 and summarise it"

## `extract_xbrl_facts`

> Parse a cached iXBRL `.xhtml` file and return structured financial facts as JSON.
> Use the `local_path` returned by `fetch_filing` or `get_latest` when `content_type` is `application/xhtml+xml`.

- "Extract all financial facts from the iXBRL file at /path/to/filing.xhtml"
- "Get only the revenue facts from the cached iXBRL report at /path/to/filing.xhtml"
- "Extract facts with name prefix 'Revenue' from the accounts I just downloaded"
- "Parse the iXBRL file and include text facts such as director names"
- "The result was truncated — re-run with name_prefix='Assets' to get just the asset facts"

## `clear_cache`

> Delete cached filing documents.

- "Clear the cache for company 00445790"
- "Clear all cached filings"
- "How much disk space will be freed if I clear the cache for company 01234567?"

---

## Multi-tool prompts

Prompts that chain several tools together.

- "Find a company called 'Monzo' and tell me whether it's still active" — `search_company` → `get_company_profile`
- "Look up 'Rolls-Royce' on Companies House and list their last 10 accounts filings" — `search_company` → `get_company_profile` → `list_filings`
- "Search for 'DeepMind', then download and summarise their most recent annual accounts" — `search_company` → `get_latest`
- "Find 'Dyson' and compare the SIC codes and registered addresses of any matching companies" — `search_company` → `get_company_profile` (multiple)
- "Get the latest accounts and latest confirmation statement for company 00445790, then clear its cache" — `get_latest` (×2) → `clear_cache`
- "Which directors are listed in the most recent confirmation statement for 'Arm Holdings'?" — `search_company` → `get_latest`
- "Download the latest accounts for 'Tesco' and extract all revenue and profit facts" — `search_company` → `get_latest` → `extract_xbrl_facts`

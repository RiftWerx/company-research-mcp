# company-research-mcp

An MCP server that fetches official company filings — annual reports, AGM documents, regulatory
announcements — from public sources and makes them available to an AI client.

## Current Coverage

- UK companies via Companies House

## Installation

**Download a pre-built binary** (no Go required):

Download the latest release for your platform from
[GitHub Releases](https://github.com/riftwerx/company-research-mcp/releases/latest),
extract the archive, and place the binary on your `$PATH`.

**Install via `go install`:**

```bash
go install github.com/riftwerx/company-research-mcp/cmd/company-research-mcp@latest
```

**Or build from source** (installs to `$GOPATH/bin`):

```bash
git clone https://github.com/riftwerx/company-research-mcp
cd company-research-mcp
make local-release
```

> `make` targets require a Unix-like environment (Linux, macOS, or WSL on Windows).
> Windows users without WSL can run `go install ./cmd/company-research-mcp` directly —
> the binary is fully Windows-compatible; only the build tooling requires Unix.

The binary is named `company-research-mcp`. Ensure `$(go env GOPATH)/bin` is on your `$PATH`.

Verify the installation:

```bash
company-research-mcp --version
```

## Prerequisites

Register for a free Companies House API key at
https://developer.company-information.service.gov.uk

## Client Configuration

### Claude Code

```bash
claude mcp add --transport stdio company-research \
  --scope user \
  --env CH_API_KEY=your-key-here \
  -- company-research-mcp
```

### Claude Desktop

Add to `claude_desktop_config.json`:

- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "company-research": {
      "command": "company-research-mcp",
      "env": {
        "CH_API_KEY": "your-key-here"
      }
    }
  }
}
```

### Cursor

Add to `~/.cursor/mcp.json` (global) or `.cursor/mcp.json` (project):

```json
{
  "mcpServers": {
    "company-research": {
      "command": "company-research-mcp",
      "env": {
        "CH_API_KEY": "your-key-here"
      }
    }
  }
}
```

### Windsurf

Add to `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "company-research": {
      "command": "company-research-mcp",
      "env": {
        "CH_API_KEY": "your-key-here"
      }
    }
  }
}
```

### Continue.dev

Add to `~/.continue/config.json` under `mcpServers`:

```json
{
  "mcpServers": [
    {
      "name": "company-research",
      "command": "company-research-mcp",
      "env": {
        "CH_API_KEY": "your-key-here"
      }
    }
  ]
}
```

## Available Tools

| Tool | Description |
|---|---|
| `search_company` | Search for UK companies by name |
| `get_company_profile` | Get a company profile by Companies House number |
| `list_filings` | List filing history for a company (filings without a downloadable document are excluded) |
| `fetch_filing` | Download a specific filing document (`document_url` must be a `document-api.company-information.service.gov.uk` URL) |
| `get_latest` | Fetch the most recent filing of a given category |
| `clear_cache` | Delete cached filing documents |

### Document formats

Filings are returned as **PDF** or **iXBRL** (`.xhtml`) depending on what Companies House provides.
When a filing is served as a zip archive the primary document is extracted automatically — the
`local_path` in the response points to the extracted file, not the original zip.

### Local cache

Downloaded filings are stored on disk and reused on subsequent requests:

| Platform | Path |
|---|---|
| Linux | `~/.cache/company-research.mcp/` |
| macOS | `~/Library/Caches/company-research.mcp/` |
| Windows | `%LOCALAPPDATA%\company-research.mcp\` |

Use the `clear_cache` tool to free disk space.

## Examples

(See [EXAMPLE_PROMPTS.md](EXAMPLE_PROMPTS.md) for more usage examples)

### Company status check

> Find Monzo Bank and tell me whether it's still active

*Tools: `search_company` → `get_company_profile`*

**Agent:** Monzo Bank Limited (CH: 09446231) is **active**. It's a private limited company
registered in England & Wales. Registered office: Broadwalk House, 5 Appold Street, London,
EC2A 2AG. SIC code: 64191 (Banks).

---

### Fetch and summarise the latest accounts

> Look up Rolls-Royce Holdings, then download and summarise their most recent annual accounts

*Tools: `search_company` → `get_company_profile` → `get_latest`*

**Agent:** Found Rolls-Royce Holdings plc (CH: 07524813), active. Fetching the latest
`accounts` filing… downloaded 4.2 MB PDF to the local cache.

*[reads document]*

The accounts cover the year ended 31 December 2023. Revenue was £16.5 billion (up 17%
year-on-year), driven by growth in Civil Aerospace. Underlying operating profit was £1.6
billion, compared to £652 million in 2022. Net debt reduced from £3.3 billion to £0.2
billion. The auditors issued an unqualified opinion.

---

### Compare multiple companies by name

> Search for "British Steel" and compare the registered addresses of the top results

*Tools: `search_company` → `get_company_profile` ×N*

**Agent:**

| Company | CH Number | Status | Registered Address |
|---|---|---|---|
| British Steel Limited | 09523808 | Active | Brigg Road, Scunthorpe, DN16 1BP |
| British Steel Pension Scheme Trustee Ltd | 06437005 | Active | 1 Kingsway, London, WC2B 6AN |
| British Steel (Overseas Holdings) Ltd | 04669999 | Dissolved | — |

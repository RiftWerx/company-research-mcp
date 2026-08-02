// Package skills embeds the Claude skill definition files for installation.
package skills

import "embed"

const (
	NameMCP = "company-research-mcp"
	NameCLI = "company-research-cli"
)

//go:embed company-research-mcp company-research-cli
var Files embed.FS

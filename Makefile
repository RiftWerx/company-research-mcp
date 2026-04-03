.PHONY: test lint vuln local-release release-dry-run

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
LDFLAGS := -X github.com/riftwerx/company-research-mcp/internal/mcp.Version=$(VERSION)

test:
	go test ./...

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

local-release:
	go install -ldflags "$(LDFLAGS)" ./cmd/company-research-mcp

release-dry-run:
	goreleaser release --snapshot --clean

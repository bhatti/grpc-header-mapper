#!/bin/bash
set -e

echo "Installing development tools..."

# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Install goreleaser
go install github.com/goreleaser/goreleaser@latest

# Install other tools via go mod
go mod download
go install -tags tools ./tools

echo "All tools installed successfully!"

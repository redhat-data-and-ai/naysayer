#!/bin/bash

# Naysayer Code Quality Setup Script
# Installs minimal required tools for code quality

set -euo pipefail

echo "🛠️  Setting up code quality tools for Naysayer..."

# Install golangci-lint if not present
if ! command -v golangci-lint > /dev/null; then
    echo "📦 Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v1.54.2
    echo "✅ golangci-lint installed"
else
    echo "✅ golangci-lint already installed"
fi

echo ""
echo "🎯 Code quality tools ready!"
echo ""
echo "📋 Available commands:"
echo "  make lint       - Run golangci-lint"
echo "  make lint-fix   - Run golangci-lint with automatic fixes"
echo "  make fmt        - Format code"
echo "  make vet        - Run go vet"
echo "  make test       - Run tests"
echo ""
echo "🚀 Run 'make lint-fix' to automatically fix issues, or 'make lint' to just check!"

#!/usr/bin/env bash
# Manual test script for .github/workflows/go.yml
# This replicates the build job steps locally

set -e

echo "=== Testing GitHub Actions Workflow Locally ==="
echo ""

# Check Go version
echo "Checking Go version..."
go version
echo ""

# Build without cgo
echo "Building with cgo disabled..."
CGO_ENABLED=0 go build -v ./...
echo ""

# Test without cgo
echo "Testing with cgo disabled..."
CGO_ENABLED=0 go test -v $(go list ./... | xargs -n1 sh -c 'ls $0/*_test.go 1>/dev/null 2>&1 && echo $0' | grep .)
echo ""

echo "=== Build job completed successfully ==="


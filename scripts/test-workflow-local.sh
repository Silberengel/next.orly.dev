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

# Setup library path (runtime optional)
if [ -f "pkg/crypto/p8k/libsecp256k1.so" ]; then
    export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)/pkg/crypto/p8k"
    echo "Library path set for libsecp256k1.so (runtime optional)"
fi
echo ""

# Build with pure Go + purego (no CGO needed)
echo "Building with pure Go + purego..."
CGO_ENABLED=0 go build -v ./...
echo ""

# Test with pure Go + purego
echo "Testing with pure Go + purego..."
CGO_ENABLED=0 go test -v $(go list ./... | xargs -n1 sh -c 'ls $0/*_test.go 1>/dev/null 2>&1 && echo $0' | grep .)
echo ""

echo "=== Build job completed successfully ==="


#!/bin/bash

# Policy System Test Runner
# This script runs all policy-related tests and benchmarks

set -e

echo "🧪 Running Policy System Tests"
echo "================================"

# Change to the project directory
cd "$(dirname "$0")"

# Run policy package tests
echo ""
echo "📦 Running Policy Package Tests..."
go test -v ./pkg/policy/... -run "Test.*" -timeout 30s

# Run policy integration tests
echo ""
echo "🔗 Running Policy Integration Tests..."
go test -v ./app/... -run "TestPolicy.*" -timeout 30s

# Run policy benchmarks
echo ""
echo "⚡ Running Policy Benchmarks..."
go test -v ./pkg/policy/... -run "Benchmark.*" -bench=. -benchmem -timeout 60s

# Run edge case tests
echo ""
echo "🔍 Running Edge Case Tests..."
go test -v ./pkg/policy/... -run "TestEdge.*" -timeout 30s

# Run race condition tests
echo ""
echo "🏃 Running Race Condition Tests..."
go test -v ./pkg/policy/... -race -timeout 30s

# Run coverage analysis
echo ""
echo "📊 Running Coverage Analysis..."
go test -v ./pkg/policy/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
echo "Coverage report generated: coverage.html"

# Check for any linting issues
echo ""
echo "🔍 Running Linter Checks..."
golangci-lint run ./pkg/policy/... || echo "Linter not available, skipping..."

echo ""
echo "✅ All Policy Tests Completed!"
echo "================================"

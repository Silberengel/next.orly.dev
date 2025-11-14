#!/bin/bash

# Test script for WASM shell examples

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================="
echo "WASM Shell Test Suite"
echo "========================================="
echo ""

# Build first
echo "[1/4] Building WASM modules..."
./build.sh
echo ""

# Test hello.wasm
echo "[2/4] Testing hello.wasm (stdout only)..."
echo "---"
./run.sh hello.wasm
echo ""

# Test echo.wasm with piped input
echo "[3/4] Testing echo.wasm (stdin/stdout with pipe)..."
echo "---"
echo "This is a test message" | ./run.sh echo.wasm
echo ""

# Test echo.wasm with heredoc
echo "[4/4] Testing echo.wasm (stdin/stdout with heredoc)..."
echo "---"
./run.sh echo.wasm <<< "Testing heredoc input"
echo ""

echo "========================================="
echo "All tests passed!"
echo "========================================="
echo ""
echo "Try these commands:"
echo "  ./run.sh hello.wasm"
echo "  echo 'your text' | ./run.sh echo.wasm"
echo "  ./run.sh echo.wasm  # interactive mode"

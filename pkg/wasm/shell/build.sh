#!/bin/bash

# Build script for WASM shell examples
# Compiles WAT (WebAssembly Text) to WASM binary

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building WASM modules from WAT files..."

# Check if wat2wasm is available
if ! command -v wat2wasm &> /dev/null; then
    echo "Error: wat2wasm not found. Install wabt:"
    echo "  sudo apt install wabt"
    exit 1
fi

# Build each .wat file to .wasm
for wat_file in *.wat; do
    if [ -f "$wat_file" ]; then
        wasm_file="${wat_file%.wat}.wasm"
        echo "  $wat_file -> $wasm_file"
        wat2wasm "$wat_file" -o "$wasm_file"
    fi
done

echo "Build complete!"
echo ""
echo "Run with:"
echo "  ./run.sh hello.wasm"
echo "  or"
echo "  wasmtime hello.wasm"

#!/bin/bash

# Run script for WASM shell examples
# Executes WASM files using wasmtime with WASI support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Find wasmtime executable
WASMTIME=""
if command -v wasmtime &> /dev/null; then
    WASMTIME="wasmtime"
elif [ -x "$HOME/.wasmtime/bin/wasmtime" ]; then
    WASMTIME="$HOME/.wasmtime/bin/wasmtime"
else
    echo "Error: wasmtime not found. Install it:"
    echo "  curl https://wasmtime.dev/install.sh -sSf | bash"
    echo ""
    echo "Or add to PATH:"
    echo "  export PATH=\"\$HOME/.wasmtime/bin:\$PATH\""
    exit 1
fi

# Get the WASM file from argument, default to hello.wasm
WASM_FILE="${1:-hello.wasm}"

# If relative path, make it relative to script dir
if [[ "$WASM_FILE" != /* ]]; then
    WASM_FILE="$SCRIPT_DIR/$WASM_FILE"
fi

if [ ! -f "$WASM_FILE" ]; then
    echo "Error: WASM file not found: $WASM_FILE"
    echo ""
    echo "Usage: $0 [wasm-file]"
    echo ""
    echo "Available WASM files:"
    cd "$SCRIPT_DIR"
    ls -1 *.wasm 2>/dev/null || echo "  (none - run ./build.sh first)"
    exit 1
fi

echo "Running: $WASM_FILE"
echo "---"

# Run the WASM file with wasmtime
# Additional flags you might want:
#   --dir=. : Mount current directory
#   --env VAR=value : Set environment variable
#   --invoke function : Call specific function instead of _start
"$WASMTIME" "$WASM_FILE" "$@"

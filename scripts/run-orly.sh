#!/bin/bash

# Universal run script for ORLY relay
# Automatically selects the correct binary for the current platform

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Source platform detection
source "$SCRIPT_DIR/platform-detect.sh"

# Get version
VERSION=$(cat "$REPO_ROOT/pkg/version/version")

# Detect platform
PLATFORM=$(detect_platform)
if [ $? -ne 0 ]; then
    echo "Error: Could not detect platform"
    exit 1
fi

echo "Detected platform: $PLATFORM"

# Get binary name
BINARY_NAME=$(get_binary_name "$VERSION")
BINARY_PATH="$REPO_ROOT/build/$BINARY_NAME"

# Check if binary exists
if [ ! -f "$BINARY_PATH" ]; then
    echo "Error: Binary not found: $BINARY_PATH"
    echo ""
    echo "Please run: ./scripts/build-all-platforms.sh"
    exit 1
fi

# Get library name and set library path
LIBRARY_NAME=$(get_library_name)
LIBRARY_PATH="$REPO_ROOT/build/$LIBRARY_NAME"

if [ -f "$LIBRARY_PATH" ]; then
    case "$PLATFORM" in
        linux-*)
            export LD_LIBRARY_PATH="$REPO_ROOT/build:${LD_LIBRARY_PATH:-}"
            ;;
        darwin-*)
            export DYLD_LIBRARY_PATH="$REPO_ROOT/build:${DYLD_LIBRARY_PATH:-}"
            ;;
        windows-*)
            export PATH="$REPO_ROOT/build:${PATH:-}"
            ;;
    esac
    echo "Library path set for: $LIBRARY_NAME"
else
    echo "Warning: Library not found: $LIBRARY_PATH"
    echo "Binary may not work if it requires libsecp256k1"
fi

echo "Running: $BINARY_NAME"
echo ""

# Execute the binary with all arguments passed to this script
exec "$BINARY_PATH" "$@"


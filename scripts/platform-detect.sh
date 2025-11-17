#!/bin/bash

# Platform detection and binary selection helper
# This script detects the current platform and returns the appropriate binary name

detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    # Normalize OS name
    case "$os" in
        linux*)
            os="linux"
            ;;
        darwin*)
            os="darwin"
            ;;
        mingw*|msys*|cygwin*)
            os="windows"
            ;;
        *)
            echo "unknown"
            return 1
            ;;
    esac
    
    # Normalize architecture
    case "$arch" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        armv7l)
            arch="arm"
            ;;
        *)
            echo "unknown"
            return 1
            ;;
    esac
    
    echo "${os}-${arch}"
    return 0
}

get_binary_name() {
    local version=$1
    local platform=$(detect_platform)
    
    if [ "$platform" = "unknown" ]; then
        echo "Error: Unsupported platform" >&2
        return 1
    fi
    
    local ext=""
    if [[ "$platform" == windows-* ]]; then
        ext=".exe"
    fi
    
    echo "orly-${version}-${platform}${ext}"
    return 0
}

get_library_name() {
    local platform=$(detect_platform)
    
    if [ "$platform" = "unknown" ]; then
        echo "Error: Unsupported platform" >&2
        return 1
    fi
    
    case "$platform" in
        linux-*)
            echo "libsecp256k1-${platform}.so"
            ;;
        darwin-*)
            echo "libsecp256k1-${platform}.dylib"
            ;;
        windows-*)
            echo "libsecp256k1-${platform}.dll"
            ;;
        *)
            return 1
            ;;
    esac
    
    return 0
}

# If called directly, run the function based on first argument
if [ "${BASH_SOURCE[0]}" -ef "$0" ]; then
    case "$1" in
        detect|platform)
            detect_platform
            ;;
        binary)
            get_binary_name "${2:-v0.25.0}"
            ;;
        library|lib)
            get_library_name
            ;;
        *)
            echo "Usage: $0 {detect|binary <version>|library}"
            echo ""
            echo "Commands:"
            echo "  detect     - Detect current platform (e.g., linux-amd64)"
            echo "  binary     - Get binary name for current platform"
            echo "  library    - Get library name for current platform"
            echo ""
            echo "Examples:"
            echo "  $0 detect"
            echo "  $0 binary v0.25.0"
            echo "  $0 library"
            exit 1
            ;;
    esac
fi


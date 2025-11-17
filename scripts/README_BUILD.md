# Multi-Platform Build Scripts

This directory contains scripts for building and running ORLY relay binaries across multiple platforms.

## Available Scripts

### `build-all-platforms.sh`
Comprehensive build script that creates binaries for all supported platforms:
- Linux (AMD64, ARM64)
- macOS (AMD64 Intel, ARM64 Apple Silicon)
- Windows (AMD64)
- Android (ARM64, AMD64) - requires Android NDK

**Usage:**
```bash
./scripts/build-all-platforms.sh
```

**Output:** All binaries are placed in `build/` directory with platform-specific names.

### `platform-detect.sh`
Helper script for platform detection and binary/library name resolution.

**Usage:**
```bash
# Detect current platform
./scripts/platform-detect.sh detect
# Output: linux-amd64

# Get binary name for version
./scripts/platform-detect.sh binary v0.25.0
# Output: orly-v0.25.0-linux-amd64

# Get library name
./scripts/platform-detect.sh library
# Output: libsecp256k1-linux-amd64.so
```

### `run-orly.sh`
Universal launcher that automatically selects and runs the correct binary for your platform.

**Usage:**
```bash
./scripts/run-orly.sh [arguments for orly]
```

Features:
- Auto-detects platform
- Sets correct library path
- Passes all arguments to the binary
- Shows helpful error if binary not found

## Build Prerequisites

### For Linux Host

```bash
# Install cross-compilation tools
sudo apt-get update
sudo apt-get install -y \
    gcc-mingw-w64-x86-64 \
    gcc-aarch64-linux-gnu \
    autoconf automake libtool
```

### For Android (optional)

Download and install Android NDK, then:
```bash
export ANDROID_NDK_HOME=/path/to/android-ndk
```

## Quick Start

1. **Build all platforms:**
   ```bash
   ./scripts/build-all-platforms.sh
   ```

2. **Run the binary:**
   ```bash
   ./scripts/run-orly.sh
   ```

3. **Or run specific platform binary:**
   ```bash
   # Linux
   export LD_LIBRARY_PATH=./build:$LD_LIBRARY_PATH
   ./build/orly-v0.25.0-linux-amd64

   # macOS
   ./build/orly-v0.25.0-darwin-arm64

   # Windows
   set PATH=.\build;%PATH%
   .\build\orly-v0.25.0-windows-amd64.exe
   ```

## Platform Support Matrix

| Platform       | Architecture | CGO | Library Required | Status       |
|----------------|--------------|-----|------------------|--------------|
| Linux          | AMD64        | ✓   | libsecp256k1.so  | Full Support |
| Linux          | ARM64        | ✓   | libsecp256k1.so  | Full Support |
| macOS          | AMD64        | ✗   | -                | Full Support |
| macOS          | ARM64        | ✗   | -                | Full Support |
| Windows        | AMD64        | ✓   | libsecp256k1.dll | Full Support |
| Android        | ARM64        | ✓   | libsecp256k1.so  | Experimental |
| Android        | AMD64        | ✓   | libsecp256k1.so  | Experimental |

**Note:** macOS builds use pure Go (no CGO) for easier distribution. Crypto operations are slower but no library dependency is required.

## Integration with Other Scripts

All deployment and test scripts have been updated to support platform detection:

- `scripts/deploy.sh` - Builds with CGO and copies libraries
- `scripts/test.sh` - Sets library paths for tests
- `scripts/benchmark.sh` - Uses platform-specific binaries
- `scripts/runtests.sh` - Sets library paths for benchmark tests

## CI/CD

GitHub Actions workflow (`.github/workflows/go.yml`) automatically:
- Builds for all major platforms on release
- Includes platform-specific libraries
- Creates SHA256 checksums
- Uploads all artifacts to GitHub releases

## File Naming Convention

Binaries:
- `orly-{version}-{os}-{arch}[.ext]`
- Example: `orly-v0.25.0-linux-amd64`

Libraries:
- `libsecp256k1-{os}-{arch}.{ext}`
- Example: `libsecp256k1-linux-amd64.so`

## Distribution

When distributing binaries, include:
1. The platform-specific binary
2. The corresponding library file (for CGO builds)
3. SHA256SUMS file for verification

Example release structure:
```
release/
├── orly-v0.25.0-linux-amd64
├── orly-v0.25.0-linux-arm64
├── orly-v0.25.0-darwin-amd64
├── orly-v0.25.0-darwin-arm64
├── orly-v0.25.0-windows-amd64.exe
├── libsecp256k1-linux-amd64.so
├── libsecp256k1-linux-arm64.so
├── libsecp256k1-windows-amd64.dll
└── SHA256SUMS-v0.25.0.txt
```

## Troubleshooting

**Binary not found:**
Run `./scripts/build-all-platforms.sh` to build binaries.

**Library not found at runtime:**
Set the library path:
```bash
# Linux
export LD_LIBRARY_PATH=./build:$LD_LIBRARY_PATH

# macOS
export DYLD_LIBRARY_PATH=./build:$DYLD_LIBRARY_PATH

# Windows
set PATH=.\build;%PATH%
```

**Cross-compilation fails:**
Install the required cross-compiler (see Build Prerequisites above).

## Performance Comparison

| Build Type    | Crypto Speed | Binary Size | Distribution |
|---------------|--------------|-------------|--------------|
| CGO (Linux)   | Fast (2-3x)  | Smaller     | Needs .so    |
| Pure Go (Mac) | Slower       | Larger      | Self-contained |

For detailed documentation, see [docs/BUILD_PLATFORMS.md](../docs/BUILD_PLATFORMS.md).


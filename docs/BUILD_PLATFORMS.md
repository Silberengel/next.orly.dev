# Multi-Platform Build Guide

This guide explains how to build ORLY binaries for multiple platforms.

## Quick Start

Build for all platforms:
```bash
./scripts/build-all-platforms.sh
```

Run the platform-specific binary:
```bash
./scripts/run-orly.sh
```

## Supported Platforms

### Tested Platforms
- **Linux AMD64** - Full CGO support with libsecp256k1
- **Linux ARM64** - Full CGO support with libsecp256k1
- **macOS AMD64** (Intel) - Pure Go (no CGO)
- **macOS ARM64** (Apple Silicon) - Pure Go (no CGO)
- **Windows AMD64** - Full CGO support with libsecp256k1

### Experimental Platforms
- **Android ARM64** - Requires Android NDK
- **Android AMD64** - Requires Android NDK

## Prerequisites

### Linux Build Host

Install cross-compilation tools:
```bash
# For Windows builds
sudo apt-get install gcc-mingw-w64-x86-64

# For ARM64 builds
sudo apt-get install gcc-aarch64-linux-gnu

# For Android builds (optional)
# Download Android NDK and set ANDROID_NDK_HOME
```

### macOS Cross-Compilation

CGO cross-compilation for macOS from Linux requires osxcross, which is complex to set up. The build script builds macOS binaries without CGO by default (pure Go).

## Build Output

All binaries are placed in `build/` directory:
```
build/
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

## Platform Detection

The `platform-detect.sh` script automatically detects the current platform:

```bash
# Detect platform
./scripts/platform-detect.sh detect
# Output: linux-amd64

# Get binary name for current platform
./scripts/platform-detect.sh binary v0.25.0
# Output: orly-v0.25.0-linux-amd64

# Get library name for current platform
./scripts/platform-detect.sh library
# Output: libsecp256k1-linux-amd64.so
```

## Running Platform-Specific Binaries

### Option 1: Use the run script (recommended)
```bash
./scripts/run-orly.sh [arguments]
```

This automatically:
- Detects your platform
- Sets the correct library path
- Runs the appropriate binary

### Option 2: Run directly

Linux:
```bash
export LD_LIBRARY_PATH=./build:$LD_LIBRARY_PATH
./build/orly-v0.25.0-linux-amd64
```

macOS:
```bash
export DYLD_LIBRARY_PATH=./build:$DYLD_LIBRARY_PATH
./build/orly-v0.25.0-darwin-arm64
```

Windows:
```powershell
set PATH=.\build;%PATH%
.\build\orly-v0.25.0-windows-amd64.exe
```

## Integration with Scripts

All test and deployment scripts now support platform detection:

### Test Scripts
Test scripts automatically set up the library path:
- `scripts/test.sh` - Sets LD_LIBRARY_PATH for tests
- `scripts/runtests.sh` - Sets LD_LIBRARY_PATH for benchmarks
- `scripts/test_policy.sh` - Sets LD_LIBRARY_PATH for policy tests

### Deployment Scripts
- `scripts/deploy.sh` - Builds with CGO and copies library
- `scripts/run-orly.sh` - Runs platform-specific binary

## Distribution

When distributing binaries, include both the executable and library:

For Linux:
```
orly-v0.25.0-linux-amd64
libsecp256k1-linux-amd64.so
```

For Windows:
```
orly-v0.25.0-windows-amd64.exe
libsecp256k1-windows-amd64.dll
```

Users can then run:
```bash
# Linux
chmod +x orly-v0.25.0-linux-amd64
export LD_LIBRARY_PATH=.:$LD_LIBRARY_PATH
./orly-v0.25.0-linux-amd64

# Windows
orly-v0.25.0-windows-amd64.exe
```

## CI/CD Integration

GitHub Actions workflow automatically:
- Builds for Linux (AMD64, ARM64), macOS (AMD64, ARM64), and Windows (AMD64)
- Includes platform-specific libraries
- Creates checksums
- Uploads all artifacts to releases

## Troubleshooting

### CGO_ENABLED but no C compiler
Install the appropriate cross-compiler:
```bash
# Windows
sudo apt-get install gcc-mingw-w64-x86-64

# ARM64
sudo apt-get install gcc-aarch64-linux-gnu
```

### Library not found at runtime
Set the appropriate library path:
```bash
# Linux
export LD_LIBRARY_PATH=./build:$LD_LIBRARY_PATH

# macOS
export DYLD_LIBRARY_PATH=./build:$DYLD_LIBRARY_PATH

# Windows
set PATH=.\build;%PATH%
```

### Android builds fail
Ensure Android NDK is installed and `ANDROID_NDK_HOME` is set:
```bash
export ANDROID_NDK_HOME=/path/to/android-ndk
./scripts/build-all-platforms.sh
```

## Performance Notes

- **CGO builds** (Linux, Windows): ~2-3x faster crypto operations
- **Pure Go builds** (macOS): Slower crypto but easier to distribute
- Consider using CGO builds for production servers
- Use pure Go builds for easier deployment on macOS


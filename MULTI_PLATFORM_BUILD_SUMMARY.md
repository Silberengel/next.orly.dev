# Multi-Platform Build System - Implementation Summary

## Created Scripts

### 1. `scripts/build-all-platforms.sh`
**Purpose:** Master build script for all platforms

**Features:**
- Builds for Linux (AMD64, ARM64)
- Builds for macOS (AMD64, ARM64) - pure Go
- Builds for Windows (AMD64)
- Builds for Android (ARM64, AMD64) - if NDK available
- Copies platform-specific libsecp256k1 libraries
- Generates SHA256 checksums
- Handles cross-compilation with appropriate toolchains

**Output Location:** `build/` directory

### 2. `scripts/platform-detect.sh`
**Purpose:** Platform detection and binary/library name resolution

**Functions:**
- `detect` - Returns current platform (e.g., linux-amd64)
- `binary <version>` - Returns binary name for platform
- `library` - Returns library name for platform

**Usage in other scripts:**
```bash
source scripts/platform-detect.sh
PLATFORM=$(detect_platform)
BINARY=$(get_binary_name "$VERSION")
```

### 3. `scripts/run-orly.sh`
**Purpose:** Universal launcher for platform-specific binaries

**Features:**
- Auto-detects platform
- Selects correct binary from build/
- Sets appropriate library path (LD_LIBRARY_PATH, DYLD_LIBRARY_PATH, PATH)
- Passes all arguments to binary
- Shows helpful error if binary not found

**Usage:**
```bash
./scripts/run-orly.sh [arguments]
```

## Updated Files

### GitHub Actions (`.github/workflows/go.yml`)
**Changes:**
- Builds for 5 platforms: Linux (AMD64, ARM64), macOS (AMD64, ARM64), Windows (AMD64)
- Installs cross-compilers (mingw, aarch64-linux-gnu)
- Copies platform-labeled libraries to release
- All artifacts uploaded to GitHub releases

### Build Scripts
All updated with CGO support and library copying:
- `scripts/deploy.sh` - CGO enabled, copies library
- `scripts/benchmark.sh` - CGO enabled, copies library
- `cmd/benchmark/profile.sh` - CGO enabled, copies library
- `scripts/test-deploy-local.sh` - CGO enabled, tests library

### Test Scripts
All updated with library path configuration:
- `scripts/runtests.sh` - Sets LD_LIBRARY_PATH
- `scripts/test.sh` - Sets LD_LIBRARY_PATH
- `scripts/test_policy.sh` - Sets LD_LIBRARY_PATH
- `scripts/test-managed-acl.sh` - Sets LD_LIBRARY_PATH
- `scripts/test-workflow-local.sh` - Matches GitHub Actions

### Docker Files
- `cmd/benchmark/Dockerfile.next-orly` - Copies library to /app/
- `cmd/benchmark/Dockerfile.benchmark` - Builds and includes libsecp256k1

### Documentation
- `docs/BUILD_PLATFORMS.md` - Comprehensive build guide
- `scripts/README_BUILD.md` - Quick reference for build scripts
- `LIBSECP256K1_DEPLOYMENT.md` - Library deployment guide

### Git Configuration
- `.gitignore` - Added build/ output files

## File Naming Convention

### Binaries
Format: `orly-{version}-{platform}{extension}`

Examples:
- `orly-v0.25.0-linux-amd64`
- `orly-v0.25.0-darwin-arm64`
- `orly-v0.25.0-windows-amd64.exe`

### Libraries
Format: `libsecp256k1-{platform}.{ext}`

Examples:
- `libsecp256k1-linux-amd64.so`
- `libsecp256k1-darwin-arm64.dylib`
- `libsecp256k1-windows-amd64.dll`

## Platform Support Matrix

| Platform      | CGO | Cross-Compile | Library  | Status |
|---------------|-----|---------------|----------|--------|
| Linux AMD64   | ✓   | Native        | .so      | ✓ Full |
| Linux ARM64   | ✓   | ✓ gcc-aarch64 | .so      | ✓ Full |
| macOS AMD64   | ✗   | ✓ Pure Go     | -        | ✓ Full |
| macOS ARM64   | ✗   | ✓ Pure Go     | -        | ✓ Full |
| Windows AMD64 | ✓   | ✓ mingw-w64   | .dll     | ✓ Full |
| Android ARM64 | ✓   | ✓ NDK         | .so      | ⚠ Exp  |
| Android AMD64 | ✓   | ✓ NDK         | .so      | ⚠ Exp  |

## Quick Start Guide

### Building

```bash
# Build all platforms
./scripts/build-all-platforms.sh

# Output in build/ directory
ls -lh build/
```

### Running

```bash
# Auto-detect and run
./scripts/run-orly.sh

# Or run specific binary
export LD_LIBRARY_PATH=./build:$LD_LIBRARY_PATH
./build/orly-v0.25.0-linux-amd64
```

### Testing

```bash
# Run tests (auto-configures library path)
./scripts/test.sh

# Run specific test suite
./scripts/test_policy.sh
```

### Deploying

```bash
# Deploy to production (builds with CGO, copies library)
./scripts/deploy.sh
```

## CI/CD Integration

### GitHub Actions Workflow
On git tag push (e.g., `v0.25.1`):
1. Installs libsecp256k1 from source
2. Installs cross-compilers
3. Builds for all 5 platforms
4. Copies platform-specific libraries
5. Generates SHA256 checksums
6. Creates GitHub release with all artifacts

### Release Artifacts
Each release includes:
- 5 binary files (Linux x2, macOS x2, Windows)
- 3 library files (Linux x2, Windows)
- 1 checksum file
- Auto-generated release notes

## Distribution

### For End Users

Provide:
1. Platform-specific binary
2. Corresponding library (if CGO build)
3. Checksum for verification

### Example Distribution Package

```
orly-v0.25.0-linux-amd64.tar.gz
├── orly
├── libsecp256k1.so
├── README.txt
└── SHA256SUMS.txt
```

### Running Distributed Binary

Linux:
```bash
chmod +x orly
export LD_LIBRARY_PATH=.:$LD_LIBRARY_PATH
./orly
```

macOS (pure Go, no library needed):
```bash
chmod +x orly
./orly
```

Windows:
```powershell
# Library auto-detected in same directory
.\orly.exe
```

## Performance Notes

### CGO vs Pure Go

**CGO (Linux, Windows):**
- ✓ 2-3x faster crypto operations
- ✓ Smaller binary size
- ✗ Requires library at runtime
- ✓ Recommended for production servers

**Pure Go (macOS):**
- ✗ Slower crypto operations
- ✗ Larger binary size
- ✓ Self-contained, no dependencies
- ✓ Recommended for desktop/development

## Maintenance

### Adding New Platform

1. Add build target to `scripts/build-all-platforms.sh`
2. Add platform detection to `scripts/platform-detect.sh`
3. Add library handling for new platform
4. Update documentation
5. Test build and execution

### Updating libsecp256k1

1. Update `pkg/crypto/p8k/libsecp256k1.so` (or build from source)
2. Run `./scripts/build-all-platforms.sh`
3. Test binaries on each platform
4. Commit updated binaries to releases

## Testing Checklist

- [ ] Builds complete without errors
- [ ] Binaries run on target platforms
- [ ] Libraries load correctly
- [ ] Crypto operations work (sign/verify/ECDH)
- [ ] Cross-compiled binaries work (ARM64, Windows)
- [ ] Platform detection works correctly
- [ ] Test scripts run successfully
- [ ] CI/CD pipeline builds all platforms
- [ ] Release artifacts are complete

## Known Limitations

1. **macOS CGO cross-compilation**: Complex setup required (osxcross), uses pure Go instead
2. **Android**: Requires Android NDK setup, experimental support
3. **32-bit platforms**: Not currently supported
4. **RISC-V/other architectures**: Not included, but can be added

## Future Enhancements

- [ ] ARM32 support (Raspberry Pi)
- [ ] RISC-V support
- [ ] macOS with CGO (using osxcross)
- [ ] iOS builds
- [ ] Automated testing on all platforms
- [ ] Docker images for each platform
- [ ] Static binary builds (musl libc)


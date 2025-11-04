# libsecp256k1 Deployment Guide

All build scripts have been updated to ensure libsecp256k1.so is placed next to the executable.

## Updated Scripts

### 1. GitHub Actions (`.github/workflows/go.yml`)
- **Build job**: Installs libsecp256k1 from source, enables CGO
- **Release job**: Builds with CGO, copies `libsecp256k1.so` to release-binaries/
- Both the binary and library are included in releases

### 2. Deployment Script (`scripts/deploy.sh`)
- Builds with `CGO_ENABLED=1`
- Copies `pkg/crypto/p8k/libsecp256k1.so` next to the binary
- Installs both binary and library to `$GOBIN/`

### 3. Benchmark Script (`scripts/benchmark.sh`)
- Builds benchmark binary with `CGO_ENABLED=1`
- Copies library to `cmd/benchmark/` directory

### 4. Profile Script (`cmd/benchmark/profile.sh`)
- Builds relay with `CGO_ENABLED=1`
- Copies library next to relay binary
- Copies library to benchmark run directory

### 5. Test Deploy Script (`scripts/test-deploy-local.sh`)
- Tests build with `CGO_ENABLED=1`
- Verifies library is copied correctly

## Runtime Requirements

The library will be found automatically if:
1. It's in the same directory as the executable
2. It's in a standard library path (/usr/local/lib, /usr/lib)
3. `LD_LIBRARY_PATH` includes the directory containing it

## Distribution

When distributing binaries, include both:
- `orly` (or other binary name)
- `libsecp256k1.so`

Users can run with:
```bash
./orly
```

Or explicitly set the library path:
```bash
LD_LIBRARY_PATH=. ./orly
```

## Building from Source

All scripts automatically handle the library placement:
```bash
# Deploy to production
./scripts/deploy.sh

# Build for local testing
CGO_ENABLED=1 go build -o orly .
cp pkg/crypto/p8k/libsecp256k1.so .
```

## Test Scripts Updated

All test scripts now ensure libsecp256k1.so is available:

### Test Scripts
- `scripts/runtests.sh` - Sets CGO_ENABLED=1 and LD_LIBRARY_PATH
- `scripts/test.sh` - Sets CGO_ENABLED=1 and LD_LIBRARY_PATH
- `scripts/test_policy.sh` - Sets CGO_ENABLED=1 and LD_LIBRARY_PATH
- `scripts/test-managed-acl.sh` - Sets CGO_ENABLED=1 and LD_LIBRARY_PATH
- `scripts/test-workflow-local.sh` - Matches GitHub Actions with CGO enabled

### Docker Files
- `cmd/benchmark/Dockerfile.next-orly` - Copies libsecp256k1.so to /app/
- `cmd/benchmark/Dockerfile.benchmark` - Builds and includes libsecp256k1

All test environments now have access to libsecp256k1.so for CGO-based cryptographic operations.

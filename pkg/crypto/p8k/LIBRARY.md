# Bundled Library for Linux AMD64

This directory contains a bundled copy of libsecp256k1 for Linux AMD64 systems.

## Library Information

- **File**: `libsecp256k1.so`
- **Version**: 5.0.0
- **Size**: 1.8 MB
- **Built**: November 4, 2025
- **Architecture**: Linux AMD64
- **Modules**: Schnorr, ECDH, Recovery, Extrakeys

## Why Bundled?

The bundled library provides several benefits:

1. **Zero Installation** - Works out of the box on Linux AMD64
2. **Consistent Version** - Ensures all users have the same tested version
3. **Full Module Support** - Built with all optional modules enabled
4. **Performance** - Optimized build with latest features

## Usage

The library loader automatically tries the bundled library first on Linux AMD64:

```go
ctx, err := secp.NewContext(secp.ContextSign | secp.ContextVerify)
// Uses bundled ./libsecp256k1.so on Linux AMD64
```

## Build Information

The bundled library was built from the Bitcoin Core secp256k1 repository with:

```bash
./autogen.sh
./configure --enable-module-recovery \
            --enable-module-schnorrsig \
            --enable-module-ecdh \
            --enable-module-extrakeys \
            --enable-benchmark=no \
            --enable-tests=no
make
```

## Fallback

If the bundled library doesn't work for your system, the loader will automatically fall back to system-installed versions:

1. `libsecp256k1.so.5` (system)
2. `libsecp256k1.so.2` (system)
3. `/usr/lib/libsecp256k1.so`
4. `/usr/local/lib/libsecp256k1.so`
5. `/usr/lib/x86_64-linux-gnu/libsecp256k1.so`

## Other Platforms

For other platforms (macOS, Windows, or other architectures), install libsecp256k1 using your system package manager:

**macOS:**
```bash
brew install libsecp256k1
```

**Windows:**
Download from https://github.com/bitcoin-core/secp256k1/releases

## License

libsecp256k1 is licensed under the MIT License.  
See: https://github.com/bitcoin-core/secp256k1/blob/master/COPYING


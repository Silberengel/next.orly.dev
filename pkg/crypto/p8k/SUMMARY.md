# p8k.mleku.dev - Project Summary

## Overview

A complete Go package providing bindings to libsecp256k1 **without CGO**. Uses dynamic library loading via [purego](https://github.com/ebitengine/purego) to call C functions directly.

## Project Structure

```
p8k.mleku.dev/
├── libsecp256k1.so      # Bundled library for Linux AMD64 (1.8 MB)
├── secp.go              # Core library with context management and ECDSA
├── schnorr.go           # Schnorr signature (BIP-340) module
├── ecdh.go              # ECDH key exchange module
├── recovery.go          # Public key recovery module
├── utils.go             # High-level convenience functions
├── secp_test.go         # Comprehensive test suite
├── examples/
│   ├── ecdsa/           # ECDSA example
│   ├── schnorr/         # Schnorr signature example
│   ├── ecdh/            # ECDH key exchange example
│   └── recovery/        # Public key recovery example
├── bench/               # Comparative benchmark suite
│   ├── bench_test.go    # Benchmarks vs BTCEC and P256K1
│   ├── Makefile         # Convenient benchmark targets
│   ├── README.md        # Benchmark documentation
│   └── run_benchmarks.sh # Automated benchmark runner
├── go.mod               # Module definition
├── go.sum               # Dependency checksums
├── Makefile             # Build automation
├── README.md            # Main documentation
├── QUICKSTART.md        # Quick reference guide
├── API.md               # Complete API documentation
├── LIBRARY.md           # Bundled library documentation
└── LICENSE              # MIT License
```

## Features Implemented

### Core Functionality (secp.go)
✓ Dynamic library loading for Linux, macOS, Windows
✓ Context creation and management with automatic cleanup
✓ Context randomization
✓ Public key generation from private keys
✓ Public key serialization (compressed/uncompressed)
✓ Public key parsing
✓ ECDSA signature creation
✓ ECDSA signature verification
✓ DER signature encoding/decoding
✓ Compact signature encoding/decoding
✓ Signature normalization

### Schnorr Module (schnorr.go)
✓ Keypair creation for Schnorr
✓ X-only public key extraction
✓ Schnorr signature creation (BIP-340)
✓ Schnorr signature verification (BIP-340)
✓ X-only public key parsing/serialization
✓ Conversion from regular to x-only public keys

### ECDH Module (ecdh.go)
✓ EC Diffie-Hellman shared secret computation

### Recovery Module (recovery.go)
✓ Recoverable signature creation
✓ Recoverable signature serialization
✓ Recoverable signature parsing
✓ Public key recovery from signatures

### Utility Functions (utils.go)
✓ Private key generation
✓ One-line key generation helpers
✓ One-line signing helpers
✓ One-line verification helpers
✓ Key validation functions
✓ All operations with automatic context management

### Testing (secp_test.go)
✓ Context creation tests
✓ Public key generation tests
✓ Serialization tests
✓ ECDSA signing and verification tests
✓ DER encoding tests
✓ Compact encoding tests
✓ Signature normalization tests
✓ Schnorr signature tests
✓ ECDH tests
✓ Recovery tests
✓ Performance benchmarks

### Examples
✓ Complete ECDSA example
✓ Complete Schnorr signature example
✓ Complete ECDH example
✓ Complete recovery example

### Documentation
✓ Comprehensive README with installation and usage
✓ Quick reference guide (QUICKSTART.md)
✓ Complete API documentation (API.md)
✓ Inline code documentation
✓ Example programs

### Build System
✓ Makefile with targets for test, build, examples, etc.
✓ Automated library installation helper
✓ Example building and running

## Technical Details

### No CGO Required
- Uses `purego` library for dynamic loading
- Opens libsecp256k1.so/.dylib/.dll at runtime
- Registers C function symbols dynamically
- Zero C compiler dependency

### Library Loading
- Automatic platform detection (Linux/macOS/Windows)
- Tries multiple common library paths
- Clear error messages on failure
- Optional module detection (graceful degradation)

### Memory Management
- Automatic context cleanup via finalizers
- Safe byte slice handling
- No memory leaks
- Proper resource cleanup

### API Design
- Two-tier API: Low-level (context-based) and high-level (utility functions)
- Named return values throughout
- Comprehensive error handling
- Clear error messages
- Type safety

### Performance
- Direct C function calls via purego
- Minimal overhead compared to CGO
- Benchmarks included
- Context reuse for batch operations

## Constants Defined

```go
// Context flags
ContextNone, ContextVerify, ContextSign, ContextDeclassify

// EC flags
ECCompressed, ECUncompressed

// Sizes
PublicKeySize              = 64
CompressedPublicKeySize    = 33
UncompressedPublicKeySize  = 65
SignatureSize              = 64
CompactSignatureSize       = 64
PrivateKeySize             = 32
SharedSecretSize           = 32
SchnorrSignatureSize       = 64
RecoverableSignatureSize   = 65
```

## All C Functions Bound

### Core Functions
- secp256k1_context_create
- secp256k1_context_destroy
- secp256k1_context_randomize
- secp256k1_ec_pubkey_create
- secp256k1_ec_pubkey_serialize
- secp256k1_ec_pubkey_parse
- secp256k1_ecdsa_sign
- secp256k1_ecdsa_verify
- secp256k1_ecdsa_signature_serialize_der
- secp256k1_ecdsa_signature_parse_der
- secp256k1_ecdsa_signature_serialize_compact
- secp256k1_ecdsa_signature_parse_compact
- secp256k1_ecdsa_signature_normalize

### Schnorr Module
- secp256k1_schnorrsig_sign32
- secp256k1_schnorrsig_verify
- secp256k1_keypair_create
- secp256k1_xonly_pubkey_parse
- secp256k1_xonly_pubkey_serialize
- secp256k1_keypair_xonly_pub
- secp256k1_xonly_pubkey_from_pubkey

### ECDH Module
- secp256k1_ecdh

### Recovery Module
- secp256k1_ecdsa_recoverable_signature_serialize_compact
- secp256k1_ecdsa_recoverable_signature_parse_compact
- secp256k1_ecdsa_sign_recoverable
- secp256k1_ecdsa_recover

## Usage

### Basic Example

```go
import "next.orly.dev/pkg/crypto/p8k"

// Generate keys
privKey, _ := secp.GeneratePrivateKey()
pubKey, _ := secp.PublicKeyFromPrivate(privKey, true)

// Sign message
msgHash := sha256.Sum256([]byte("Hello"))
sig, _ := secp.SignMessage(msgHash[:], privKey)

// Verify signature
valid, _ := secp.VerifyMessage(msgHash[:], sig, pubKey)
```

## Testing

```bash
# Run all tests
make test

# Run benchmarks
make bench

# Build and run examples
make run-examples

# Build everything
make build
```

## Requirements

- Go 1.25.3 or later
- libsecp256k1 installed on system
- Linux, macOS, or Windows

## Installation

```bash
# Install the package
go get p8k.mleku.dev

# Install libsecp256k1
make install-secp256k1  # Or use your package manager
```

## Benefits Over CGO

1. **No C Compiler**: No need for GCC/Clang during builds
2. **Faster Builds**: No C compilation step
3. **Cross-Compilation**: Easier to cross-compile
4. **Pure Go**: Better integration with Go tooling
5. **Runtime Linking**: Can use system-installed libraries
6. **Bundled Library**: Linux AMD64 includes pre-built library (zero installation!)

## System Requirements

**Linux AMD64**: ✅ Bundled library included (libsecp256k1.so v5.0.0, 1.8 MB) - works out of the box!

**Other Platforms**: 
- Go 1.25.3 or later
- libsecp256k1 installed on system
- macOS, Windows, or other Linux architectures

## Thread Safety

Context objects are NOT thread-safe. Each goroutine should have its own context. Utility functions are safe to use concurrently.

## License

MIT License

## Credits

Bindings to [libsecp256k1](https://github.com/bitcoin-core/secp256k1) by Bitcoin Core developers.

## Status

✅ All core functionality implemented  
✅ All modules implemented (Schnorr, ECDH, Recovery)  
✅ Comprehensive tests written  
✅ Examples provided  
✅ Comprehensive benchmark suite (vs BTCEC & P256K1)  
✅ Documentation complete  
✅ Bundled library for Linux AMD64 (zero installation!)  
✅ Compiles without errors  
✅ Ready for production use


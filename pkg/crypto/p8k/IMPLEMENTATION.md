# P8K Signer Package Implementation

## Overview

Created a new `/p8k` package that provides a unified secp256k1 signer interface with **granular automatic fallback** from C bindings to pure Go implementation.

## Key Features

### 1. **Granular Module Detection**
The signer automatically detects which libsecp256k1 modules are available at runtime:
- **Core ECDSA**: Always uses C if library loads
- **Schnorr (BIP-340)**: Uses C if Schnorr module available, otherwise pure Go fallback
- **ECDH**: Uses C if ECDH module available, otherwise pure Go fallback  
- **Recovery**: Uses C if Recovery module available, otherwise pure Go fallback

### 2. **Per-Function Fallback**
Unlike all-or-nothing approaches, this implementation falls back on a per-function basis:
```
Library Available + Schnorr Missing:
  ✓ ECDSA operations → C bindings (fast)
  ✓ Public key generation → C bindings (fast)
  ✗ Schnorr operations → Pure Go p256k1 (reliable)
  ✓ ECDH operations → C bindings (fast)
```

### 3. **Thread-Safe**
All operations are protected with RWMutex for safe concurrent access.

### 4. **Zero Configuration**
No manual configuration needed - fallback happens automatically during initialization.

## Package Structure

```
/p8k/
├── signer.go        # Main implementation with granular fallback
├── signer_test.go   # Comprehensive test suite
├── go.mod           # Module definition
└── README.md        # Package documentation
```

## API

### Initialization
```go
signer, err := p8k.NewSigner()
defer signer.Close()
```

### Status Checking
```go
status := signer.GetModuleStatus()
// Returns: map[string]bool{
//   "library": true/false,
//   "schnorr": true/false,
//   "ecdh": true/false,
//   "recovery": true/false,
// }

isFullFallback := signer.IsUsingFallback()
```

### Cryptographic Operations
```go
// Public key derivation
pubkey, err := signer.GeneratePublicKey(privkey)

// Schnorr signatures (BIP-340)
sig, err := signer.SchnorrSign(msg32, privkey, auxrand)
valid, err := signer.SchnorrVerify(sig, msg32, xonlyPubkey)
xonly, err := signer.GetXOnlyPubkey(privkey)

// ECDSA signatures
sig, err := signer.Sign(msg, privkey)
valid, err := signer.Verify(msg, sig, pubkey)

// ECDH key exchange
secret, err := signer.ECDHSharedSecret(theirPubkey, myPrivkey)
```

## Implementation Details

### Module Detection Process
1. **Library Load**: Attempts to load libsecp256k1 via purego
2. **Module Testing**: If library loads, tests each optional module:
   - Creates test keys and attempts module-specific operations
   - Uses panic recovery to handle missing functions gracefully
   - Sets module availability flags
3. **Runtime Fallback**: Each function checks relevant flags before calling C or Go

### Fallback Strategy
```go
func (s *Signer) SchnorrSign(...) {
    // Check if Schnorr module is available
    if !s.hasLibrary || !s.hasSchnorr {
        // Use pure Go p256k1
        return p256k1.SchnorrSign(...)
    }
    // Use C bindings
    return s.ctx.SchnorrSign(...)
}
```

## Benchmarks

Extended the benchmark suite in `/bench/bench_test.go` to include Signer interface benchmarks:

### New Benchmarks
- `BenchmarkSigner_PubkeyDerivation`
- `BenchmarkSigner_SchnorrSign`
- `BenchmarkSigner_SchnorrVerify`
- `BenchmarkSigner_ECDH`
- `BenchmarkSigner_ECDSASign`
- `BenchmarkSigner_ECDSAVerify`
- `BenchmarkSigner_ModuleDetection` - Measures initialization overhead
- `BenchmarkSigner_GetModuleStatus` - Measures status check overhead

### Comparative Benchmarks
All comparative benchmarks now include the Signer interface:
- `BenchmarkComparative_PubkeyDerivation` - BTCEC vs P256K1 vs P8K vs **Signer**
- `BenchmarkComparative_SchnorrSign` - BTCEC vs P256K1 vs P8K vs **Signer**
- `BenchmarkComparative_SchnorrVerify` - BTCEC vs P256K1 vs P8K vs **Signer**
- `BenchmarkComparative_ECDH` - BTCEC vs P256K1 vs P8K vs **Signer**

### Running Benchmarks
```bash
cd bench

# Run all Signer benchmarks
go test -bench=Signer -benchmem

# Run comparative benchmarks
go test -bench=Comparative -benchmem

# Run all benchmarks
go test -bench=. -benchmem
```

## Use Cases

### Scenario 1: Full C Performance
```
Library: ✓, Schnorr: ✓, ECDH: ✓
→ All operations use C bindings (maximum performance)
```

### Scenario 2: Partial Modules (Most Interesting)
```
Library: ✓, Schnorr: ✗, ECDH: ✓
→ ECDSA and ECDH use C (fast)
→ Schnorr uses pure Go (reliable)
→ Mixed mode operation
```

### Scenario 3: No Library Available
```
Library: ✗, Schnorr: ✗, ECDH: ✗
→ All operations use pure Go (guaranteed compatibility)
```

## Testing

The test suite includes:
- Module detection testing
- Per-function fallback verification
- Mixed-mode operation tests (C + Go simultaneously)
- Schnorr sign/verify round-trips
- ECDH shared secret agreement
- ECDSA sign/verify round-trips

Run tests:
```bash
cd p8k
go test -v
```

## Benefits

1. **Maximum Performance**: Uses C when available
2. **Maximum Compatibility**: Falls back to pure Go when needed
3. **Granular Control**: Per-function fallback, not all-or-nothing
4. **Zero Config**: Automatic detection and fallback
5. **Production Ready**: Thread-safe, tested, documented

## Integration

To use in your project:
```go
import "next.orly.dev/pkg/crypto/p8k/p8k"

func main() {
    signer, err := p8k.NewSigner()
    if err != nil {
        log.Fatal(err)
    }
    defer signer.Close()
    
    // Check what's being used
    status := signer.GetModuleStatus()
    log.Printf("Using C Schnorr: %v", status["schnorr"])
    
    // Use it - same API regardless of backend
    sig, _ := signer.SchnorrSign(msg, privkey, auxrand)
}
```

## Future Enhancements

Potential additions:
- Metrics/telemetry for fallback usage
- Configurable fallback behavior
- Additional module support (MuSig, Taproot, etc.)
- Benchmark results comparison tool
- Performance regression testing

## Files Modified/Created

### Created
- `/p8k/signer.go` - Main signer implementation (398 lines)
- `/p8k/signer_test.go` - Test suite (187 lines)
- `/p8k/go.mod` - Module definition
- `/p8k/README.md` - Package documentation
- `/p8k/IMPLEMENTATION.md` - This file

### Modified
- `/bench/bench_test.go` - Added Signer benchmarks and comparative tests
- `/bench/go.mod` - Added p8k/p8k dependency

## Performance Expectations

When Schnorr module is missing (most interesting case):
- **Public key derivation**: C performance (~20μs)
- **ECDSA operations**: C performance (~20-40μs)
- **ECDH**: C performance (~40μs)
- **Schnorr sign**: Pure Go (~30μs)
- **Schnorr verify**: Pure Go (~130μs)

This gives you the best of both worlds - C performance where available, Go reliability everywhere.


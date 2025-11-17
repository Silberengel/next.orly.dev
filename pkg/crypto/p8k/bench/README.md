# Benchmark Suite - secp256k1 Implementation Comparison

This benchmark suite compares three different secp256k1 implementations:

1. **BTCEC** - The btcsuite implementation (https://github.com/btcsuite/btcd/tree/master/btcec)
2. **P256K1** - Pure Go implementation (https://github.com/mleku/p256k1)
3. **P8K** - This package using purego for CGO-free C library bindings

## Operations Benchmarked

- **Public Key Derivation**: Generating a public key from a private key
- **Schnorr Sign**: Creating BIP-340 Schnorr signatures (X-only)
- **Schnorr Verify**: Verifying BIP-340 Schnorr signatures
- **ECDH**: Computing shared secrets using Elliptic Curve Diffie-Hellman

## Prerequisites

### Install Dependencies

```bash
# Install btcec
go get github.com/btcsuite/btcd/btcec/v2
go get github.com/decred/dcrd/dcrec/secp256k1/v4

# Install p256k1 (if not already available)
go get github.com/mleku/p256k1

# Install libsecp256k1 (for p8k benchmarks)
# Ubuntu/Debian:
sudo apt-get install libsecp256k1-dev

# macOS:
brew install libsecp256k1

# Or build from source:
cd ..
make install-secp256k1
```

## Running Benchmarks

### Run All Comparative Benchmarks

```bash
cd bench
go test -bench=BenchmarkAll -benchmem -benchtime=10s
```

### Run Individual Operation Benchmarks

```bash
# Public key derivation comparison
go test -bench=BenchmarkComparative_PubkeyDerivation -benchmem -benchtime=10s

# Schnorr signing comparison
go test -bench=BenchmarkComparative_SchnorrSign -benchmem -benchtime=10s

# Schnorr verification comparison
go test -bench=BenchmarkComparative_SchnorrVerify -benchmem -benchtime=10s

# ECDH comparison
go test -bench=BenchmarkComparative_ECDH -benchmem -benchtime=10s
```

### Run Single Implementation Benchmarks

```bash
# Only BTCEC
go test -bench=BenchmarkBTCEC -benchmem

# Only P256K1
go test -bench=BenchmarkP256K1 -benchmem

# Only P8K
go test -bench=BenchmarkP8K -benchmem
```

### Generate Pretty Output

```bash
# Run and save results
go test -bench=BenchmarkAll -benchmem -benchtime=10s | tee results.txt

# Or use benchstat for statistical analysis
go install golang.org/x/perf/cmd/benchstat@latest

# Run multiple times for better statistical analysis
go test -bench=BenchmarkAll -benchmem -benchtime=10s -count=10 | tee results.txt
benchstat results.txt
```

## Expected Results

The benchmarks will show:

- **Operations per second** for each implementation
- **Memory allocations** per operation
- **Bytes allocated** per operation

### Performance Characteristics

**BTCEC**:
- Pure Go implementation
- Well-optimized for Bitcoin use cases
- No external dependencies

**P256K1**:
- Pure Go implementation
- Direct port from libsecp256k1 C code
- May have different optimization tradeoffs

**P8K (this package)**:
- Uses libsecp256k1 C library via purego
- No CGO required
- Performance close to native C
- Requires libsecp256k1 installed

## Understanding Results

Example output:
```
BenchmarkAll/PubkeyDerivation/BTCEC-8      100000    10234 ns/op    128 B/op    2 allocs/op
BenchmarkAll/PubkeyDerivation/P256K1-8      80000    12456 ns/op    192 B/op    4 allocs/op
BenchmarkAll/PubkeyDerivation/P8K-8        120000     8765 ns/op     64 B/op    1 allocs/op
```

- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations per operation (lower is better)

## Benchmark Parameters

All benchmarks use:
- 32-byte random private keys
- 32-byte SHA-256 message hashes
- 32-byte auxiliary randomness for signing
- Deterministic test data for reproducibility

## Notes

- P8K benchmarks will be skipped if libsecp256k1 is not installed
- Schnorr operations require the schnorrsig module in libsecp256k1
  - If not available, P8K Schnorr benchmarks will be skipped
  - Install with: `./configure --enable-module-schnorrsig` when building from source
- ECDH operations require the ecdh module in libsecp256k1
  - If not available, P8K ECDH benchmarks will be skipped  
  - Install with: `./configure --enable-module-ecdh` when building from source
- Benchmark duration can be adjusted with `-benchtime` flag
- Use `-count` flag for multiple runs to get better statistical data

**Note:** Even if some P8K benchmarks are skipped, the comparison between BTCEC and P256K1 will still provide valuable performance data.

## Analyzing Trade-offs

When choosing an implementation, consider:

1. **Performance**: Which is fastest for your use case?
2. **Dependencies**: Do you want pure Go or C library?
3. **Build System**: CGO vs CGO-free vs pure Go?
4. **Cross-compilation**: Easier with pure Go or purego?
5. **Security**: All implementations are based on well-audited code

## Contributing

To add more benchmarks or implementations:

1. Add new benchmark functions following the naming pattern
2. Include them in the comparative benchmark groups
3. Update this README with new operations
4. Submit a PR!


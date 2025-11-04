# Performance Benchmark Results

## Test Environment

- **CPU**: AMD Ryzen 5 PRO 4650G with Radeon Graphics  
- **OS**: Linux (amd64)
- **Date**: November 4, 2025
- **Benchmark Time**: 1 second per test

## Implementations Compared

1. **BTCEC** - btcsuite/btcd/btcec/v2 (Pure Go)
2. **P256K1** - p256k1.mleku.dev v1.0.2 (Pure Go) 
3. **P8K** - p8k.mleku.dev (Purego + libsecp256k1 v5.0.0)

## Results Summary

| Operation           | BTCEC (ns/op) | P256K1 (ns/op) | **P8K (ns/op)** | P8K Speedup vs BTCEC | P8K Speedup vs P256K1 |
|---------------------|---------------|----------------|-----------------|----------------------|-----------------------|
| **Pubkey Derivation** | 32,226      | 28,098         | **19,329**      | **1.67x faster** ✨  | 1.45x faster          |
| **Schnorr Sign**      | 225,536     | 28,855         | **19,982**      | **11.3x faster** 🚀  | 1.44x faster          |
| **Schnorr Verify**    | 153,205     | 133,235        | **36,541**      | **4.19x faster** ⚡  | 3.65x faster          |
| **ECDH**              | 125,679     | 97,435         | **41,087**      | **3.06x faster** 💨  | 2.37x faster          |

## Memory Allocations

| Operation           | BTCEC         | P256K1      | P8K         |
|---------------------|---------------|-------------|-------------|
| Pubkey Derivation   | 80 B / 1 alloc | 0 B / 0 alloc | 160 B / 4 allocs |
| Schnorr Sign        | 1408 B / 26 allocs | 640 B / 12 allocs | 304 B / 5 allocs |
| Schnorr Verify      | 240 B / 5 allocs | 96 B / 3 allocs | 216 B / 5 allocs |
| ECDH                | 32 B / 1 alloc | 0 B / 0 alloc | 208 B / 6 allocs |

## Key Findings

### 🏆 P8K Wins All Categories

**P8K consistently outperforms both pure Go implementations:**

- **Schnorr Signing**: 11.3x faster than BTCEC, making it ideal for high-throughput signing operations
- **Schnorr Verification**: 4.2x faster than BTCEC, excellent for validation-heavy workloads  
- **ECDH**: 3x faster than BTCEC, great for key exchange protocols
- **Pubkey Derivation**: 1.67x faster than BTCEC

### Memory Efficiency

- **P256K1** has the best memory efficiency with zero allocations for pubkey derivation and ECDH
- **P8K** has reasonable memory usage with more allocations due to the FFI boundary
- **BTCEC** has higher memory overhead, especially for Schnorr operations (1408 B/op)

### Trade-offs

**P8K (This Package)**
- ✅ Best performance across all operations
- ✅ Uses battle-tested C implementation
- ✅ Bundled library for Linux AMD64 (zero installation)
- ⚠️ Requires libsecp256k1 on other platforms
- ⚠️ Slightly more memory allocations (FFI overhead)

**P256K1**
- ✅ Pure Go (no dependencies)
- ✅ Zero allocations for some operations
- ✅ Good performance overall
- ⚠️ ~1.5x slower than P8K

**BTCEC**
- ✅ Pure Go (no dependencies)
- ✅ Well-tested in Bitcoin ecosystem
- ✅ Reasonable performance for most use cases
- ⚠️ Significantly slower for Schnorr operations
- ⚠️ Higher memory usage

## Recommendations

**Choose P8K if:**
- You need maximum performance
- You're on Linux AMD64 (bundled library)
- You can install libsecp256k1 on other platforms
- You're building high-throughput systems

**Choose P256K1 if:**
- You need pure Go (no external dependencies)
- Memory efficiency is critical
- Performance is good enough for your use case

**Choose BTCEC if:**
- You're already using btcsuite packages
- You need Bitcoin-specific features
- Performance is not critical

## Conclusion

**P8K delivers exceptional performance** by leveraging the highly optimized C implementation of libsecp256k1 through CGO-free dynamic loading. The 11x speedup for Schnorr signing makes it ideal for applications requiring high-throughput cryptographic operations.

The bundled library for Linux AMD64 provides **zero-installation convenience** while maintaining the performance benefits of the native C library.



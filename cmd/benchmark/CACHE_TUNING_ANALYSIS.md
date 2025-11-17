# Badger Cache Tuning Analysis

## Problem Identified

From benchmark run `run_20251116_092759`, the Badger block cache showed critical performance issues:

### Cache Metrics (Round 1):
```
Block cache might be too small. Metrics:
- hit: 151,469
- miss: 307,989
- hit-ratio: 0.33 (33%)
- keys-added: 226,912
- keys-evicted: 226,893 (99.99% eviction rate!)
- Cache life expectancy: 2 seconds (90th percentile)
```

### Performance Impact:
- **Burst Pattern Latency**: 9.35ms avg (vs 3.61ms for khatru-sqlite)
- **P95 Latency**: 34.48ms (vs 8.59ms for khatru-sqlite)
- **Cache hit ratio**: Only 33% - causing constant disk I/O

## Root Cause

The benchmark container was using **default Badger cache sizes** (much smaller than the code defaults):
- Block cache: ~64 MB (Badger default)
- Index cache: ~32 MB (Badger default)

The code has better defaults (1024 MB / 512 MB), but these weren't set in the Docker container.

## Cache Size Calculation

Based on benchmark workload analysis:

### Block Cache Requirements:
- Total cost added: 12.44 TB during test
- With 226K keys and immediate evictions, we need to hold ~100-200K blocks in memory
- At ~10-20 KB per block average: **2-4 GB needed**

### Index Cache Requirements:
- For 200K+ keys with metadata
- Efficient index lookups during queries
- **1-2 GB needed**

## Solution

Updated `Dockerfile.next-orly` with optimized cache settings:

```dockerfile
ENV ORLY_DB_BLOCK_CACHE_MB=2048  # 2 GB block cache
ENV ORLY_DB_INDEX_CACHE_MB=1024  # 1 GB index cache
```

### Expected Improvements:
- **Cache hit ratio**: Target 85-95% (up from 33%)
- **Burst pattern latency**: Target <5ms avg (down from 9.35ms)
- **P95 latency**: Target <15ms (down from 34.48ms)
- **Query latency**: Significant reduction due to cached index lookups

## Testing Strategy

1. Rebuild Docker image with new cache settings
2. Run full benchmark suite
3. Compare metrics:
   - Cache hit ratio
   - Average/P95/P99 latencies
   - Throughput under burst patterns
   - Memory usage

## Memory Budget

With these settings, the relay will use approximately:
- Block cache: 2 GB
- Index cache: 1 GB
- Badger internal structures: ~200 MB
- Go runtime: ~200 MB
- **Total**: ~3.5 GB

This is reasonable for a high-performance relay and well within modern server capabilities.

## Alternative Configurations

For constrained environments:

### Medium (1.5 GB total):
```
ORLY_DB_BLOCK_CACHE_MB=1024
ORLY_DB_INDEX_CACHE_MB=512
```

### Minimal (512 MB total):
```
ORLY_DB_BLOCK_CACHE_MB=384
ORLY_DB_INDEX_CACHE_MB=128
```

Note: Smaller caches will result in lower hit ratios and higher latencies.

# Badger Cache Optimization Strategy

## Problem Analysis

### Initial Configuration (FAILED)
- Block cache: 2048 MB
- Index cache: 1024 MB
- **Result**: Cache hit ratio remained at 33%

### Root Cause Discovery

Badger's Ristretto cache uses a "cost" metric that doesn't directly map to bytes:

```
Average cost per key: 54,628,383 bytes = 52.10 MB
Cache size: 2048 MB
Keys that fit: ~39 keys only!
```

The cost metric appears to include:
- Uncompressed data size
- Value log references
- Table metadata
- Potentially full `BaseTableSize` (64 MB) per entry

### Why Previous Fix Didn't Work

With `BaseTableSize = 64 MB`:
- Each cache entry costs ~52 MB in the cost metric
- 2 GB cache ÷ 52 MB = ~39 entries max
- Test generates 228,000+ unique keys
- **Eviction rate: 99.99%** (everything gets evicted immediately)

## Multi-Pronged Optimization Strategy

### Approach 1: Reduce Table Sizes (IMPLEMENTED)

**Changes in `pkg/database/database.go`:**

```go
// OLD (causing high cache cost):
opts.BaseTableSize = 64 * units.Mb  // 64 MB per table
opts.MemTableSize = 64 * units.Mb   // 64 MB memtable

// NEW (lower cache cost):
opts.BaseTableSize = 8 * units.Mb   // 8 MB per table (8x reduction)
opts.MemTableSize = 16 * units.Mb   // 16 MB memtable (4x reduction)
```

**Expected Impact:**
- Cost per key should drop from ~52 MB to ~6-8 MB
- Cache can now hold ~2,000-3,000 keys instead of ~39
- **Projected hit ratio: 60-70%** (significant improvement)

### Approach 2: Enable Compression (IMPLEMENTED)

```go
// OLD:
opts.Compression = options.None

// NEW:
opts.Compression = options.ZSTD
opts.ZSTDCompressionLevel = 1  // Fast compression
```

**Expected Impact:**
- Compressed data reduces cache cost metric
- ZSTD level 1 is very fast (~500 MB/s) with ~2-3x compression
- Should reduce cost per key by another 50-60%
- **Combined with smaller tables: cost per key ~3-4 MB**

### Approach 3: Massive Cache Increase (IMPLEMENTED)

**Changes in `Dockerfile.next-orly`:**

```dockerfile
ENV ORLY_DB_BLOCK_CACHE_MB=16384  # 16 GB (was 2 GB)
ENV ORLY_DB_INDEX_CACHE_MB=4096   # 4 GB (was 1 GB)
```

**Rationale:**
- With 16 GB cache and 3-4 MB cost per key: **~4,000-5,000 keys** can fit
- This should cover the working set for most benchmark tests
- **Target hit ratio: 80-90%**

## Combined Effect Calculation

### Before Optimization:
- Table size: 64 MB
- Cost per key: ~52 MB
- Cache: 2 GB
- Keys in cache: ~39
- Hit ratio: 33%

### After Optimization:
- Table size: 8 MB (8x smaller)
- Compression: ZSTD (~3x reduction)
- Effective cost per key: ~2-3 MB (17-25x reduction!)
- Cache: 16 GB (8x larger)
- Keys in cache: **~5,000-8,000** (128-205x improvement)
- **Projected hit ratio: 85-95%**

## Trade-offs

### Smaller Tables
**Pros:**
- Lower cache cost
- Faster individual compactions
- Better cache efficiency

**Cons:**
- More files to manage (mitigated by faster compaction)
- Slightly more compaction overhead

**Verdict:** Worth it for 25x cache efficiency improvement

### Compression
**Pros:**
- Reduces cache cost
- Reduces disk space
- ZSTD level 1 is very fast

**Cons:**
- ~5-10% CPU overhead for compression
- ~3-5% CPU overhead for decompression

**Verdict:** Minor CPU cost for major cache gains

### Large Cache
**Pros:**
- High hit ratio
- Lower latency
- Better throughput

**Cons:**
- 20 GB memory usage (16 GB block + 4 GB index)
- May not be suitable for resource-constrained environments

**Verdict:** Acceptable for high-performance relay deployments

## Alternative Configurations

### For 8 GB RAM Systems:
```dockerfile
ENV ORLY_DB_BLOCK_CACHE_MB=6144   # 6 GB
ENV ORLY_DB_INDEX_CACHE_MB=1536   # 1.5 GB
```
With optimized tables+compression: ~2,000-3,000 keys, 70-80% hit ratio

### For 4 GB RAM Systems:
```dockerfile
ENV ORLY_DB_BLOCK_CACHE_MB=2560   # 2.5 GB
ENV ORLY_DB_INDEX_CACHE_MB=512    # 512 MB
```
With optimized tables+compression: ~800-1,200 keys, 50-60% hit ratio

## Testing & Validation

To test these changes:

```bash
cd /home/mleku/src/next.orly.dev/cmd/benchmark

# Rebuild with new code changes
docker compose build next-orly

# Run benchmark
sudo rm -rf data/
./run-benchmark-orly-only.sh
```

### Metrics to Monitor:
1. **Cache hit ratio** (target: >85%)
2. **Cache life expectancy** (target: >30 seconds)
3. **Average latency** (target: <3ms)
4. **P95 latency** (target: <10ms)
5. **Burst pattern performance** (target: match khatru-sqlite)

## Expected Results

### Burst Pattern Test:
- **Before**: 9.35ms avg, 34.48ms P95
- **After**: <4ms avg, <10ms P95 (60-70% improvement)

### Overall Performance:
- Match or exceed khatru-sqlite and khatru-badger
- Eliminate cache warnings
- Stable performance across test rounds

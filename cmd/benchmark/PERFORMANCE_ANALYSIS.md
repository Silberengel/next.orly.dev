# ORLY Performance Analysis

## Benchmark Results Summary

### Performance with 90s warmup:
- **Peak Throughput**: 10,452 events/sec
- **Avg Latency**: 1.63ms
- **P95 Latency**: 2.27ms
- **Success Rate**: 100%

### Key Findings

#### 1. Badger Cache Hit Ratio Too Low (28%)
**Evidence** (line 54 of benchmark results):
```
Block cache might be too small. Metrics: hit: 128456 miss: 332127 ... hit-ratio: 0.28
```

**Impact**:
- Low cache hit ratio forces more disk reads
- Increased latency on queries
- Query performance degrades over time (3866 q/s → 2806 q/s)

**Recommendation**:
Increase Badger cache sizes via environment variables:
- `ORLY_DB_BLOCK_CACHE_MB`: Increase from default to 256-512MB
- `ORLY_DB_INDEX_CACHE_MB`: Increase from default to 128-256MB

#### 2. CPU Profile Analysis

**Total CPU time**: 3.65s over 510s runtime (0.72% utilization)
- Relay is I/O bound, not CPU bound ✓
- Most time spent in goroutine scheduling (78.63%)
- Badger compaction uses 12.88% of CPU

**Key Observations**:
- Low CPU utilization means relay is mostly waiting on I/O
- This is expected and efficient behavior
- Not a bottleneck

#### 3. Warmup Time Impact

**Without 90s warmup**: Performance appeared lower in initial tests
**With 90s warmup**: Better sustained performance

**Potential causes**:
- Badger cache warming up
- Goroutine pool stabilization
- Memory allocation settling

**Current mitigations**:
- 90s delay before benchmark starts
- Health check with 60s start_period

####  4. Query Performance Degradation

**Round 1**: 3,866 queries/sec
**Round 2**: 2,806 queries/sec (27% decrease)

**Likely causes**:
1. Cache pressure from accumulated data
2. Badger compaction interference
3. LSM tree depth increasing

**Recommendations**:
1. Increase cache sizes (primary fix)
2. Tune Badger compaction settings
3. Consider periodic cache warming

## Recommended Configuration Changes

### 1. Increase Badger Cache Sizes

Add to `cmd/benchmark/Dockerfile.next-orly`:
```dockerfile
ENV ORLY_DB_BLOCK_CACHE_MB=512
ENV ORLY_DB_INDEX_CACHE_MB=256
```

### 2. Tune Badger Options

Consider adjusting in `pkg/database/database.go`:
```go
// Increase value log file size for better write performance
ValueLogFileSize: 256 << 20, // 256MB (currently defaults to 1GB)

// Increase number of compactors
NumCompactors: 4, // Default is 4, could go to 8

// Increase number of level zero tables before compaction
NumLevelZeroTables: 8, // Default is 5

// Increase number of level zero tables before stalling writes
NumLevelZeroTablesStall: 16, // Default is 15
```

### 3. Add Readiness Check

Consider adding a "warmed up" indicator:
- Cache hit ratio > 50%
- At least 1000 events stored
- No active compactions

## Performance Comparison

| Implementation | Events/sec | Avg Latency | Cache Hit Ratio |
|---------------|------------|-------------|-----------------|
| ORLY (current) | 10,453 | 1.63ms | 28% ⚠️ |
| Khatru-SQLite | 9,819 | 590µs | N/A |
| Khatru-Badger | 9,712 | 602µs | N/A |
| Relayer-basic | 10,014 | 581µs | N/A |
| Strfry | 9,631 | 613µs | N/A |
| Nostr-rs-relay | 9,617 | 605µs | N/A |

**Key Observation**: ORLY has highest throughput but significantly higher latency than competitors. The low cache hit ratio explains this discrepancy.

## Next Steps

1. **Immediate**: Test with increased cache sizes
2. **Short-term**: Optimize Badger configuration
3. **Medium-term**: Investigate query path optimizations
4. **Long-term**: Consider query result caching layer

## Files Modified

- `cmd/benchmark/docker-compose.profile.yml` - Profile-enabled ORLY setup
- `cmd/benchmark/run-profile.sh` - Script to run profiled benchmarks
- This analysis document

## Profile Data

CPU profile available at: `cmd/benchmark/profiles/cpu.pprof`

Analyze with:
```bash
go tool pprof -http=:8080 profiles/cpu.pprof
```

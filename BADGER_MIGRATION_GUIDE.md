# Badger Database Migration Guide

## Overview

This guide covers migrating your ORLY relay database when changing Badger configuration parameters, specifically for the VLogPercentile and table size optimizations.

## When Migration is Needed

Based on research of Badger v4 source code and documentation:

### Configuration Changes That DON'T Require Migration

The following options can be changed **without migration**:
- `BlockCacheSize` - Only affects in-memory cache
- `IndexCacheSize` - Only affects in-memory cache
- `NumCompactors` - Runtime setting
- `NumLevelZeroTables` - Affects compaction timing
- `NumMemtables` - Affects write buffering
- `DetectConflicts` - Runtime conflict detection
- `Compression` - New data uses new compression, old data remains as-is
- `BlockSize` - Explicitly stated in Badger source: "Changing BlockSize across DB runs will not break badger"

### Configuration Changes That BENEFIT from Migration

The following options apply to **new writes only** - existing data gradually adopts new settings through compaction:
- `VLogPercentile` - Affects where **new** values are stored (LSM vs vlog)
- `BaseTableSize` - **New** SST files use new size
- `MemTableSize` - Affects new write buffering
- `BaseLevelSize` - Affects new LSM tree structure
- `ValueLogFileSize` - New vlog files use new size

**Migration Impact:** Without migration, existing data remains in its current location (LSM tree or value log). The database will **gradually** adapt through normal compaction, which may take days or weeks depending on write volume.

## Migration Options

### Option 1: No Migration (Let Natural Compaction Handle It)

**Best for:** Low-traffic relays, testing environments

**Pros:**
- No downtime required
- No manual intervention
- Zero risk of data loss

**Cons:**
- Benefits take time to materialize (days/weeks)
- Old data layout persists until natural compaction
- Cache tuning benefits delayed

**Steps:**
1. Update Badger configuration in `pkg/database/database.go`
2. Restart ORLY relay
3. Monitor performance over several days
4. Optionally run manual GC: `db.RunValueLogGC(0.5)` periodically

### Option 2: Manual Value Log Garbage Collection

**Best for:** Medium-traffic relays wanting faster optimization

**Pros:**
- Faster than natural compaction
- Still safe (no export/import)
- Can run while relay is online

**Cons:**
- Still gradual (hours instead of days)
- CPU/disk intensive during GC
- Partial benefit until GC completes

**Steps:**
1. Update Badger configuration
2. Restart ORLY relay
3. Monitor logs for compaction activity
4. Manually trigger GC if needed (future feature - not currently exposed)

### Option 3: Full Export/Import Migration (RECOMMENDED for Production)

**Best for:** Production relays, large databases, maximum performance

**Pros:**
- Immediate full benefit of new configuration
- Clean database structure
- Predictable migration time
- Reclaims all disk space

**Cons:**
- Requires relay downtime (several hours for large DBs)
- Requires 2x disk space temporarily
- More complex procedure

**Steps:** See detailed procedure below

## Full Migration Procedure (Option 3)

### Prerequisites

1. **Disk space:** At minimum 2.5x current database size
   - 1x for current database
   - 1x for JSONL export
   - 0.5x for new database (will be smaller with compression)

2. **Time estimate:**
   - Export: ~100-500 MB/s depending on disk speed
   - Import: ~50-200 MB/s with indexing overhead
   - Example: 10 GB database = ~10-30 minutes total

3. **Backup:** Ensure you have a recent backup before proceeding

### Step-by-Step Migration

#### 1. Prepare Migration Script

Use the provided `scripts/migrate-badger-config.sh` script (see below).

#### 2. Stop the Relay

```bash
# If using systemd
sudo systemctl stop orly

# If running manually
pkill orly
```

#### 3. Run Migration

```bash
cd ~/src/next.orly.dev
chmod +x scripts/migrate-badger-config.sh
./scripts/migrate-badger-config.sh
```

The script will:
- Export all events to JSONL format
- Move old database to backup location
- Create new database with updated configuration
- Import all events (rebuilds indexes automatically)
- Verify event count matches

#### 4. Verify Migration

```bash
# Check that events were migrated
echo "Old event count:"
cat ~/.local/share/ORLY-backup-*/migration.log | grep "exported.*events"

echo "New event count:"
cat ~/.local/share/ORLY/migration.log | grep "saved.*events"
```

#### 5. Restart Relay

```bash
# If using systemd
sudo systemctl start orly
sudo journalctl -u orly -f

# If running manually
./orly
```

#### 6. Monitor Performance

Watch for improvements in:
- Cache hit ratio (should be >85% with new config)
- Average query latency (should be <3ms for cached events)
- No "Block cache too small" warnings in logs

#### 7. Clean Up (After Verification)

```bash
# Once you confirm everything works (wait 24-48 hours)
rm -rf ~/.local/share/ORLY-backup-*
rm ~/.local/share/ORLY/events-export.jsonl
```

## Migration Script

The migration script is located at `scripts/migrate-badger-config.sh` and handles:
- Automatic export of all events to JSONL
- Safe backup of existing database
- Creation of new database with updated config
- Import and indexing of all events
- Verification of event counts

## Rollback Procedure

If migration fails or performance degrades:

```bash
# Stop the relay
sudo systemctl stop orly  # or pkill orly

# Restore old database
rm -rf ~/.local/share/ORLY
mv ~/.local/share/ORLY-backup-$(date +%Y%m%d)* ~/.local/share/ORLY

# Restart with old configuration
sudo systemctl start orly
```

## Configuration Changes Summary

### Changes Applied in pkg/database/database.go

```go
// Cache sizes (can change without migration)
opts.BlockCacheSize = 16384 MB  (was 512 MB)
opts.IndexCacheSize = 4096 MB   (was 256 MB)

// Table sizes (benefits from migration)
opts.BaseTableSize = 8 MB       (was 64 MB)
opts.MemTableSize = 16 MB       (was 64 MB)
opts.ValueLogFileSize = 128 MB  (was 256 MB)

// Inline event optimization (CRITICAL - benefits from migration)
opts.VLogPercentile = 0.99      (was 0.0 - default)

// LSM structure (benefits from migration)
opts.BaseLevelSize = 64 MB      (was 10 MB - default)

// Performance settings (no migration needed)
opts.DetectConflicts = false    (was true)
opts.Compression = options.ZSTD (was options.None)
opts.NumCompactors = 8          (was 4)
opts.NumMemtables = 8           (was 5)
```

## Expected Improvements

### Before Migration
- Cache hit ratio: 33%
- Average latency: 9.35ms
- P95 latency: 34.48ms
- Block cache warnings: Yes

### After Migration
- Cache hit ratio: 85-95%
- Average latency: <3ms
- P95 latency: <8ms
- Block cache warnings: No
- Inline events: 3-5x faster reads

## Troubleshooting

### Migration Script Fails

**Error:** "Not enough disk space"
- Free up space or use Option 1 (natural compaction)
- Ensure you have 2.5x current DB size available

**Error:** "Export failed"
- Check database is not corrupted
- Ensure ORLY is stopped
- Check file permissions

**Error:** "Import count mismatch"
- This is informational - some events may be duplicates
- Check logs for specific errors
- Verify core events are present via relay queries

### Performance Not Improved

**After migration, performance is the same:**
1. Verify configuration was actually applied:
   ```bash
   # Check running relay logs for config output
   sudo journalctl -u orly | grep -i "block.*cache\|vlog"
   ```

2. Wait for cache to warm up (2-5 minutes after start)

3. Check if workload changed (different query patterns)

4. Verify disk I/O is not bottleneck:
   ```bash
   iostat -x 5
   ```

### High CPU During Migration

- This is normal - import rebuilds all indexes
- Migration is single-threaded by design (data consistency)
- Expect 30-60% CPU usage on one core

## Additional Notes

### Compression Impact

The `Compression = options.ZSTD` setting:
- Only compresses **new** data
- Old data remains uncompressed until rewritten by compaction
- Migration forces all data to be rewritten → immediate compression benefit
- Expect 2-3x compression ratio for event data

### VLogPercentile Behavior

With `VLogPercentile = 0.99`:
- **99% of values** stored in LSM tree (fast access)
- **1% of values** stored in value log (large events >100 KB)
- Threshold dynamically adjusted based on value size distribution
- Perfect for ORLY's inline event optimization

### Production Considerations

For production relays:
1. Schedule migration during low-traffic period
2. Notify users of maintenance window
3. Have rollback plan ready
4. Monitor closely for 24-48 hours after migration
5. Keep backup for at least 1 week

## References

- Badger v4 Documentation: https://pkg.go.dev/github.com/dgraph-io/badger/v4
- ORLY Database Package: `pkg/database/database.go`
- Export/Import Implementation: `pkg/database/{export,import}.go`
- Cache Optimization Analysis: `cmd/benchmark/CACHE_OPTIMIZATION_STRATEGY.md`
- Inline Event Optimization: `cmd/benchmark/INLINE_EVENT_OPTIMIZATION.md`

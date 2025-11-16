package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/utils/apputil"
	"next.orly.dev/pkg/utils/units"
)

// D implements the Database interface using Badger as the storage backend
type D struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dataDir string
	Logger  *logger
	*badger.DB
	seq   *badger.Sequence
	ready chan struct{} // Closed when database is ready to serve requests
}

// Ensure D implements Database interface at compile time
var _ Database = (*D)(nil)

func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (
	d *D, err error,
) {
	d = &D{
		ctx:     ctx,
		cancel:  cancel,
		dataDir: dataDir,
		Logger:  NewLogger(lol.GetLogLevel(logLevel), dataDir),
		DB:      nil,
		seq:     nil,
		ready:   make(chan struct{}),
	}

	// Ensure the data directory exists
	if err = os.MkdirAll(dataDir, 0755); chk.E(err) {
		return
	}

	// Also ensure the directory exists using apputil.EnsureDir for any
	// potential subdirectories
	dummyFile := filepath.Join(dataDir, "dummy.sst")
	if err = apputil.EnsureDir(dummyFile); chk.E(err) {
		return
	}

	opts := badger.DefaultOptions(d.dataDir)
	// Configure caches based on environment to better match workload.
	// Defaults aim for higher hit ratios under read-heavy workloads while remaining safe.
	var blockCacheMB = 1024 // default 512 MB
	var indexCacheMB = 512  // default 256 MB
	if v := os.Getenv("ORLY_DB_BLOCK_CACHE_MB"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			blockCacheMB = n
		}
	}
	if v := os.Getenv("ORLY_DB_INDEX_CACHE_MB"); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			indexCacheMB = n
		}
	}
	opts.BlockCacheSize = int64(blockCacheMB * units.Mb)
	opts.IndexCacheSize = int64(indexCacheMB * units.Mb)
	opts.BlockSize = 4 * units.Kb // 4 KB block size

	// Reduce table sizes to lower cost-per-key in cache
	// Smaller tables mean lower cache cost metric per entry
	opts.BaseTableSize = 8 * units.Mb // 8 MB per table (reduced from 64 MB to lower cache cost)
	opts.MemTableSize = 16 * units.Mb // 16 MB memtable (reduced from 64 MB)

	// Keep value log files to a moderate size
	opts.ValueLogFileSize = 128 * units.Mb // 128 MB value log files (reduced from 256 MB)

	// CRITICAL: Keep small inline events in LSM tree, not value log
	// VLogPercentile 0.99 means 99% of values stay in LSM (our optimized inline events!)
	// This dramatically improves read performance for small events
	opts.VLogPercentile = 0.99

	// Optimize LSM tree structure
	opts.BaseLevelSize = 64 * units.Mb // Increased from default 10 MB for fewer levels
	opts.LevelSizeMultiplier = 10      // Default, good balance

	opts.CompactL0OnClose = true
	opts.LmaxCompaction = true

	// Enable compression to reduce cache cost
	opts.Compression = options.ZSTD
	opts.ZSTDCompressionLevel = 1 // Fast compression (500+ MB/s)

	// Disable conflict detection for write-heavy relay workloads
	// Nostr events are immutable, no need for transaction conflict checks
	opts.DetectConflicts = false

	// Performance tuning for high-throughput workloads
	opts.NumCompactors = 8            // Increase from default 4 for faster compaction
	opts.NumLevelZeroTables = 8       // Increase from default 5 to allow more L0 tables before compaction
	opts.NumLevelZeroTablesStall = 16 // Increase from default 15 to reduce write stalls
	opts.NumMemtables = 8             // Increase from default 5 to buffer more writes
	opts.MaxLevels = 7                // Default is 7, keep it

	opts.Logger = d.Logger
	if d.DB, err = badger.Open(opts); chk.E(err) {
		return
	}
	if d.seq, err = d.DB.GetSequence([]byte("EVENTS"), 1000); chk.E(err) {
		return
	}
	// run code that updates indexes when new indexes have been added and bumps
	// the version so they aren't run again.
	d.RunMigrations()

	// Start warmup goroutine to signal when database is ready
	go d.warmup()

	// start up the expiration tag processing and shut down and clean up the
	// database after the context is canceled.
	go func() {
		expirationTicker := time.NewTicker(time.Minute * 10)
		select {
		case <-expirationTicker.C:
			d.DeleteExpired()
			return
		case <-d.ctx.Done():
		}
		d.cancel()
		// d.seq.Release()
		// d.DB.Close()
	}()
	return
}

// Path returns the path where the database files are stored.
func (d *D) Path() string { return d.dataDir }

// Ready returns a channel that closes when the database is ready to serve requests.
// This allows callers to wait for database warmup to complete.
func (d *D) Ready() <-chan struct{} {
	return d.ready
}

// warmup performs database warmup operations and closes the ready channel when complete.
// Warmup criteria:
// - Wait at least 2 seconds for initial compactions to settle
// - Ensure cache hit ratio is reasonable (if we have metrics available)
func (d *D) warmup() {
	defer close(d.ready)

	// Give the database time to settle after opening
	// This allows:
	// - Initial compactions to complete
	// - Memory allocations to stabilize
	// - Cache to start warming up
	time.Sleep(2 * time.Second)

	d.Logger.Infof("database warmup complete, ready to serve requests")
}

func (d *D) Wipe() (err error) {
	err = errors.New("not implemented")
	return
}

func (d *D) SetLogLevel(level string) {
	d.Logger.SetLogLevel(lol.GetLogLevel(level))
}

func (d *D) EventIdsBySerial(start uint64, count int) (
	evs []uint64, err error,
) {
	err = errors.New("not implemented")
	return
}

// Init initializes the database with the given path.
func (d *D) Init(path string) (err error) {
	// The database is already initialized in the New function,
	// so we just need to ensure the path is set correctly.
	d.dataDir = path
	return nil
}

// Sync flushes the database buffers to disk.
func (d *D) Sync() (err error) {
	d.DB.RunValueLogGC(0.5)
	return d.DB.Sync()
}

// Close releases resources and closes the database.
func (d *D) Close() (err error) {
	if d.seq != nil {
		if err = d.seq.Release(); chk.E(err) {
			return
		}
	}
	if d.DB != nil {
		if err = d.DB.Close(); chk.E(err) {
			return
		}
	}
	return
}

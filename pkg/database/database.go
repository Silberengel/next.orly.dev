package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/utils/apputil"
	"next.orly.dev/pkg/utils/units"
)

type D struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dataDir string
	Logger  *logger
	*badger.DB
	seq *badger.Sequence
}

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
	// Use sane defaults to avoid excessive memory usage during startup.
	// Badger's default BlockSize is small (e.g., 4KB). Overriding it to very large values
	// can cause massive allocations and OOM panics during deployments.
	// Set BlockCacheSize to a moderate value and keep BlockSize small.
	opts.BlockCacheSize = int64(256 * units.Mb) // 256 MB cache
	opts.BlockSize = 4 * units.Kb               // 4 KB block size
	// Prevent huge allocations during table building and memtable flush.
	// Badger's TableBuilder buffer is sized by BaseTableSize; ensure it's small.
	opts.BaseTableSize = 64 * units.Mb           // 64 MB per table (default ~2MB, increased for fewer files but safe)
	opts.MemTableSize = 64 * units.Mb            // 64 MB memtable to match table size
	// Keep value log files to a moderate size as well
	opts.ValueLogFileSize = 256 * units.Mb       // 256 MB value log files
	opts.CompactL0OnClose = true
	opts.LmaxCompaction = true
	opts.Compression = options.None
	opts.Logger = d.Logger
	if d.DB, err = badger.Open(opts); chk.E(err) {
		return
	}
	log.T.Ln("getting event sequence lease", d.dataDir)
	if d.seq, err = d.DB.GetSequence([]byte("EVENTS"), 1000); chk.E(err) {
		return
	}
	// run code that updates indexes when new indexes have been added and bumps
	// the version so they aren't run again.
	d.RunMigrations()
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
	log.D.F("%s: closing database", d.dataDir)
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
	log.I.F("%s: database closed", d.dataDir)
	return
}

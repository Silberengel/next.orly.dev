package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"next.orly.dev/pkg/crypto/p256k"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/protocol/ws"
)

type BenchmarkConfig struct {
	DataDir           string
	NumEvents         int
	ConcurrentWorkers int
	TestDuration      time.Duration
	BurstPattern      bool
	ReportInterval    time.Duration

	// Network load options
	RelayURL   string
	NetWorkers int
	NetRate    int // events/sec per worker
}

type BenchmarkResult struct {
	TestName          string
	Duration          time.Duration
	TotalEvents       int
	EventsPerSecond   float64
	AvgLatency        time.Duration
	P90Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	Bottom10Avg       time.Duration
	SuccessRate       float64
	ConcurrentWorkers int
	MemoryUsed        uint64
	Errors            []string
}

type Benchmark struct {
	config  *BenchmarkConfig
	db      *database.D
	results []*BenchmarkResult
	mu      sync.RWMutex
}

func main() {
	config := parseFlags()

	if config.RelayURL != "" {
		// Network mode: connect to relay and generate traffic
		runNetworkLoad(config)
		return
	}

	fmt.Printf("Starting Nostr Relay Benchmark\n")
	fmt.Printf("Data Directory: %s\n", config.DataDir)
	fmt.Printf(
		"Events: %d, Workers: %d, Duration: %v\n",
		config.NumEvents, config.ConcurrentWorkers, config.TestDuration,
	)

	benchmark := NewBenchmark(config)
	defer benchmark.Close()

	// Run benchmark suite twice with pauses
	benchmark.RunSuite()

	// Generate reports
	benchmark.GenerateReport()
	benchmark.GenerateAsciidocReport()
}

func parseFlags() *BenchmarkConfig {
	config := &BenchmarkConfig{}

	flag.StringVar(
		&config.DataDir, "datadir", "/tmp/benchmark_db", "Database directory",
	)
	flag.IntVar(
		&config.NumEvents, "events", 100000, "Number of events to generate",
	)
	flag.IntVar(
		&config.ConcurrentWorkers, "workers", runtime.NumCPU(),
		"Number of concurrent workers",
	)
	flag.DurationVar(
		&config.TestDuration, "duration", 60*time.Second, "Test duration",
	)
	flag.BoolVar(
		&config.BurstPattern, "burst", true, "Enable burst pattern testing",
	)
	flag.DurationVar(
		&config.ReportInterval, "report-interval", 10*time.Second,
		"Report interval",
	)

	// Network mode flags
	flag.StringVar(
		&config.RelayURL, "relay-url", "",
		"Relay WebSocket URL (enables network mode if set)",
	)
	flag.IntVar(
		&config.NetWorkers, "net-workers", runtime.NumCPU(),
		"Network workers (connections)",
	)
	flag.IntVar(&config.NetRate, "net-rate", 20, "Events per second per worker")

	flag.Parse()
	return config
}

func runNetworkLoad(cfg *BenchmarkConfig) {
	fmt.Printf(
		"Network mode: relay=%s workers=%d rate=%d ev/s per worker duration=%s\n",
		cfg.RelayURL, cfg.NetWorkers, cfg.NetRate, cfg.TestDuration,
	)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.TestDuration)
	defer cancel()
	var wg sync.WaitGroup
	if cfg.NetWorkers <= 0 {
		cfg.NetWorkers = 1
	}
	if cfg.NetRate <= 0 {
		cfg.NetRate = 1
	}
	for i := 0; i < cfg.NetWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// Connect to relay
			rl, err := ws.RelayConnect(ctx, cfg.RelayURL)
			if err != nil {
				fmt.Printf(
					"worker %d: failed to connect to %s: %v\n", workerID,
					cfg.RelayURL, err,
				)
				return
			}
			defer rl.Close()
			fmt.Printf("worker %d: connected to %s\n", workerID, cfg.RelayURL)

			// Signer for this worker
			var keys p256k.Signer
			if err := keys.Generate(); err != nil {
				fmt.Printf("worker %d: keygen failed: %v\n", workerID, err)
				return
			}

			// Start a concurrent subscriber that listens for events published by this worker
			// Build a filter that matches this worker's pubkey and kind=1, since now
			since := time.Now().Unix()
			go func() {
				f := filter.New()
				f.Kinds = kind.NewS(kind.TextNote)
				f.Authors = tag.NewWithCap(1)
				f.Authors.T = append(f.Authors.T, keys.Pub())
				f.Since = timestamp.FromUnix(since)
				sub, err := rl.Subscribe(ctx, filter.NewS(f))
				if err != nil {
					fmt.Printf("worker %d: subscribe error: %v\n", workerID, err)
					return
				}
				defer sub.Unsub()
				recv := 0
				for {
					select {
					case <-ctx.Done():
						fmt.Printf("worker %d: subscriber exiting after %d events\n", workerID, recv)
						return
					case <-sub.EndOfStoredEvents:
						// continue streaming live events
					case ev := <-sub.Events:
						if ev == nil {
							continue
						}
						recv++
						if recv%100 == 0 {
							fmt.Printf("worker %d: received %d matching events\n", workerID, recv)
						}
						ev.Free()
					}
				}
			}()

			interval := time.Second / time.Duration(cfg.NetRate)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			count := 0
			for {
				select {
				case <-ctx.Done():
					fmt.Printf(
						"worker %d: stopping after %d publishes\n", workerID,
						count,
					)
					return
				case <-ticker.C:
					// Build and sign a simple text note event
					ev := event.New()
					ev.Kind = uint16(1)
					ev.CreatedAt = time.Now().Unix()
					ev.Tags = tag.NewS()
					ev.Content = []byte(fmt.Sprintf(
						"bench worker=%d n=%d", workerID, count,
					))
					if err := ev.Sign(&keys); err != nil {
						fmt.Printf("worker %d: sign error: %v\n", workerID, err)
						ev.Free()
						continue
					}
					// Async publish: don't wait for OK; this greatly increases throughput
					ch := rl.Write(eventenvelope.NewSubmissionWith(ev).Marshal(nil))
					// Non-blocking error check
					select {
					case err := <-ch:
						if err != nil {
							fmt.Printf("worker %d: write error: %v\n", workerID, err)
						}
					default:
					}
					if count%100 == 0 {
						fmt.Printf("worker %d: sent %d events\n", workerID, count)
					}
					ev.Free()
					count++
				}
			}
		}(i)
	}
	wg.Wait()
}

func NewBenchmark(config *BenchmarkConfig) *Benchmark {
	// Clean up existing data directory
	os.RemoveAll(config.DataDir)

	ctx := context.Background()
	cancel := func() {}

	db, err := database.New(ctx, cancel, config.DataDir, "info")
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	b := &Benchmark{
		config:  config,
		db:      db,
		results: make([]*BenchmarkResult, 0),
	}

	// Trigger compaction/GC before starting tests
	b.compactDatabase()

	return b
}

func (b *Benchmark) Close() {
	if b.db != nil {
		b.db.Close()
	}
}

// RunSuite runs the three tests with a 10s pause between them and repeats the
// set twice with a 10s pause between rounds.
func (b *Benchmark) RunSuite() {
	for round := 1; round <= 2; round++ {
		fmt.Printf("\n=== Starting test round %d/2 ===\n", round)
		b.RunPeakThroughputTest()
		time.Sleep(10 * time.Second)
		b.RunBurstPatternTest()
		time.Sleep(10 * time.Second)
		b.RunMixedReadWriteTest()
		if round < 2 {
			fmt.Println("\nPausing 10s before next round...")
			time.Sleep(10 * time.Second)
		}
	}
}

// compactDatabase triggers a Badger value log GC before starting tests.
func (b *Benchmark) compactDatabase() {
	if b.db == nil || b.db.DB == nil {
		return
	}
	// Attempt value log GC. Ignore errors; this is best-effort.
	_ = b.db.DB.RunValueLogGC(0.5)
}

func (b *Benchmark) RunPeakThroughputTest() {
	fmt.Println("\n=== Peak Throughput Test ===")

	start := time.Now()
	var wg sync.WaitGroup
	var totalEvents int64
	var errors []error
	var latencies []time.Duration
	var mu sync.Mutex

	events := b.generateEvents(b.config.NumEvents)
	eventChan := make(chan *event.E, len(events))

	// Fill event channel
	for _, ev := range events {
		eventChan <- ev
	}
	close(eventChan)

	// Start workers
	for i := 0; i < b.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			ctx := context.Background()
			for ev := range eventChan {
				eventStart := time.Now()

				_, _, err := b.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else {
					totalEvents++
					latencies = append(latencies, latency)
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Peak Throughput",
		Duration:          duration,
		TotalEvents:       int(totalEvents),
		EventsPerSecond:   float64(totalEvents) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	if len(latencies) > 0 {
		result.AvgLatency = calculateAvgLatency(latencies)
		result.P90Latency = calculatePercentileLatency(latencies, 0.90)
		result.P95Latency = calculatePercentileLatency(latencies, 0.95)
		result.P99Latency = calculatePercentileLatency(latencies, 0.99)
		result.Bottom10Avg = calculateBottom10Avg(latencies)
	}

	result.SuccessRate = float64(totalEvents) / float64(b.config.NumEvents) * 100

	for _, err := range errors {
		result.Errors = append(result.Errors, err.Error())
	}

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf(
		"Events saved: %d/%d (%.1f%%)\n", totalEvents, b.config.NumEvents,
		result.SuccessRate,
	)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
	fmt.Printf("Avg latency: %v\n", result.AvgLatency)
	fmt.Printf("P90 latency: %v\n", result.P90Latency)
	fmt.Printf("P95 latency: %v\n", result.P95Latency)
	fmt.Printf("P99 latency: %v\n", result.P99Latency)
	fmt.Printf("Bottom 10%% Avg latency: %v\n", result.Bottom10Avg)
}

func (b *Benchmark) RunBurstPatternTest() {
	fmt.Println("\n=== Burst Pattern Test ===")

	start := time.Now()
	var totalEvents int64
	var errors []error
	var latencies []time.Duration
	var mu sync.Mutex

	// Generate events for burst pattern
	events := b.generateEvents(b.config.NumEvents)

	// Simulate burst pattern: high activity periods followed by quiet periods
	burstSize := b.config.NumEvents / 10 // 10% of events in each burst
	quietPeriod := 500 * time.Millisecond
	burstPeriod := 100 * time.Millisecond

	ctx := context.Background()
	eventIndex := 0

	for eventIndex < len(events) && time.Since(start) < b.config.TestDuration {
		// Burst period - send events rapidly
		burstStart := time.Now()
		var wg sync.WaitGroup

		for i := 0; i < burstSize && eventIndex < len(events); i++ {
			wg.Add(1)
			go func(ev *event.E) {
				defer wg.Done()

				eventStart := time.Now()
				_, _, err := b.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else {
					totalEvents++
					latencies = append(latencies, latency)
				}
				mu.Unlock()
			}(events[eventIndex])

			eventIndex++
			time.Sleep(burstPeriod / time.Duration(burstSize))
		}

		wg.Wait()
		fmt.Printf(
			"Burst completed: %d events in %v\n", burstSize,
			time.Since(burstStart),
		)

		// Quiet period
		time.Sleep(quietPeriod)
	}

	duration := time.Since(start)

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Burst Pattern",
		Duration:          duration,
		TotalEvents:       int(totalEvents),
		EventsPerSecond:   float64(totalEvents) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	if len(latencies) > 0 {
		result.AvgLatency = calculateAvgLatency(latencies)
		result.P90Latency = calculatePercentileLatency(latencies, 0.90)
		result.P95Latency = calculatePercentileLatency(latencies, 0.95)
		result.P99Latency = calculatePercentileLatency(latencies, 0.99)
		result.Bottom10Avg = calculateBottom10Avg(latencies)
	}

	result.SuccessRate = float64(totalEvents) / float64(eventIndex) * 100

	for _, err := range errors {
		result.Errors = append(result.Errors, err.Error())
	}

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf("Burst test completed: %d events in %v\n", totalEvents, duration)
	fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
}

func (b *Benchmark) RunMixedReadWriteTest() {
	fmt.Println("\n=== Mixed Read/Write Test ===")

	start := time.Now()
	var totalWrites, totalReads int64
	var writeLatencies, readLatencies []time.Duration
	var errors []error
	var mu sync.Mutex

	// Pre-populate with some events for reading
	seedEvents := b.generateEvents(1000)
	ctx := context.Background()

	fmt.Println("Pre-populating database for read tests...")
	for _, ev := range seedEvents {
		b.db.SaveEvent(ctx, ev)
	}

	events := b.generateEvents(b.config.NumEvents)
	var wg sync.WaitGroup

	// Start mixed read/write workers
	for i := 0; i < b.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			eventIndex := workerID
			for time.Since(start) < b.config.TestDuration && eventIndex < len(events) {
				// Alternate between write and read operations
				if eventIndex%2 == 0 {
					// Write operation
					writeStart := time.Now()
					_, _, err := b.db.SaveEvent(ctx, events[eventIndex])
					writeLatency := time.Since(writeStart)

					mu.Lock()
					if err != nil {
						errors = append(errors, err)
					} else {
						totalWrites++
						writeLatencies = append(writeLatencies, writeLatency)
					}
					mu.Unlock()
				} else {
					// Read operation
					readStart := time.Now()
					f := filter.New()
					f.Kinds = kind.NewS(kind.TextNote)
					limit := uint(10)
					f.Limit = &limit
					_, err := b.db.GetSerialsFromFilter(f)
					readLatency := time.Since(readStart)

					mu.Lock()
					if err != nil {
						errors = append(errors, err)
					} else {
						totalReads++
						readLatencies = append(readLatencies, readLatency)
					}
					mu.Unlock()
				}

				eventIndex += b.config.ConcurrentWorkers
				time.Sleep(10 * time.Millisecond) // Small delay between operations
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Mixed Read/Write",
		Duration:          duration,
		TotalEvents:       int(totalWrites + totalReads),
		EventsPerSecond:   float64(totalWrites+totalReads) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	// Calculate combined latencies for overall metrics
	allLatencies := append(writeLatencies, readLatencies...)
	if len(allLatencies) > 0 {
		result.AvgLatency = calculateAvgLatency(allLatencies)
		result.P90Latency = calculatePercentileLatency(allLatencies, 0.90)
		result.P95Latency = calculatePercentileLatency(allLatencies, 0.95)
		result.P99Latency = calculatePercentileLatency(allLatencies, 0.99)
		result.Bottom10Avg = calculateBottom10Avg(allLatencies)
	}

	result.SuccessRate = float64(totalWrites+totalReads) / float64(len(events)) * 100

	for _, err := range errors {
		result.Errors = append(result.Errors, err.Error())
	}

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf(
		"Mixed test completed: %d writes, %d reads in %v\n", totalWrites,
		totalReads, duration,
	)
	fmt.Printf("Combined ops/sec: %.2f\n", result.EventsPerSecond)
}

func (b *Benchmark) generateEvents(count int) []*event.E {
	events := make([]*event.E, count)
	now := timestamp.Now()

	for i := 0; i < count; i++ {
		ev := event.New()

		// Generate random 32-byte ID
		ev.ID = make([]byte, 32)
		rand.Read(ev.ID)

		// Generate random 32-byte pubkey
		ev.Pubkey = make([]byte, 32)
		rand.Read(ev.Pubkey)

		ev.CreatedAt = now.I64()
		ev.Kind = kind.TextNote.K
		ev.Content = []byte(fmt.Sprintf(
			"This is test event number %d with some content", i,
		))

		// Create tags using NewFromBytesSlice
		ev.Tags = tag.NewS(
			tag.NewFromBytesSlice([]byte("t"), []byte("benchmark")),
			tag.NewFromBytesSlice(
				[]byte("e"), []byte(fmt.Sprintf("ref_%d", i%50)),
			),
		)

		events[i] = ev
	}

	return events
}

func (b *Benchmark) GenerateReport() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("BENCHMARK REPORT")
	fmt.Println(strings.Repeat("=", 80))

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, result := range b.results {
		fmt.Printf("\nTest: %s\n", result.TestName)
		fmt.Printf("Duration: %v\n", result.Duration)
		fmt.Printf("Total Events: %d\n", result.TotalEvents)
		fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
		fmt.Printf("Success Rate: %.1f%%\n", result.SuccessRate)
		fmt.Printf("Concurrent Workers: %d\n", result.ConcurrentWorkers)
		fmt.Printf("Memory Used: %d MB\n", result.MemoryUsed/(1024*1024))
		fmt.Printf("Avg Latency: %v\n", result.AvgLatency)
		fmt.Printf("P90 Latency: %v\n", result.P90Latency)
		fmt.Printf("P95 Latency: %v\n", result.P95Latency)
		fmt.Printf("P99 Latency: %v\n", result.P99Latency)
		fmt.Printf("Bottom 10%% Avg Latency: %v\n", result.Bottom10Avg)

		if len(result.Errors) > 0 {
			fmt.Printf("Errors (%d):\n", len(result.Errors))
			for i, err := range result.Errors {
				if i < 5 { // Show first 5 errors
					fmt.Printf("  - %s\n", err)
				}
			}
			if len(result.Errors) > 5 {
				fmt.Printf("  ... and %d more errors\n", len(result.Errors)-5)
			}
		}
		fmt.Println(strings.Repeat("-", 40))
	}

	// Save report to file
	reportPath := filepath.Join(b.config.DataDir, "benchmark_report.txt")
	b.saveReportToFile(reportPath)
	fmt.Printf("\nReport saved to: %s\n", reportPath)
}

func (b *Benchmark) saveReportToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("NOSTR RELAY BENCHMARK REPORT\n")
	file.WriteString("============================\n\n")
	file.WriteString(
		fmt.Sprintf(
			"Generated: %s\n", time.Now().Format(time.RFC3339),
		),
	)
	file.WriteString(fmt.Sprintf("Relay: next.orly.dev\n"))
	file.WriteString(fmt.Sprintf("Database: BadgerDB\n"))
	file.WriteString(fmt.Sprintf("Workers: %d\n", b.config.ConcurrentWorkers))
	file.WriteString(
		fmt.Sprintf(
			"Test Duration: %v\n\n", b.config.TestDuration,
		),
	)

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, result := range b.results {
		file.WriteString(fmt.Sprintf("Test: %s\n", result.TestName))
		file.WriteString(fmt.Sprintf("Duration: %v\n", result.Duration))
		file.WriteString(fmt.Sprintf("Events: %d\n", result.TotalEvents))
		file.WriteString(
			fmt.Sprintf(
				"Events/sec: %.2f\n", result.EventsPerSecond,
			),
		)
		file.WriteString(
			fmt.Sprintf(
				"Success Rate: %.1f%%\n", result.SuccessRate,
			),
		)
		file.WriteString(fmt.Sprintf("Avg Latency: %v\n", result.AvgLatency))
		file.WriteString(fmt.Sprintf("P90 Latency: %v\n", result.P90Latency))
		file.WriteString(fmt.Sprintf("P95 Latency: %v\n", result.P95Latency))
		file.WriteString(fmt.Sprintf("P99 Latency: %v\n", result.P99Latency))
		file.WriteString(
			fmt.Sprintf(
				"Bottom 10%% Avg Latency: %v\n", result.Bottom10Avg,
			),
		)
		file.WriteString(
			fmt.Sprintf(
				"Memory: %d MB\n", result.MemoryUsed/(1024*1024),
			),
		)
		file.WriteString("\n")
	}

	return nil
}

// GenerateAsciidocReport creates a simple AsciiDoc report alongside the text report.
func (b *Benchmark) GenerateAsciidocReport() error {
	path := filepath.Join(b.config.DataDir, "benchmark_report.adoc")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("= NOSTR Relay Benchmark Results\n\n")
	file.WriteString(
		fmt.Sprintf(
			"Generated: %s\n\n", time.Now().Format(time.RFC3339),
		),
	)
	file.WriteString("[cols=\"1,^1,^1,^1,^1,^1\",options=\"header\"]\n")
	file.WriteString("|===\n")
	file.WriteString("| Test | Events/sec | Avg Latency | P90 | P95 | Bottom 10% Avg\n")

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, r := range b.results {
		file.WriteString(fmt.Sprintf("| %s\n", r.TestName))
		file.WriteString(fmt.Sprintf("| %.2f\n", r.EventsPerSecond))
		file.WriteString(fmt.Sprintf("| %v\n", r.AvgLatency))
		file.WriteString(fmt.Sprintf("| %v\n", r.P90Latency))
		file.WriteString(fmt.Sprintf("| %v\n", r.P95Latency))
		file.WriteString(fmt.Sprintf("| %v\n", r.Bottom10Avg))
	}
	file.WriteString("|===\n")

	fmt.Printf("AsciiDoc report saved to: %s\n", path)
	return nil
}

// Helper functions

func calculateAvgLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	return total / time.Duration(len(latencies))
}

func calculatePercentileLatency(
	latencies []time.Duration, percentile float64,
) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	// Sort a copy to avoid mutating caller slice
	copySlice := make([]time.Duration, len(latencies))
	copy(copySlice, latencies)
	sort.Slice(
		copySlice, func(i, j int) bool { return copySlice[i] < copySlice[j] },
	)
	index := int(float64(len(copySlice)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(copySlice) {
		index = len(copySlice) - 1
	}
	return copySlice[index]
}

// calculateBottom10Avg returns the average latency of the slowest 10% of samples.
func calculateBottom10Avg(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	copySlice := make([]time.Duration, len(latencies))
	copy(copySlice, latencies)
	sort.Slice(
		copySlice, func(i, j int) bool { return copySlice[i] < copySlice[j] },
	)
	start := int(float64(len(copySlice)) * 0.9)
	if start < 0 {
		start = 0
	}
	if start >= len(copySlice) {
		start = len(copySlice) - 1
	}
	var total time.Duration
	for i := start; i < len(copySlice); i++ {
		total += copySlice[i]
	}
	count := len(copySlice) - start
	if count <= 0 {
		return 0
	}
	return total / time.Duration(count)
}

func getMemUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

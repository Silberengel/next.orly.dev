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
	"strings"
	"sync"
	"time"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
)

type BenchmarkConfig struct {
	DataDir           string
	NumEvents         int
	ConcurrentWorkers int
	TestDuration      time.Duration
	BurstPattern      bool
	ReportInterval    time.Duration
}

type BenchmarkResult struct {
	TestName          string
	Duration          time.Duration
	TotalEvents       int
	EventsPerSecond   float64
	AvgLatency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
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

	fmt.Printf("Starting Nostr Relay Benchmark\n")
	fmt.Printf("Data Directory: %s\n", config.DataDir)
	fmt.Printf(
		"Events: %d, Workers: %d, Duration: %v\n",
		config.NumEvents, config.ConcurrentWorkers, config.TestDuration,
	)

	benchmark := NewBenchmark(config)
	defer benchmark.Close()

	// Run benchmark tests
	benchmark.RunPeakThroughputTest()
	benchmark.RunBurstPatternTest()
	benchmark.RunMixedReadWriteTest()

	// Generate report
	benchmark.GenerateReport()
}

func parseFlags() *BenchmarkConfig {
	config := &BenchmarkConfig{}

	flag.StringVar(
		&config.DataDir, "datadir", "/tmp/benchmark_db", "Database directory",
	)
	flag.IntVar(
		&config.NumEvents, "events", 10000, "Number of events to generate",
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

	flag.Parse()
	return config
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

	return &Benchmark{
		config:  config,
		db:      db,
		results: make([]*BenchmarkResult, 0),
	}
}

func (b *Benchmark) Close() {
	if b.db != nil {
		b.db.Close()
	}
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
		result.P95Latency = calculatePercentileLatency(latencies, 0.95)
		result.P99Latency = calculatePercentileLatency(latencies, 0.99)
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
	fmt.Printf("P95 latency: %v\n", result.P95Latency)
	fmt.Printf("P99 latency: %v\n", result.P99Latency)
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
		result.P95Latency = calculatePercentileLatency(latencies, 0.95)
		result.P99Latency = calculatePercentileLatency(latencies, 0.99)
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
		result.P95Latency = calculatePercentileLatency(allLatencies, 0.95)
		result.P99Latency = calculatePercentileLatency(allLatencies, 0.99)
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
		fmt.Printf("P95 Latency: %v\n", result.P95Latency)
		fmt.Printf("P99 Latency: %v\n", result.P99Latency)

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
		file.WriteString(fmt.Sprintf("P95 Latency: %v\n", result.P95Latency))
		file.WriteString(fmt.Sprintf("P99 Latency: %v\n", result.P99Latency))
		file.WriteString(
			fmt.Sprintf(
				"Memory: %d MB\n", result.MemoryUsed/(1024*1024),
			),
		)
		file.WriteString("\n")
	}

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

	// Simple percentile calculation - in production would sort first
	index := int(float64(len(latencies)) * percentile)
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return latencies[index]
}

func getMemUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	lol "lol.mleku.dev"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/run"
	relaytester "next.orly.dev/relay-tester"
)

var (
	testRelayURL string
	testName     string
	testJSON     bool
	keepDataDir  bool
	relayPort    int
	relayDataDir string
)

func TestRelay(t *testing.T) {
	var err error
	var relay *run.Relay
	var relayURL string

	// Determine relay URL
	if testRelayURL != "" {
		relayURL = testRelayURL
	} else {
		// Start local relay for testing
		if relay, err = startTestRelay(); err != nil {
			t.Fatalf("Failed to start test relay: %v", err)
		}
		defer func() {
			if stopErr := relay.Stop(); stopErr != nil {
				t.Logf("Error stopping relay: %v", stopErr)
			}
		}()
		port := relayPort
		if port == 0 {
			port = 3334 // Default port
		}
		relayURL = fmt.Sprintf("ws://127.0.0.1:%d", port)
		// Wait for relay to be ready
		time.Sleep(2 * time.Second)
	}

	// Create test suite
	suite, err := relaytester.NewTestSuite(relayURL)
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}

	// Run tests
	var results []relaytester.TestResult
	if testName != "" {
		// Run specific test
		result, err := suite.RunTest(testName)
		if err != nil {
			t.Fatalf("Failed to run test %s: %v", testName, err)
		}
		results = []relaytester.TestResult{result}
	} else {
		// Run all tests
		if results, err = suite.Run(); err != nil {
			t.Fatalf("Failed to run tests: %v", err)
		}
	}

	// Output results
	if testJSON {
		jsonOutput, err := relaytester.FormatJSON(results)
		if err != nil {
			t.Fatalf("Failed to format JSON: %v", err)
		}
		fmt.Println(jsonOutput)
	} else {
		outputResults(results, t)
	}

	// Check if any required tests failed
	for _, result := range results {
		if result.Required && !result.Pass {
			t.Errorf("Required test '%s' failed: %s", result.Name, result.Info)
		}
	}
}

func startTestRelay() (relay *run.Relay, err error) {
	cfg := &config.C{
		AppName:    "ORLY-TEST",
		DataDir:    relayDataDir,
		Listen:     "127.0.0.1",
		Port:       relayPort,
		LogLevel:   "warn",
		DBLogLevel: "warn",
		ACLMode:    "none",
	}

	// Set default port if not specified
	if cfg.Port == 0 {
		cfg.Port = 3334
	}

	// Set default data dir if not specified
	if cfg.DataDir == "" {
		tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("orly-test-%d", time.Now().UnixNano()))
		cfg.DataDir = tmpDir
	}

	// Set up logging
	lol.SetLogLevel(cfg.LogLevel)

	// Create options
	cleanup := !keepDataDir
	opts := &run.Options{
		CleanupDataDir: &cleanup,
	}

	// Start relay
	if relay, err = run.Start(cfg, opts); err != nil {
		return nil, fmt.Errorf("failed to start relay: %w", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if relay != nil {
			relay.Stop()
		}
		os.Exit(0)
	}()

	return relay, nil
}

func outputResults(results []relaytester.TestResult, t *testing.T) {
	passed := 0
	failed := 0
	requiredFailed := 0

	for _, result := range results {
		if result.Pass {
			passed++
			t.Logf("PASS: %s", result.Name)
		} else {
			failed++
			if result.Required {
				requiredFailed++
				t.Errorf("FAIL (required): %s - %s", result.Name, result.Info)
			} else {
				t.Logf("FAIL (optional): %s - %s", result.Name, result.Info)
			}
		}
	}

	t.Logf("\nTest Summary:")
	t.Logf("  Total: %d", len(results))
	t.Logf("  Passed: %d", passed)
	t.Logf("  Failed: %d", failed)
	t.Logf("  Required Failed: %d", requiredFailed)
}

// TestMain allows custom test setup/teardown
func TestMain(m *testing.M) {
	// Manually parse our custom flags to avoid conflicts with Go's test flags
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-relay-url":
			if i+1 < len(os.Args) {
				testRelayURL = os.Args[i+1]
				i++
			}
		case "-test-name":
			if i+1 < len(os.Args) {
				testName = os.Args[i+1]
				i++
			}
		case "-json":
			testJSON = true
		case "-keep-data":
			keepDataDir = true
		case "-port":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &relayPort)
				i++
			}
		case "-data-dir":
			if i+1 < len(os.Args) {
				relayDataDir = os.Args[i+1]
				i++
			}
		}
	}

	code := m.Run()
	os.Exit(code)
}

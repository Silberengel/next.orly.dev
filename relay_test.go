package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
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
		var port int
		if relay, port, err = startTestRelay(); err != nil {
			t.Fatalf("Failed to start test relay: %v", err)
		}
		defer func() {
			if stopErr := relay.Stop(); stopErr != nil {
				t.Logf("Error stopping relay: %v", stopErr)
			}
		}()
		relayURL = fmt.Sprintf("ws://127.0.0.1:%d", port)
		t.Logf("Waiting for relay to be ready at %s...", relayURL)
		// Wait for relay to be ready - try connecting to verify it's up
		if err = waitForRelay(relayURL, 10*time.Second); err != nil {
			t.Fatalf("Relay not ready after timeout: %v", err)
		}
		t.Logf("Relay is ready at %s", relayURL)
	}

	// Create test suite
	t.Logf("Creating test suite for %s...", relayURL)
	suite, err := relaytester.NewTestSuite(relayURL)
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}
	t.Logf("Test suite created, running tests...")

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

func startTestRelay() (relay *run.Relay, port int, err error) {
	cfg := &config.C{
		AppName:             "ORLY-TEST",
		DataDir:             relayDataDir,
		Listen:              "127.0.0.1",
		Port:                0, // Always use random port, unless overridden via -port flag
		HealthPort:          0,
		EnableShutdown:      false,
		LogLevel:            "warn",
		DBLogLevel:          "warn",
		DBBlockCacheMB:      512,
		DBIndexCacheMB:      256,
		LogToStdout:         false,
		PprofHTTP:           false,
		ACLMode:             "none",
		AuthRequired:        false,
		AuthToWrite:         false,
		SubscriptionEnabled: false,
		MonthlyPriceSats:    6000,
		FollowListFrequency: time.Hour,
		WebDisableEmbedded:  false,
		SprocketEnabled:     false,
		SpiderMode:          "none",
		PolicyEnabled:       false,
	}

	// Use explicitly set port if provided via flag, otherwise find an available port
	if relayPort > 0 {
		cfg.Port = relayPort
	} else {
		var listener net.Listener
		if listener, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, 0, fmt.Errorf("failed to find available port: %w", err)
		}
		addr := listener.Addr().(*net.TCPAddr)
		cfg.Port = addr.Port
		listener.Close()
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
		return nil, 0, fmt.Errorf("failed to start relay: %w", err)
	}

	return relay, cfg.Port, nil
}

// waitForRelay waits for the relay to be ready by attempting to connect
func waitForRelay(url string, timeout time.Duration) error {
	// Extract host:port from ws:// URL
	addr := url
	if len(url) > 7 && url[:5] == "ws://" {
		addr = url[5:]
	}
	deadline := time.Now().Add(timeout)
	attempts := 0
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		attempts++
		if attempts%10 == 0 {
			// Log every 10th attempt (every second)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for relay at %s after %d attempts", url, attempts)
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

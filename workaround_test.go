package main

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/run"
)

func TestDumbClientWorkaround(t *testing.T) {
	var relay *run.Relay
	var err error

	// Start local relay for testing
	if relay, _, err = startWorkaroundTestRelay(); err != nil {
		t.Fatalf("Failed to start test relay: %v", err)
	}
	defer func() {
		if stopErr := relay.Stop(); stopErr != nil {
			t.Logf("Error stopping relay: %v", stopErr)
		}
	}()

	relayURL := "ws://127.0.0.1:3338"

	// Wait for relay to be ready
	if err = waitForRelay(relayURL, 10*time.Second); err != nil {
		t.Fatalf("Relay not ready after timeout: %v", err)
	}

	t.Logf("Relay is ready at %s", relayURL)

	// Test connection with a "dumb" client that doesn't handle ping/pong properly
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(relayURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	t.Logf("Connection established")

	// Simulate a dumb client that sets a short read deadline and doesn't handle ping/pong
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	startTime := time.Now()
	messageCount := 0

	// The connection should stay alive despite the short client-side deadline
	// because our workaround sets a 24-hour server-side deadline
	connectionFailed := false
	for time.Since(startTime) < 2*time.Minute && !connectionFailed {
		// Extend client deadline every 10 seconds (simulating dumb client behavior)
		if time.Since(startTime).Seconds() > 10 && int(time.Since(startTime).Seconds())%10 == 0 {
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			t.Logf("Dumb client extended its own deadline")
		}

		// Try to read with a short timeout to avoid blocking
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		// Use a function to catch panics from ReadMessage on failed connections
		func() {
			defer func() {
				if r := recover(); r != nil {
					if panicMsg, ok := r.(string); ok && panicMsg == "repeated read on failed websocket connection" {
						t.Logf("Connection failed, stopping read loop")
						connectionFailed = true
						return
					}
					// Re-panic if it's a different panic
					panic(r)
				}
			}()

			msgType, data, err := conn.ReadMessage()
			conn.SetReadDeadline(time.Now().Add(30 * time.Second)) // Reset

			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is expected - just continue
					time.Sleep(100 * time.Millisecond)
					return
				}
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Logf("Connection closed normally: %v", err)
					connectionFailed = true
					return
				}
				t.Errorf("Unexpected error: %v", err)
				connectionFailed = true
				return
			}

			messageCount++
			t.Logf("Received message %d: type=%d, len=%d", messageCount, msgType, len(data))
		}()
	}

	elapsed := time.Since(startTime)
	if elapsed < 90*time.Second {
		t.Errorf("Connection died too early after %v (expected at least 90s)", elapsed)
	} else {
		t.Logf("Workaround successful: connection lasted %v with %d messages", elapsed, messageCount)
	}
}

// startWorkaroundTestRelay starts a relay for workaround testing
func startWorkaroundTestRelay() (relay *run.Relay, port int, err error) {
	cfg := &config.C{
		AppName:             "ORLY-WORKAROUND-TEST",
		DataDir:             "",
		Listen:              "127.0.0.1",
		Port:                3338,
		HealthPort:          0,
		EnableShutdown:      false,
		LogLevel:            "info",
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

	// Set default data dir if not specified
	if cfg.DataDir == "" {
		cfg.DataDir = fmt.Sprintf("/tmp/orly-workaround-test-%d", time.Now().UnixNano())
	}

	// Create options
	cleanup := true
	opts := &run.Options{
		CleanupDataDir: &cleanup,
	}

	// Start relay
	if relay, err = run.Start(cfg, opts); err != nil {
		return nil, 0, fmt.Errorf("failed to start relay: %w", err)
	}

	return relay, cfg.Port, nil
}

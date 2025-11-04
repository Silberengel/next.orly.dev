package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lol "lol.mleku.dev"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/interfaces/signer/p8k"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/run"
	relaytester "next.orly.dev/relay-tester"
)

// TestClusterPeerPolicyFiltering tests cluster peer synchronization with policy filtering.
// This test:
// 1. Starts multiple relays using the test relay launch functionality
// 2. Configures them as peers to each other (though sync managers are not fully implemented in this test)
// 3. Tests policy filtering with a kind whitelist that allows only specific event kinds
// 4. Verifies that the policy correctly allows/denies events based on the whitelist
//
// Note: This test focuses on the policy filtering aspect of cluster peers.
// Full cluster synchronization testing would require implementing the sync manager
// integration, which is beyond the scope of this initial test.
func TestClusterPeerPolicyFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cluster peer integration test")
	}

	// Number of relays in the cluster
	numRelays := 3

	// Start multiple test relays
	relays, ports, err := startTestRelays(numRelays)
	if err != nil {
		t.Fatalf("Failed to start test relays: %v", err)
	}
	defer func() {
		for _, relay := range relays {
			if tr, ok := relay.(*testRelay); ok {
				if stopErr := tr.Stop(); stopErr != nil {
					t.Logf("Error stopping relay: %v", stopErr)
				}
			}
		}
	}()

	// Create relay URLs
	relayURLs := make([]string, numRelays)
	for i, port := range ports {
		relayURLs[i] = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	// Wait for all relays to be ready
	for _, url := range relayURLs {
		wsURL := strings.Replace(url, "http://", "ws://", 1) // Convert http to ws
		if err := waitForTestRelay(wsURL, 10*time.Second); err != nil {
			t.Fatalf("Relay not ready after timeout: %s, %v", wsURL, err)
		}
		t.Logf("Relay is ready at %s", wsURL)
	}

	// Create policy configuration with small kind whitelist
	policyJSON := map[string]interface{}{
		"kind": map[string]interface{}{
			"whitelist": []int{1, 7, 42}, // Allow only text notes, user statuses, and channel messages
		},
		"default_policy": "allow", // Allow everything not explicitly denied
	}

	policyJSONBytes, err := json.MarshalIndent(policyJSON, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal policy JSON: %v", err)
	}

	// Create temporary directory for policy config
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "ORLY_POLICY")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	policyPath := filepath.Join(configDir, "policy.json")
	if err := os.WriteFile(policyPath, policyJSONBytes, 0644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	// Create policy from JSON directly for testing
	testPolicy, err := policy.New(policyJSONBytes)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Generate test keys
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate test signer: %v", err)
	}

	// Create test events of different kinds
	testEvents := []*event.E{
		// Kind 1 (text note) - should be allowed by policy
		createTestEvent(t, signer, "Text note - should sync", 1),
		// Kind 7 (user status) - should be allowed by policy
		createTestEvent(t, signer, "User status - should sync", 7),
		// Kind 42 (channel message) - should be allowed by policy
		createTestEvent(t, signer, "Channel message - should sync", 42),
		// Kind 0 (metadata) - should be denied by policy
		createTestEvent(t, signer, "Metadata - should NOT sync", 0),
		// Kind 3 (follows) - should be denied by policy
		createTestEvent(t, signer, "Follows - should NOT sync", 3),
	}

	t.Logf("Created %d test events", len(testEvents))

	// Publish events to the first relay (non-policy relay)
	firstRelayWS := fmt.Sprintf("ws://127.0.0.1:%d", ports[0])
	client, err := relaytester.NewClient(firstRelayWS)
	if err != nil {
		t.Fatalf("Failed to connect to first relay: %v", err)
	}
	defer client.Close()

	// Publish all events to the first relay
	for i, ev := range testEvents {
		if err := client.Publish(ev); err != nil {
			t.Fatalf("Failed to publish event %d: %v", i, err)
		}

		// Wait for OK response
		accepted, reason, err := client.WaitForOK(ev.ID, 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to get OK response for event %d: %v", i, err)
		}
		if !accepted {
			t.Logf("Event %d rejected: %s (kind: %d)", i, reason, ev.Kind)
		} else {
			t.Logf("Event %d accepted (kind: %d)", i, ev.Kind)
		}
	}

	// Test policy filtering directly
	t.Logf("Testing policy filtering...")

	// Test that the policy correctly allows/denies events based on the whitelist
	// Only kinds 1, 7, and 42 should be allowed
	for i, ev := range testEvents {
		allowed, err := testPolicy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
		if err != nil {
			t.Fatalf("Policy check failed for event %d: %v", i, err)
		}

		expectedAllowed := ev.Kind == 1 || ev.Kind == 7 || ev.Kind == 42
		if allowed != expectedAllowed {
			t.Errorf("Event %d (kind %d): expected allowed=%v, got %v", i, ev.Kind, expectedAllowed, allowed)
		}
	}

	t.Logf("Policy filtering test completed successfully")

	// Note: In a real cluster setup, the sync manager would use this policy
	// to filter events during synchronization between peers. This test demonstrates
	// that the policy correctly identifies which events should be allowed to sync.
}

// testRelay wraps a run.Relay for testing purposes
type testRelay struct {
	*run.Relay
}

// startTestRelays starts multiple test relays with different configurations
func startTestRelays(count int) ([]interface{}, []int, error) {
	relays := make([]interface{}, count)
	ports := make([]int, count)

	for i := 0; i < count; i++ {
		cfg := &config.C{
			AppName:             fmt.Sprintf("ORLY-TEST-%d", i),
			DataDir:             "", // Use temp dir
			Listen:              "127.0.0.1",
			Port:                0, // Random port
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
			PolicyEnabled:       false, // We'll enable it separately for one relay
		}

		// Find available port
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find available port for relay %d: %w", i, err)
		}
		addr := listener.Addr().(*net.TCPAddr)
		cfg.Port = addr.Port
		listener.Close()

		// Set up logging
		lol.SetLogLevel(cfg.LogLevel)

		opts := &run.Options{
			CleanupDataDir: func(b bool) *bool { return &b }(true),
		}

		relay, err := run.Start(cfg, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to start relay %d: %w", i, err)
		}

		relays[i] = &testRelay{Relay: relay}
		ports[i] = cfg.Port
	}

	return relays, ports, nil
}

// waitForTestRelay waits for a relay to be ready by attempting to connect
func waitForTestRelay(url string, timeout time.Duration) error {
	// Extract host:port from ws:// URL
	addr := url
	if len(url) > 5 && url[:5] == "ws://" {
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
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for relay at %s after %d attempts", url, attempts)
}

// createTestEvent creates a test event with proper signing
func createTestEvent(t *testing.T, signer *p8k.Signer, content string, eventKind uint16) *event.E {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = eventKind
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Sign the event
	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign test event: %v", err)
	}

	return ev
}

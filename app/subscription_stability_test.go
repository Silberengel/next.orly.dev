package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"next.orly.dev/pkg/encoders/event"
)

// TestLongRunningSubscriptionStability verifies that subscriptions remain active
// for extended periods and correctly receive real-time events without dropping.
func TestLongRunningSubscriptionStability(t *testing.T) {
	// Create test server
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Start HTTP test server
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(httpServer.URL, "http://", "ws://", 1)

	// Connect WebSocket client
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Subscribe to kind 1 events
	subID := "test-long-running"
	reqMsg := fmt.Sprintf(`["REQ","%s",{"kinds":[1]}]`, subID)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(reqMsg)); err != nil {
		t.Fatalf("Failed to send REQ: %v", err)
	}

	// Read until EOSE
	gotEOSE := false
	for !gotEOSE {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}
		if strings.Contains(string(msg), `"EOSE"`) && strings.Contains(string(msg), subID) {
			gotEOSE = true
			t.Logf("Received EOSE for subscription %s", subID)
		}
	}

	// Set up event counter
	var receivedCount atomic.Int64
	var mu sync.Mutex
	receivedEvents := make(map[string]bool)

	// Start goroutine to read events
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				if strings.Contains(err.Error(), "timeout") {
					continue
				}
				t.Logf("Read error: %v", err)
				return
			}

			// Parse message to check if it's an EVENT for our subscription
			var envelope []interface{}
			if err := json.Unmarshal(msg, &envelope); err != nil {
				continue
			}

			if len(envelope) >= 3 && envelope[0] == "EVENT" && envelope[1] == subID {
				// Extract event ID
				eventMap, ok := envelope[2].(map[string]interface{})
				if !ok {
					continue
				}
				eventID, ok := eventMap["id"].(string)
				if !ok {
					continue
				}

				mu.Lock()
				if !receivedEvents[eventID] {
					receivedEvents[eventID] = true
					receivedCount.Add(1)
					t.Logf("Received event %s (total: %d)", eventID[:8], receivedCount.Load())
				}
				mu.Unlock()
			}
		}
	}()

	// Publish events at regular intervals over 30 seconds
	const numEvents = 30
	const publishInterval = 1 * time.Second

	publishCtx, publishCancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer publishCancel()

	for i := 0; i < numEvents; i++ {
		select {
		case <-publishCtx.Done():
			t.Fatalf("Publish timeout exceeded")
		default:
		}

		// Create test event
		ev := &event.E{
			Kind:      1,
			Content:   []byte(fmt.Sprintf("Test event %d for long-running subscription", i)),
			CreatedAt: uint64(time.Now().Unix()),
		}

		// Save event to database (this will trigger publisher)
		if err := server.D.SaveEvent(context.Background(), ev); err != nil {
			t.Errorf("Failed to save event %d: %v", i, err)
			continue
		}

		t.Logf("Published event %d", i)

		// Wait before next publish
		if i < numEvents-1 {
			time.Sleep(publishInterval)
		}
	}

	// Wait a bit more for all events to be delivered
	time.Sleep(3 * time.Second)

	// Cancel context and wait for reader to finish
	cancel()
	<-readDone

	// Check results
	received := receivedCount.Load()
	t.Logf("Test complete: published %d events, received %d events", numEvents, received)

	// We should receive at least 90% of events (allowing for some timing edge cases)
	minExpected := int64(float64(numEvents) * 0.9)
	if received < minExpected {
		t.Errorf("Subscription stability issue: expected at least %d events, got %d", minExpected, received)
	}

	// Close subscription
	closeMsg := fmt.Sprintf(`["CLOSE","%s"]`, subID)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(closeMsg)); err != nil {
		t.Errorf("Failed to send CLOSE: %v", err)
	}

	t.Logf("Long-running subscription test PASSED: %d/%d events delivered", received, numEvents)
}

// TestMultipleConcurrentSubscriptions verifies that multiple subscriptions
// can coexist on the same connection without interfering with each other.
func TestMultipleConcurrentSubscriptions(t *testing.T) {
	// Create test server
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Start HTTP test server
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := strings.Replace(httpServer.URL, "http://", "ws://", 1)

	// Connect WebSocket client
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Create 3 subscriptions for different kinds
	subscriptions := []struct {
		id   string
		kind int
	}{
		{"sub1", 1},
		{"sub2", 3},
		{"sub3", 7},
	}

	// Subscribe to all
	for _, sub := range subscriptions {
		reqMsg := fmt.Sprintf(`["REQ","%s",{"kinds":[%d]}]`, sub.id, sub.kind)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(reqMsg)); err != nil {
			t.Fatalf("Failed to send REQ for %s: %v", sub.id, err)
		}
	}

	// Read until we get EOSE for all subscriptions
	eoseCount := 0
	for eoseCount < len(subscriptions) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}
		if strings.Contains(string(msg), `"EOSE"`) {
			eoseCount++
			t.Logf("Received EOSE %d/%d", eoseCount, len(subscriptions))
		}
	}

	// Track received events per subscription
	var mu sync.Mutex
	receivedByKind := make(map[int]int)

	// Start reader goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "timeout") {
					continue
				}
				return
			}

			// Parse message
			var envelope []interface{}
			if err := json.Unmarshal(msg, &envelope); err != nil {
				continue
			}

			if len(envelope) >= 3 && envelope[0] == "EVENT" {
				eventMap, ok := envelope[2].(map[string]interface{})
				if !ok {
					continue
				}
				kindFloat, ok := eventMap["kind"].(float64)
				if !ok {
					continue
				}
				kind := int(kindFloat)

				mu.Lock()
				receivedByKind[kind]++
				t.Logf("Received event for kind %d (count: %d)", kind, receivedByKind[kind])
				mu.Unlock()
			}
		}
	}()

	// Publish events for each kind
	for _, sub := range subscriptions {
		for i := 0; i < 5; i++ {
			ev := &event.E{
				Kind:      uint16(sub.kind),
				Content:   []byte(fmt.Sprintf("Test for kind %d event %d", sub.kind, i)),
				CreatedAt: uint64(time.Now().Unix()),
			}

			if err := server.D.SaveEvent(context.Background(), ev); err != nil {
				t.Errorf("Failed to save event: %v", err)
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	// Wait for events to be delivered
	time.Sleep(2 * time.Second)

	// Cancel and cleanup
	cancel()
	<-readDone

	// Verify each subscription received its events
	mu.Lock()
	defer mu.Unlock()

	for _, sub := range subscriptions {
		count := receivedByKind[sub.kind]
		if count < 4 { // Allow for some timing issues, expect at least 4/5
			t.Errorf("Subscription %s (kind %d) only received %d/5 events", sub.id, sub.kind, count)
		}
	}

	t.Logf("Multiple concurrent subscriptions test PASSED")
}

// setupTestServer creates a test relay server for subscription testing
func setupTestServer(t *testing.T) (*Server, func()) {
	// This is a simplified setup - adapt based on your actual test setup
	// You may need to create a proper test database, etc.
	t.Skip("Implement setupTestServer based on your existing test infrastructure")
	return nil, func() {}
}

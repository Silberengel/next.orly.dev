# Relay Testing Guide

This guide explains how to use ORLY's comprehensive testing infrastructure for protocol validation, especially when developing features that require multiple relays to test the Nostr protocol correctly.

## Overview

ORLY provides multiple testing tools and scripts designed for different testing scenarios:

- **relay-tester**: Protocol compliance testing against NIP specifications
- **Benchmark suite**: Performance testing across multiple relay implementations
- **Policy testing**: Custom policy validation
- **Integration scripts**: Multi-relay testing scenarios

## Testing Tools Overview

### relay-tester

The primary tool for testing Nostr protocol compliance:

```bash
# Basic usage
relay-tester -url ws://127.0.0.1:3334

# Test with different configurations
relay-tester -url wss://relay.example.com -v -json
```

**Key Features:**
- Tests all major NIP-01, NIP-09, NIP-42 features
- Validates event publishing, querying, and subscription handling
- Checks JSON compliance and signature validation
- Provides both human-readable and JSON output

### Benchmark Suite

Performance testing across multiple relay implementations:

```bash
# Setup external relays
cd cmd/benchmark
./setup-external-relays.sh

# Run benchmark suite
docker-compose up --build
```

**Key Features:**
- Compares ORLY against other relay implementations
- Tests throughput, latency, and reliability
- Provides detailed performance metrics
- Generates comparison reports

### Policy Testing

Custom policy validation tools:

```bash
# Test policy with sample events
./scripts/run-policy-test.sh

# Test policy filter integration
./scripts/run-policy-filter-test.sh
```

## Multi-Relay Testing Scenarios

### Why Multiple Relays?

Many Nostr protocol features require testing with multiple relays:

- **Event replication** between relays
- **Cross-relay subscriptions** and queries
- **Relay discovery** and connection management
- **Protocol interoperability** between different implementations
- **Distributed features** like directory consensus

### Testing Infrastructure

ORLY provides several ways to run multiple relays for testing:

#### 1. Local Multi-Relay Setup

Run multiple instances on different ports:

```bash
# Terminal 1: Relay 1 on port 3334
ORLY_PORT=3334 ./orly &

# Terminal 2: Relay 2 on port 3335
ORLY_PORT=3335 ./orly &

# Terminal 3: Relay 3 on port 3336
ORLY_PORT=3336 ./orly &
```

#### 2. Docker-based Multi-Relay

Use Docker for isolated relay instances:

```bash
# Run multiple relays with Docker
docker run -d -p 3334:3334 -e ORLY_PORT=3334 orly:latest
docker run -d -p 3335:3334 -e ORLY_PORT=3334 orly:latest
docker run -d -p 3336:3334 -e ORLY_PORT=3334 orly:latest
```

#### 3. Benchmark Suite Multi-Relay

The benchmark suite automatically sets up multiple relays:

```bash
cd cmd/benchmark
./setup-external-relays.sh
docker-compose up next-orly khatru-sqlite strfry
```

## Developing Features Requiring Multiple Relays

### 1. Event Replication Testing

Test how events propagate between relays:

```go
// Example test for event replication
func TestEventReplication(t *testing.T) {
    // Start two relays
    relay1 := startTestRelay(t, 3334)
    defer relay1.Stop()

    relay2 := startTestRelay(t, 3335)
    defer relay2.Stop()

    // Connect clients to both relays
    client1 := connectToRelay(t, "ws://127.0.0.1:3334")
    client2 := connectToRelay(t, "ws://127.0.0.1:3335")

    // Publish event to relay1
    event := createTestEvent(t)
    ok := client1.Publish(event)
    assert.True(t, ok)

    // Wait for replication/propagation
    time.Sleep(100 * time.Millisecond)

    // Query relay2 for the event
    events := client2.Query(filterForEvent(event.ID))
    assert.Len(t, events, 1)
    assert.Equal(t, event.ID, events[0].ID)
}
```

### 2. Cross-Relay Subscriptions

Test subscriptions that span multiple relays:

```go
func TestCrossRelaySubscriptions(t *testing.T) {
    // Setup multiple relays
    relays := setupMultipleRelays(t, 3)
    defer stopRelays(t, relays)

    clients := connectToRelays(t, relays)

    // Subscribe to same filter on all relays
    filter := Filter{Kinds: []int{1}, Limit: 10}

    for _, client := range clients {
        client.Subscribe(filter)
    }

    // Publish events to different relays
    for i, client := range clients {
        event := createTestEvent(t)
        event.Content = fmt.Sprintf("Event from relay %d", i)
        client.Publish(event)
    }

    // Verify events appear on all relays (if replication is enabled)
    time.Sleep(200 * time.Millisecond)

    for _, client := range clients {
        events := client.GetReceivedEvents()
        assert.GreaterOrEqual(t, len(events), 3) // At least the events from all relays
    }
}
```

### 3. Relay Discovery Testing

Test relay list events and dynamic relay discovery:

```go
func TestRelayDiscovery(t *testing.T) {
    relay1 := startTestRelay(t, 3334)
    relay2 := startTestRelay(t, 3335)
    defer relay1.Stop()
    defer relay2.Stop()

    client := connectToRelay(t, "ws://127.0.0.1:3334")

    // Publish relay list event (kind 10002)
    relayList := createRelayListEvent(t, []string{
        "wss://relay1.example.com",
        "wss://relay2.example.com",
    })
    client.Publish(relayList)

    // Test that relay discovery works
    discovered := client.QueryRelays()
    assert.Contains(t, discovered, "wss://relay1.example.com")
    assert.Contains(t, discovered, "wss://relay2.example.com")
}
```

## Testing Scripts and Automation

### Automated Multi-Relay Testing

Use the provided scripts for automated testing:

#### 1. relaytester-test.sh

Tests relay with protocol compliance:

```bash
# Test single relay
./scripts/relaytester-test.sh

# Test with policy enabled
ORLY_POLICY_ENABLED=true ./scripts/relaytester-test.sh

# Test with ACL enabled
ORLY_ACL_MODE=follows ./scripts/relaytester-test.sh
```

#### 2. test.sh (Full Test Suite)

Runs all tests including multi-component scenarios:

```bash
# Run complete test suite
./scripts/test.sh

# Run specific package tests
go test ./pkg/sync/... # Test synchronization features
go test ./pkg/protocol/... # Test protocol implementations
```

#### 3. runtests.sh (Performance Tests)

```bash
# Run performance benchmarks
./scripts/runtests.sh
```

### Custom Testing Scripts

Create custom scripts for specific multi-relay scenarios:

```bash
#!/bin/bash
# test-multi-relay-replication.sh

# Start multiple relays
echo "Starting relays..."
ORLY_PORT=3334 ./orly &
RELAY1_PID=$!

ORLY_PORT=3335 ./orly &
RELAY2_PID=$!

ORLY_PORT=3336 ./orly &
RELAY3_PID=$!

# Wait for startup
sleep 2

# Run replication tests
echo "Running replication tests..."
go test -v ./pkg/sync -run TestReplication

# Run protocol tests
echo "Running protocol tests..."
relay-tester -url ws://127.0.0.1:3334 -json > relay1-results.json
relay-tester -url ws://127.0.0.1:3335 -json > relay2-results.json
relay-tester -url ws://127.0.0.1:3336 -json > relay3-results.json

# Cleanup
kill $RELAY1_PID $RELAY2_PID $RELAY3_PID

echo "Tests completed"
```

## Testing Distributed Features

### Directory Consensus Testing

Test NIP-XX directory consensus protocol:

```go
func TestDirectoryConsensus(t *testing.T) {
    // Setup multiple relays with directory support
    relays := setupDirectoryRelays(t, 5)
    defer stopRelays(t, relays)

    clients := connectToRelays(t, relays)

    // Create trust acts between relays
    for i, client := range clients {
        trustAct := createTrustAct(t, client.Pubkey, relays[(i+1)%len(relays)].Pubkey, 80)
        client.Publish(trustAct)
    }

    // Wait for consensus
    time.Sleep(1 * time.Second)

    // Verify trust relationships
    for _, client := range clients {
        trustGraph := client.QueryTrustGraph()
        // Verify expected trust relationships exist
        assert.True(t, len(trustGraph.GetAllTrustActs()) > 0)
    }
}
```

### Sync Protocol Testing

Test event synchronization between relays:

```go
func TestRelaySynchronization(t *testing.T) {
    relay1 := startTestRelay(t, 3334)
    relay2 := startTestRelay(t, 3335)
    defer relay1.Stop()
    defer relay2.Stop()

    // Enable sync between relays
    configureSync(t, relay1, relay2)

    client1 := connectToRelay(t, "ws://127.0.0.1:3334")
    client2 := connectToRelay(t, "ws://127.0.0.1:3335")

    // Publish events to relay1
    events := createTestEvents(t, 100)
    for _, event := range events {
        client1.Publish(event)
    }

    // Wait for sync
    waitForSync(t, relay1, relay2)

    // Verify events on relay2
    syncedEvents := client2.Query(Filter{Kinds: []int{1}, Limit: 200})
    assert.Len(t, syncedEvents, 100)
}
```

## Performance Testing with Multiple Relays

### Load Testing

Test performance under load with multiple relays:

```bash
# Start multiple relays
for port in 3334 3335 3336; do
    ORLY_PORT=$port ./orly &
    echo $! >> relay_pids.txt
done

# Run load tests against each relay
for port in 3334 3335 3336; do
    echo "Testing relay on port $port"
    relay-tester -url ws://127.0.0.1:$port -json > results_$port.json &
done

wait

# Analyze results
# Combine and compare performance across relays
```

### Benchmarking Comparisons

Use the benchmark suite for comparative testing:

```bash
cd cmd/benchmark

# Setup all relay types
./setup-external-relays.sh

# Run benchmarks comparing multiple implementations
docker-compose up --build

# Results in reports/run_YYYYMMDD_HHMMSS/
cat reports/run_*/aggregate_report.txt
```

## Debugging Multi-Relay Issues

### Logging

Enable detailed logging for multi-relay debugging:

```bash
# Enable debug logging
export ORLY_LOG_LEVEL=debug
export ORLY_LOG_TO_STDOUT=true

# Start relays with logging
ORLY_PORT=3334 ./orly 2>&1 | tee relay1.log &
ORLY_PORT=3335 ./orly 2>&1 | tee relay2.log &
```

### Connection Monitoring

Monitor WebSocket connections between relays:

```bash
# Monitor network connections
netstat -tlnp | grep :3334
ss -tlnp | grep :3334

# Monitor relay logs
tail -f relay1.log | grep -E "(connect|disconnect|sync)"
```

### Event Tracing

Trace events across multiple relays:

```go
func traceEventPropagation(t *testing.T, eventID string, relays []*TestRelay) {
    for _, relay := range relays {
        client := connectToRelay(t, relay.URL)
        events := client.Query(Filter{IDs: []string{eventID}})
        if len(events) > 0 {
            t.Logf("Event %s found on relay %s", eventID, relay.URL)
        } else {
            t.Logf("Event %s NOT found on relay %s", eventID, relay.URL)
        }
    }
}
```

## CI/CD Integration

### GitHub Actions Example

```yaml
# .github/workflows/multi-relay-tests.yml
name: Multi-Relay Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Setup Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Install dependencies
      run: |
        sudo apt-get update
        sudo apt-get install -y docker.io docker-compose

    - name: Run single relay tests
      run: ./scripts/relaytester-test.sh

    - name: Run multi-relay integration tests
      run: |
        # Start multiple relays
        ORLY_PORT=3334 ./orly &
        ORLY_PORT=3335 ./orly &
        ORLY_PORT=3336 ./orly &
        sleep 3

        # Run integration tests
        go test -v ./pkg/sync -run TestMultiRelay

    - name: Run benchmark suite
      run: |
        cd cmd/benchmark
        ./setup-external-relays.sh
        docker-compose up --build --abort-on-container-exit

    - name: Upload test results
      uses: actions/upload-artifact@v3
      with:
        name: test-results
        path: |
          cmd/benchmark/reports/
          *-results.json
```

## Best Practices

### 1. Test Isolation

- Use separate databases for each test relay
- Clean up resources after tests
- Use unique ports to avoid conflicts

### 2. Timing Considerations

- Allow time for event propagation between relays
- Use exponential backoff for retry logic
- Account for network latency in assertions

### 3. Resource Management

- Limit concurrent relays in CI/CD
- Clean up Docker containers and processes
- Monitor resource usage during tests

### 4. Error Handling

- Test both success and failure scenarios
- Verify error propagation across relays
- Test network failure scenarios

### 5. Performance Monitoring

- Measure latency between relays
- Track memory and CPU usage
- Monitor WebSocket connection stability

## Troubleshooting Common Issues

### Connection Failures

```bash
# Check if relays are listening
netstat -tlnp | grep :3334

# Test WebSocket connection manually
websocat ws://127.0.0.1:3334
```

### Event Propagation Delays

```bash
# Increase wait times in tests
time.Sleep(500 * time.Millisecond)

// Or use polling
func waitForEvent(t *testing.T, client *Client, eventID string) {
    timeout := time.After(5 * time.Second)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-timeout:
            t.Fatalf("Event %s not found within timeout", eventID)
        case <-ticker.C:
            events := client.Query(Filter{IDs: []string{eventID}})
            if len(events) > 0 {
                return
            }
        }
    }
}
```

### Race Conditions

```go
// Use proper synchronization
var mu sync.Mutex
eventCount := 0

// In test goroutines
mu.Lock()
eventCount++
mu.Unlock()
```

### Resource Exhaustion

```bash
# Limit relay instances in tests
const maxRelays = 3

func setupLimitedRelays(t *testing.T, count int) []*TestRelay {
    if count > maxRelays {
        t.Skipf("Skipping test requiring %d relays (max %d)", count, maxRelays)
    }
    // Setup relays...
}
```

## Contributing

When adding new features that require multi-relay testing:

1. Add unit tests for single-relay scenarios
2. Add integration tests for multi-relay scenarios
3. Update this guide with new testing patterns
4. Ensure tests work in CI/CD environment
5. Document any new testing tools or scripts

## Related Documentation

- [POLICY_USAGE_GUIDE.md](POLICY_USAGE_GUIDE.md) - Policy system testing
- [README.md](../../README.md) - Main project documentation
- [cmd/benchmark/README.md](../../cmd/benchmark/README.md) - Benchmark suite
- [cmd/relay-tester/README.md](../../cmd/relay-tester/README.md) - Protocol testing

This guide provides the foundation for testing complex Nostr protocol features that require multiple relay coordination. The testing infrastructure is designed to be extensible and support various testing scenarios while maintaining reliability and performance.






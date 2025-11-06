# Subscription Stability Testing Guide

This guide explains how to test the subscription stability fixes.

## Quick Test

### 1. Start the Relay

```bash
# Build the relay with fixes
go build -o orly

# Start the relay
./orly
```

### 2. Run the Subscription Test

In another terminal:

```bash
# Run the built-in test tool
./subscription-test -url ws://localhost:3334 -duration 60 -kind 1 -v

# Or use the helper script
./scripts/test-subscriptions.sh
```

### 3. Publish Events (While Test is Running)

The subscription test will wait for events. You need to publish events while it's running to verify the subscription remains active.

**Option A: Using the relay-tester tool (if available):**
```bash
go run cmd/relay-tester/main.go -url ws://localhost:3334
```

**Option B: Using your client application:**
Publish events to the relay through your normal client workflow.

**Option C: Manual WebSocket connection:**
Use any WebSocket client to publish events:
```json
["EVENT",{"kind":1,"content":"Test event","created_at":1234567890,"tags":[],"pubkey":"...","id":"...","sig":"..."}]
```

## What to Look For

### ✅ Success Indicators

1. **Subscription stays active:**
   - Test receives EOSE immediately
   - Events are delivered throughout the entire test duration
   - No "subscription may have dropped" warnings

2. **Event delivery:**
   - All published events are received by the subscription
   - Events arrive within 1-2 seconds of publishing
   - No delivery timeouts in relay logs

3. **Clean shutdown:**
   - Test can be interrupted with Ctrl+C
   - Subscription closes cleanly
   - No error messages in relay logs

### ❌ Failure Indicators

1. **Subscription drops:**
   - Events stop being received after ~30-60 seconds
   - Warning: "No events received for Xs"
   - Relay logs show timeout errors

2. **Event delivery failures:**
   - Events are published but not received
   - Relay logs show "delivery TIMEOUT" messages
   - Subscription is removed from publisher

3. **Resource leaks:**
   - Memory usage grows over time
   - Goroutine count increases continuously
   - Connection not cleaned up properly

## Test Scenarios

### 1. Basic Long-Running Test

**Duration:** 60 seconds
**Event Rate:** 1 event every 2-5 seconds
**Expected:** All events received, subscription stays active

```bash
./subscription-test -url ws://localhost:3334 -duration 60
```

### 2. Extended Duration Test

**Duration:** 300 seconds (5 minutes)
**Event Rate:** 1 event every 10 seconds
**Expected:** All events received throughout 5 minutes

```bash
./subscription-test -url ws://localhost:3334 -duration 300
```

### 3. Multiple Subscriptions

Run multiple test instances simultaneously:

```bash
# Terminal 1
./subscription-test -url ws://localhost:3334 -duration 120 -kind 1 -sub sub1

# Terminal 2
./subscription-test -url ws://localhost:3334 -duration 120 -kind 1 -sub sub2

# Terminal 3
./subscription-test -url ws://localhost:3334 -duration 120 -kind 1 -sub sub3
```

**Expected:** All subscriptions receive events independently

### 4. Idle Subscription Test

**Duration:** 120 seconds
**Event Rate:** Publish events only at start and end
**Expected:** Subscription remains active even during long idle period

```bash
# Start test
./subscription-test -url ws://localhost:3334 -duration 120

# Publish 1-2 events immediately
# Wait 100 seconds (subscription should stay alive)
# Publish 1-2 more events
# Verify test receives the late events
```

## Debugging

### Enable Verbose Logging

```bash
# Relay
export ORLY_LOG_LEVEL=debug
./orly

# Test tool
./subscription-test -url ws://localhost:3334 -duration 60 -v
```

### Check Relay Logs

Look for these log patterns:

**Good (working subscription):**
```
subscription test-123456 created and goroutine launched for 127.0.0.1
delivered real-time event abc123... to subscription test-123456 @ 127.0.0.1
subscription delivery QUEUED: event=abc123... to=127.0.0.1
```

**Bad (subscription issues):**
```
subscription delivery TIMEOUT: event=abc123...
removing failed subscriber connection
subscription goroutine exiting unexpectedly
```

### Monitor Resource Usage

```bash
# Watch memory usage
watch -n 1 'ps aux | grep orly'

# Check goroutine count (requires pprof enabled)
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

## Expected Performance

With the fixes applied:

- **Subscription lifetime:** Unlimited (hours/days)
- **Event delivery latency:** < 100ms
- **Max concurrent subscriptions:** Thousands per relay
- **Memory per subscription:** ~10KB (goroutine + buffers)
- **CPU overhead:** Minimal (event-driven)

## Automated Tests

Run the Go test suite:

```bash
# Run all tests
./scripts/test.sh

# Run subscription tests only (once implemented)
go test -v -run TestLongRunningSubscription ./app
go test -v -run TestMultipleConcurrentSubscriptions ./app
```

## Common Issues

### Issue: "Failed to connect"

**Cause:** Relay not running or wrong URL
**Solution:**
```bash
# Check relay is running
ps aux | grep orly

# Verify port
netstat -tlnp | grep 3334
```

### Issue: "No events received"

**Cause:** No events being published
**Solution:** Publish test events while test is running (see section 3 above)

### Issue: "Subscription CLOSED by relay"

**Cause:** Filter policy or ACL rejecting subscription
**Solution:** Check relay configuration and ACL settings

### Issue: Test hangs at EOSE

**Cause:** Relay not sending EOSE
**Solution:** Check relay logs for query errors

## Manual Testing with Raw WebSocket

If you prefer manual testing, you can use any WebSocket client:

```bash
# Install wscat (Node.js based, no glibc issues)
npm install -g wscat

# Connect and subscribe
wscat -c ws://localhost:3334
> ["REQ","manual-test",{"kinds":[1]}]

# Wait for EOSE
< ["EOSE","manual-test"]

# Events should arrive as they're published
< ["EVENT","manual-test",{"id":"...","kind":1,...}]
```

## Comparison: Before vs After Fixes

### Before (Broken)

```
$ ./subscription-test -duration 60
✓ Connected
✓ Received EOSE
[EVENT #1] id=abc123... kind=1
[EVENT #2] id=def456... kind=1
...
[EVENT #30] id=xyz789... kind=1
⚠ Warning: No events received for 35s - subscription may have dropped
Test complete: 30 events received (expected 60)
```

### After (Fixed)

```
$ ./subscription-test -duration 60
✓ Connected
✓ Received EOSE
[EVENT #1] id=abc123... kind=1
[EVENT #2] id=def456... kind=1
...
[EVENT #60] id=xyz789... kind=1
✓ TEST PASSED - Subscription remained stable
Test complete: 60 events received
```

## Reporting Issues

If subscriptions still drop after the fixes, please report with:

1. Relay logs (with `ORLY_LOG_LEVEL=debug`)
2. Test output
3. Steps to reproduce
4. Relay configuration
5. Event publishing method

## Summary

The subscription stability fixes ensure:

✅ Subscriptions remain active indefinitely
✅ All events are delivered without timeouts
✅ Clean resource management (no leaks)
✅ Multiple concurrent subscriptions work correctly
✅ Idle subscriptions don't timeout

Follow the test scenarios above to verify these improvements in your deployment.

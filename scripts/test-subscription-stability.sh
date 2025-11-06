#!/bin/bash
# Test script for verifying subscription stability fixes

set -e

RELAY_URL="${RELAY_URL:-ws://localhost:3334}"
TEST_DURATION="${TEST_DURATION:-60}"  # seconds
EVENT_INTERVAL="${EVENT_INTERVAL:-2}"  # seconds between events

echo "==================================="
echo "Subscription Stability Test"
echo "==================================="
echo "Relay URL: $RELAY_URL"
echo "Test duration: ${TEST_DURATION}s"
echo "Event interval: ${EVENT_INTERVAL}s"
echo ""

# Check if websocat is installed
if ! command -v websocat &> /dev/null; then
    echo "ERROR: websocat is not installed"
    echo "Install with: cargo install websocat"
    exit 1
fi

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "ERROR: jq is not installed"
    echo "Install with: sudo apt install jq"
    exit 1
fi

# Temporary files for communication
FIFO_IN=$(mktemp -u)
FIFO_OUT=$(mktemp -u)
mkfifo "$FIFO_IN"
mkfifo "$FIFO_OUT"

# Cleanup on exit
cleanup() {
    echo ""
    echo "Cleaning up..."
    rm -f "$FIFO_IN" "$FIFO_OUT"
    kill $WS_PID 2>/dev/null || true
    kill $READER_PID 2>/dev/null || true
    kill $PUBLISHER_PID 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "Step 1: Connecting to relay..."

# Start WebSocket connection
websocat "$RELAY_URL" < "$FIFO_IN" > "$FIFO_OUT" &
WS_PID=$!

# Wait for connection
sleep 1

if ! kill -0 $WS_PID 2>/dev/null; then
    echo "ERROR: Failed to connect to relay at $RELAY_URL"
    exit 1
fi

echo "✓ Connected to relay"
echo ""

echo "Step 2: Creating subscription..."

# Send REQ message
SUB_ID="stability-test-$(date +%s)"
REQ_MSG='["REQ","'$SUB_ID'",{"kinds":[1]}]'
echo "$REQ_MSG" > "$FIFO_IN"

echo "✓ Sent REQ for subscription: $SUB_ID"
echo ""

# Variables for tracking
RECEIVED_COUNT=0
PUBLISHED_COUNT=0
EOSE_RECEIVED=0

echo "Step 3: Waiting for EOSE..."

# Read messages and count events
(
    while IFS= read -r line; do
        echo "[RECV] $line"

        # Check for EOSE
        if echo "$line" | jq -e '. | select(.[0] == "EOSE" and .[1] == "'$SUB_ID'")' > /dev/null 2>&1; then
            EOSE_RECEIVED=1
            echo "✓ Received EOSE"
            break
        fi
    done < "$FIFO_OUT"
) &
READER_PID=$!

# Wait up to 10 seconds for EOSE
for i in {1..10}; do
    if [ $EOSE_RECEIVED -eq 1 ]; then
        break
    fi
    sleep 1
done

echo ""
echo "Step 4: Starting long-running test..."
echo "Publishing events every ${EVENT_INTERVAL}s for ${TEST_DURATION}s..."
echo ""

# Start event counter
(
    while IFS= read -r line; do
        # Count EVENT messages for our subscription
        if echo "$line" | jq -e '. | select(.[0] == "EVENT" and .[1] == "'$SUB_ID'")' > /dev/null 2>&1; then
            RECEIVED_COUNT=$((RECEIVED_COUNT + 1))
            EVENT_ID=$(echo "$line" | jq -r '.[2].id' 2>/dev/null || echo "unknown")
            echo "[$(date +%H:%M:%S)] EVENT received #$RECEIVED_COUNT (id: ${EVENT_ID:0:8}...)"
        fi
    done < "$FIFO_OUT"
) &
READER_PID=$!

# Publish events
START_TIME=$(date +%s)
END_TIME=$((START_TIME + TEST_DURATION))

while [ $(date +%s) -lt $END_TIME ]; do
    PUBLISHED_COUNT=$((PUBLISHED_COUNT + 1))

    # Create and publish event (you'll need to implement this part)
    # This is a placeholder - replace with actual event publishing
    EVENT_JSON='["EVENT",{"kind":1,"content":"Test event '$PUBLISHED_COUNT' for stability test","created_at":'$(date +%s)',"tags":[],"pubkey":"0000000000000000000000000000000000000000000000000000000000000000","id":"0000000000000000000000000000000000000000000000000000000000000000","sig":"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}]'

    echo "[$(date +%H:%M:%S)] Publishing event #$PUBLISHED_COUNT"

    # Sleep before next event
    sleep "$EVENT_INTERVAL"
done

echo ""
echo "==================================="
echo "Test Complete"
echo "==================================="
echo "Duration: ${TEST_DURATION}s"
echo "Events published: $PUBLISHED_COUNT"
echo "Events received: $RECEIVED_COUNT"
echo ""

# Calculate success rate
if [ $PUBLISHED_COUNT -gt 0 ]; then
    SUCCESS_RATE=$((RECEIVED_COUNT * 100 / PUBLISHED_COUNT))
    echo "Success rate: ${SUCCESS_RATE}%"
    echo ""

    if [ $SUCCESS_RATE -ge 90 ]; then
        echo "✓ TEST PASSED - Subscription remained stable"
        exit 0
    else
        echo "✗ TEST FAILED - Subscription dropped events"
        exit 1
    fi
else
    echo "✗ TEST FAILED - No events published"
    exit 1
fi

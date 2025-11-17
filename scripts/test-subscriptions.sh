#!/bin/bash
# Simple subscription stability test script

set -e

RELAY_URL="${RELAY_URL:-ws://localhost:3334}"
DURATION="${DURATION:-60}"
KIND="${KIND:-1}"

echo "==================================="
echo "Subscription Stability Test"
echo "==================================="
echo ""
echo "This tool tests whether subscriptions remain stable over time."
echo ""
echo "Configuration:"
echo "  Relay URL: $RELAY_URL"
echo "  Duration: ${DURATION}s"
echo "  Event kind: $KIND"
echo ""
echo "To test properly, you should:"
echo "  1. Start this test"
echo "  2. In another terminal, publish events to the relay"
echo "  3. Verify events are received throughout the test duration"
echo ""

# Check if the test tool is built
if [ ! -f "./subscription-test" ]; then
    echo "Building subscription-test tool..."
    go build -o subscription-test ./cmd/subscription-test
    echo "✓ Built"
    echo ""
fi

# Run the test
echo "Starting test..."
echo ""

./subscription-test -url "$RELAY_URL" -duration "$DURATION" -kind "$KIND" -v

exit $?

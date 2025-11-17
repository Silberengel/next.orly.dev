#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== ORLY Dgraph Integration Test Suite ==="
echo ""

# Check if dgraph is running
echo "Checking for dgraph server..."
DGRAPH_URL="${ORLY_DGRAPH_URL:-localhost:9080}"

if ! timeout 2 bash -c "echo > /dev/tcp/${DGRAPH_URL%:*}/${DGRAPH_URL#*:}" 2>/dev/null; then
    echo "❌ Dgraph server not available at $DGRAPH_URL"
    echo ""
    echo "To start dgraph using docker-compose:"
    echo "  cd $SCRIPT_DIR && docker-compose -f dgraph-docker-compose.yml up -d"
    echo ""
    echo "Or using docker directly:"
    echo "  docker run -d -p 8080:8080 -p 9080:9080 -p 8000:8000 --name dgraph-orly dgraph/standalone:latest"
    echo ""
    exit 1
fi

echo "✅ Dgraph server is running at $DGRAPH_URL"
echo ""

# Run dgraph tests
echo "Running dgraph package tests..."
cd "$PROJECT_ROOT"
CGO_ENABLED=0 go test -v -timeout 10m ./pkg/dgraph/... || {
    echo "❌ Dgraph tests failed"
    exit 1
}

echo ""
echo "✅ All dgraph tests passed!"
echo ""

# Optional: Run relay-tester if requested
if [ "$1" == "--relay-tester" ]; then
    echo "Starting ORLY with dgraph backend..."
    export ORLY_DB_TYPE=dgraph
    export ORLY_DGRAPH_URL="$DGRAPH_URL"
    export ORLY_LOG_LEVEL=info
    export ORLY_PORT=3334

    # Kill any existing ORLY instance
    pkill -f "./orly" || true
    sleep 1

    # Start ORLY in background
    ./orly &
    ORLY_PID=$!

    # Wait for ORLY to start
    echo "Waiting for ORLY to start..."
    for i in {1..30}; do
        if curl -s http://localhost:3334 > /dev/null 2>&1; then
            echo "✅ ORLY started successfully"
            break
        fi
        sleep 1
        if [ $i -eq 30 ]; then
            echo "❌ ORLY failed to start"
            kill $ORLY_PID 2>/dev/null || true
            exit 1
        fi
    done

    echo ""
    echo "Running relay-tester against dgraph backend..."
    go run cmd/relay-tester/main.go -url ws://localhost:3334 || {
        echo "❌ Relay-tester failed"
        kill $ORLY_PID 2>/dev/null || true
        exit 1
    }

    # Clean up
    kill $ORLY_PID 2>/dev/null || true

    echo ""
    echo "✅ Relay-tester passed!"
fi

echo ""
echo "=== All tests completed successfully! ==="

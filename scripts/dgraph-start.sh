#!/bin/bash
# Quick script to start dgraph for testing

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Starting dgraph server for ORLY testing..."
cd "$SCRIPT_DIR"

# Check if already running
if docker ps | grep -q dgraph-orly-test; then
    echo "✅ Dgraph is already running"
    echo ""
    echo "Dgraph endpoints:"
    echo "  gRPC:    localhost:9080"
    echo "  HTTP:    http://localhost:8080"
    echo "  Ratel UI: http://localhost:8000"
    exit 0
fi

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Start using docker compose
$DOCKER_COMPOSE -f dgraph-docker-compose.yml up -d

echo ""
echo "Waiting for dgraph to be healthy..."
for i in {1..30}; do
    if docker exec dgraph-orly-test curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        echo "✅ Dgraph is healthy and ready"
        echo ""
        echo "Dgraph endpoints:"
        echo "  gRPC:    localhost:9080"
        echo "  HTTP:    http://localhost:8080"
        echo "  Ratel UI: http://localhost:8000"
        echo ""
        echo "To stop: $DOCKER_COMPOSE -f dgraph-docker-compose.yml down"
        echo "To view logs: docker logs dgraph-orly-test -f"
        exit 0
    fi
    sleep 1
done

echo "❌ Dgraph failed to become healthy"
docker logs dgraph-orly-test
exit 1

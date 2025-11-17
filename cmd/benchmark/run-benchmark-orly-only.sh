#!/bin/bash

# Run benchmark for ORLY only (no other relays)

set -e

cd "$(dirname "$0")"

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Clean old data directories (may be owned by root from Docker)
if [ -d "data" ]; then
    echo "Cleaning old data directories..."
    if ! rm -rf data/ 2>/dev/null; then
        echo ""
        echo "ERROR: Cannot remove data directories due to permission issues."
        echo "Please run: sudo rm -rf data/"
        echo "Then run this script again."
        exit 1
    fi
fi

# Create fresh data directories with correct permissions
echo "Preparing data directories..."
mkdir -p data/next-orly
chmod 777 data/next-orly

echo "Building ORLY container..."
$DOCKER_COMPOSE build next-orly

echo "Starting ORLY relay..."
echo ""

# Start only next-orly and benchmark-runner
$DOCKER_COMPOSE up next-orly -d

# Wait for ORLY to be healthy
echo "Waiting for ORLY to be healthy..."
for i in {1..30}; do
    if curl -sf http://localhost:8001/ > /dev/null 2>&1; then
        echo "ORLY is ready!"
        break
    fi
    sleep 2
    if [ $i -eq 30 ]; then
        echo "ERROR: ORLY failed to become healthy"
        $DOCKER_COMPOSE logs next-orly
        exit 1
    fi
done

# Run benchmark against ORLY
echo ""
echo "Running benchmark against ORLY..."
echo "Target: http://localhost:8001"
echo ""

# Run the benchmark binary directly against the running ORLY instance
docker run --rm --network benchmark_benchmark-net \
    -e BENCHMARK_TARGETS=next-orly:8080 \
    -e BENCHMARK_EVENTS=10000 \
    -e BENCHMARK_WORKERS=24 \
    -e BENCHMARK_DURATION=20s \
    -v "$(pwd)/reports:/reports" \
    benchmark-benchmark-runner \
    /app/benchmark-runner --output-dir=/reports

echo ""
echo "Benchmark complete!"
echo "Stopping ORLY..."
$DOCKER_COMPOSE down

echo ""
echo "Results saved to ./reports/"
echo "Check the latest run_* directory for detailed results."

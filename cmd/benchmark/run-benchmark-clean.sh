#!/bin/bash

# Wrapper script that cleans data directories with sudo before running benchmark
# Use this if you encounter permission errors with run-benchmark.sh

set -e

cd "$(dirname "$0")"

# Stop any running containers first
echo "Stopping any running benchmark containers..."
if docker compose version &> /dev/null 2>&1; then
    docker compose down -v 2>&1 | grep -v "warning" || true
else
    docker-compose down -v 2>&1 | grep -v "warning" || true
fi

# Clean data directories with sudo
if [ -d "data" ]; then
    echo "Cleaning data directories (requires sudo)..."
    sudo rm -rf data/
fi

# Now run the normal benchmark script
exec ./run-benchmark.sh

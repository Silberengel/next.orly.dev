#!/bin/bash

# Run benchmark with profiling on ORLY only

set -e

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Clean up old data and profiles (may need sudo for Docker-created files)
echo "Cleaning old data and profiles..."
if [ -d "data/next-orly" ]; then
    if ! rm -rf data/next-orly/* 2>/dev/null; then
        echo "Need elevated permissions to clean data directories..."
        sudo rm -rf data/next-orly/*
    fi
fi
rm -rf profiles/* 2>/dev/null || sudo rm -rf profiles/* 2>/dev/null || true
mkdir -p data/next-orly profiles
chmod 777 data/next-orly 2>/dev/null || true

echo "Starting profiled benchmark (ORLY only)..."
echo "- 50,000 events"
echo "- 24 workers"
echo "- 90 second warmup delay"
echo "- CPU profiling enabled"
echo "- pprof HTTP on port 6060"
echo ""

# Run docker compose with profile config
$DOCKER_COMPOSE -f docker-compose.profile.yml up \
  --exit-code-from benchmark-runner \
  --abort-on-container-exit

echo ""
echo "Benchmark complete. Profiles saved to ./profiles/"
echo "Results saved to ./reports/"

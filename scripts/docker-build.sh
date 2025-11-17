#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Building ORLY Docker Images ==="
echo ""

# Change to project root
cd "$PROJECT_ROOT"

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

# Build ORLY image
echo "Building ORLY relay image..."
docker build -t orly:latest -f Dockerfile .
echo "✅ ORLY image built successfully"
echo ""

# Build relay-tester image (optional)
if [ "$1" == "--with-tester" ]; then
    echo "Building relay-tester image..."
    docker build -t orly-relay-tester:latest -f Dockerfile.relay-tester .
    echo "✅ Relay-tester image built successfully"
    echo ""
fi

# Show images
echo "Built images:"
docker images | grep -E "orly|REPOSITORY"
echo ""

echo "=== Build Complete ==="
echo ""
echo "To run:"
echo "  cd scripts && $DOCKER_COMPOSE -f docker-compose-test.yml up -d"
echo ""
echo "To test:"
echo "  ./scripts/test-docker.sh"

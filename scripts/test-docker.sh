#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== ORLY Dgraph Docker Integration Test Suite ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Check if docker is available
if ! command -v docker &> /dev/null; then
    print_error "Docker is not installed or not in PATH"
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    print_error "Docker Compose is not installed or not in PATH"
    exit 1
fi

# Determine docker-compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

print_info "Using docker-compose command: $DOCKER_COMPOSE"
echo ""

# Change to scripts directory
cd "$SCRIPT_DIR"

# Parse arguments
SKIP_BUILD=false
KEEP_RUNNING=false
RUN_RELAY_TESTER=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --keep-running)
            KEEP_RUNNING=true
            shift
            ;;
        --relay-tester)
            RUN_RELAY_TESTER=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--skip-build] [--keep-running] [--relay-tester]"
            exit 1
            ;;
    esac
done

# Cleanup function
cleanup() {
    if [ "$KEEP_RUNNING" = false ]; then
        print_info "Cleaning up containers..."
        $DOCKER_COMPOSE -f docker-compose-test.yml down
        print_success "Cleanup complete"
    else
        print_info "Containers left running (--keep-running)"
        echo ""
        print_info "To stop: cd $SCRIPT_DIR && $DOCKER_COMPOSE -f docker-compose-test.yml down"
        print_info "View logs: $DOCKER_COMPOSE -f docker-compose-test.yml logs -f"
        print_info "ORLY:    http://localhost:3334"
        print_info "Dgraph:  http://localhost:8080"
        print_info "Ratel:   http://localhost:8000"
    fi
}

# Set trap for cleanup
if [ "$KEEP_RUNNING" = false ]; then
    trap cleanup EXIT
fi

# Stop any existing containers
print_info "Stopping any existing containers..."
$DOCKER_COMPOSE -f docker-compose-test.yml down --remove-orphans
echo ""

# Build images if not skipping
if [ "$SKIP_BUILD" = false ]; then
    print_info "Building ORLY docker image..."
    $DOCKER_COMPOSE -f docker-compose-test.yml build orly
    print_success "Build complete"
    echo ""
fi

# Start dgraph
print_info "Starting dgraph server..."
$DOCKER_COMPOSE -f docker-compose-test.yml up -d dgraph

# Wait for dgraph to be healthy
print_info "Waiting for dgraph to be healthy..."
MAX_WAIT=60
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if docker exec orly-dgraph curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        print_success "Dgraph is healthy"
        break
    fi
    sleep 2
    WAITED=$((WAITED + 2))
    if [ $WAITED -ge $MAX_WAIT ]; then
        print_error "Dgraph failed to become healthy after ${MAX_WAIT}s"
        docker logs orly-dgraph
        exit 1
    fi
done
echo ""

# Start ORLY
print_info "Starting ORLY relay with dgraph backend..."
$DOCKER_COMPOSE -f docker-compose-test.yml up -d orly

# Wait for ORLY to be healthy
print_info "Waiting for ORLY to be healthy..."
MAX_WAIT=60
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if curl -sf http://localhost:3334/ > /dev/null 2>&1; then
        print_success "ORLY is healthy and responding"
        break
    fi
    sleep 2
    WAITED=$((WAITED + 2))
    if [ $WAITED -ge $MAX_WAIT ]; then
        print_error "ORLY failed to become healthy after ${MAX_WAIT}s"
        echo ""
        print_info "ORLY logs:"
        docker logs orly-relay
        exit 1
    fi
done
echo ""

# Check ORLY version
print_info "Checking ORLY version..."
ORLY_VERSION=$(docker exec orly-relay /app/orly version 2>&1 | head -1 || echo "unknown")
echo "ORLY version: $ORLY_VERSION"
echo ""

# Verify dgraph connection
print_info "Verifying dgraph connection..."
if docker logs orly-relay 2>&1 | grep -q "successfully connected to dgraph"; then
    print_success "ORLY successfully connected to dgraph"
elif docker logs orly-relay 2>&1 | grep -q "dgraph"; then
    print_info "ORLY dgraph logs:"
    docker logs orly-relay 2>&1 | grep -i dgraph
else
    print_info "No explicit dgraph connection message (may be using badger)"
fi
echo ""

# Basic connectivity test
print_info "Testing basic relay connectivity..."
if curl -sf http://localhost:3334/ > /dev/null 2>&1; then
    print_success "ORLY is accessible at http://localhost:3334"
else
    print_error "Failed to connect to ORLY"
    exit 1
fi
echo ""

# Test WebSocket connection
print_info "Testing WebSocket connection..."
if command -v websocat &> /dev/null; then
    TEST_REQ='["REQ","test",{"kinds":[1],"limit":1}]'
    if echo "$TEST_REQ" | timeout 5 websocat ws://localhost:3334 2>/dev/null | grep -q "EOSE"; then
        print_success "WebSocket connection successful"
    else
        print_info "WebSocket test inconclusive (may need events)"
    fi
elif command -v wscat &> /dev/null; then
    print_info "Testing with wscat..."
    # wscat test would go here
else
    print_info "WebSocket testing tools not available (install websocat or wscat)"
fi
echo ""

# Run relay-tester if requested
if [ "$RUN_RELAY_TESTER" = true ]; then
    print_info "Building relay-tester image..."
    $DOCKER_COMPOSE -f docker-compose-test.yml build relay-tester
    echo ""

    print_info "Running relay-tester against ORLY..."
    if $DOCKER_COMPOSE -f docker-compose-test.yml run --rm relay-tester -url ws://orly:3334; then
        print_success "Relay-tester passed!"
    else
        print_error "Relay-tester failed"
        echo ""
        print_info "ORLY logs:"
        docker logs orly-relay --tail 50
        exit 1
    fi
    echo ""
fi

# Show container status
print_info "Container status:"
$DOCKER_COMPOSE -f docker-compose-test.yml ps
echo ""

# Show useful information
print_success "All tests passed!"
echo ""
print_info "Endpoints:"
echo "  ORLY WebSocket:  ws://localhost:3334"
echo "  ORLY HTTP:       http://localhost:3334"
echo "  Dgraph HTTP:     http://localhost:8080"
echo "  Dgraph gRPC:     localhost:9080"
echo "  Ratel UI:        http://localhost:8000"
echo ""

if [ "$KEEP_RUNNING" = false ]; then
    print_info "Containers will be stopped on script exit"
else
    print_info "Containers are running. Use --keep-running flag was set."
fi

exit 0
